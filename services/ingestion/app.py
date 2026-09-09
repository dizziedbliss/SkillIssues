"""Stream and qualify local repository metadata for the SkillIssues catalog."""

from __future__ import annotations

import hashlib
import json
import logging
import os
import time
from collections.abc import Iterator
from datetime import datetime
from pathlib import Path
from typing import Any

import psycopg

logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s ingestion %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

DATABASE_URL = os.environ["DATABASE_URL"]
METADATA_PATH = Path(os.getenv("REPOSITORY_METADATA_PATH", "/repo/repo_metadata.json"))
MAX_REPOSITORIES = int(os.getenv("MAX_REPOSITORIES", "1000"))
MIN_STARS = int(os.getenv("MIN_STARS", "50"))
RUN_ONCE = os.getenv("RUN_ONCE", "false").lower() == "true"


def text(value: Any) -> str:
    return str(value or "").strip()


def integer(value: Any) -> int:
    try:
        return max(0, int(value or 0))
    except (TypeError, ValueError):
        return 0


def parse_timestamp(value: Any) -> datetime | None:
    raw = text(value)
    if not raw:
        return None
    try:
        return datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None


def stable_github_id(full_name: str) -> int:
    """Create a stable numeric key when the export does not include GitHub's node id."""
    return int.from_bytes(hashlib.sha256(full_name.encode("utf-8")).digest()[:8], "big", signed=False) & ((1 << 63) - 1)


def clean_repository(record: dict[str, Any], min_stars: int = MIN_STARS) -> dict[str, Any] | None:
    full_name = text(record.get("nameWithOwner")) or "/".join(
        part for part in (text(record.get("owner")), text(record.get("name"))) if part
    )
    license_name = text(record.get("license"))
    if not full_name or integer(record.get("stars")) <= min_stars:
        return None
    if record.get("isArchived") is not False or record.get("isFork") is True:
        return None
    if record.get("forkingAllowed") is not True or not license_name:
        return None

    primary_language = record.get("primaryLanguage")
    if isinstance(primary_language, dict):
        primary_language = primary_language.get("name")
    topics = record.get("topics") or []
    topic_names = [text(item.get("name") if isinstance(item, dict) else item) for item in topics]
    return {
        "github_id": stable_github_id(full_name),
        "full_name": full_name,
        "url": f"https://github.com/{full_name}",
        "description": text(record.get("description")),
        "language": text(primary_language) or None,
        "stars": integer(record.get("stars")),
        "license": license_name,
        "forkable": True,
        "pushed_at": parse_timestamp(record.get("pushedAt")),
        "topics": [topic for topic in topic_names if topic],
    }


def stream_records(path: Path) -> Iterator[dict[str, Any]]:
    decoder = json.JSONDecoder()
    with path.open("r", encoding="utf-8") as handle:
        buffer = ""
        end_of_file = False
        started = False
        while True:
            if not buffer and not end_of_file:
                buffer = handle.read(1024 * 1024)
                end_of_file = not buffer
            buffer = buffer.lstrip()
            if not started:
                if not buffer.startswith("["):
                    raise ValueError("repository metadata must be a top-level JSON array")
                buffer = buffer[1:]
                started = True
            buffer = buffer.lstrip()
            if buffer.startswith("]"):
                return
            if not buffer:
                if end_of_file:
                    raise ValueError("repository metadata ended before the array closed")
                continue
            try:
                record, consumed = decoder.raw_decode(buffer)
            except json.JSONDecodeError:
                if end_of_file:
                    raise ValueError("invalid repository metadata JSON") from None
                chunk = handle.read(1024 * 1024)
                buffer += chunk
                end_of_file = not chunk
                continue
            if not isinstance(record, dict):
                raise ValueError("repository metadata records must be JSON objects")
            yield record
            buffer = buffer[consumed:].lstrip()
            if buffer.startswith(","):
                buffer = buffer[1:]
            elif buffer.startswith("]"):
                return
            elif not buffer and not end_of_file:
                continue
            else:
                raise ValueError("repository metadata records must be comma-separated")


def wait_for_database() -> None:
    for attempt in range(30):
        try:
            with psycopg.connect(DATABASE_URL) as connection:
                connection.execute("SELECT 1 FROM repositories LIMIT 1")
            return
        except psycopg.OperationalError:
            logger.info("database is not initialized yet; retry %s/30", attempt + 1)
            time.sleep(2)
    raise RuntimeError("database did not become ready")


def import_repositories() -> int:
    if not METADATA_PATH.is_file():
        raise FileNotFoundError(f"repository metadata not found: {METADATA_PATH}")
    wait_for_database()
    imported = 0
    scanned = 0
    with psycopg.connect(DATABASE_URL) as connection:
        with connection.cursor() as cursor:
            for record in stream_records(METADATA_PATH):
                scanned += 1
                repository = clean_repository(record)
                if repository is None:
                    continue
                cursor.execute("""
                    INSERT INTO repositories
                        (github_id, full_name, url, description, language, stars, license, forkable, last_activity_at, last_synced_at)
                    VALUES (%(github_id)s, %(full_name)s, %(url)s, %(description)s, %(language)s,
                            %(stars)s, %(license)s, %(forkable)s, %(pushed_at)s, now())
                                        ON CONFLICT (full_name) DO UPDATE SET
                                            github_id = EXCLUDED.github_id, url = EXCLUDED.url,
                      description = EXCLUDED.description, language = EXCLUDED.language,
                      stars = EXCLUDED.stars, license = EXCLUDED.license,
                      forkable = EXCLUDED.forkable, last_activity_at = EXCLUDED.last_activity_at,
                      last_synced_at = now()
                """, repository)
                imported += 1
                if imported >= MAX_REPOSITORIES:
                    break
        connection.commit()
    logger.info("repository import complete scanned=%s imported=%s limit=%s", scanned, imported, MAX_REPOSITORIES)
    return imported


if __name__ == "__main__":
    import_repositories()
    if RUN_ONCE:
        raise SystemExit(0)
    while True:
        time.sleep(900)
