"""Stream and qualify local repository metadata for the SkillIssues catalog."""

from __future__ import annotations

import hashlib
import http.client
import json
import logging
import os
import random
import threading
import time
import urllib.error
import urllib.request
from collections.abc import Iterator
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import psycopg

logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s ingestion %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

DATABASE_URL = os.environ["DATABASE_URL"]
METADATA_PATH = Path(os.getenv("REPOSITORY_METADATA_PATH", "/repo/repo_metadata.json"))
MAX_REPOSITORIES = int(os.getenv("MAX_REPOSITORIES", "1000"))
SAMPLE_SEED = os.getenv("SAMPLE_SEED", "")
RESET_CATALOG = os.getenv("RESET_CATALOG", "false").lower() == "true"
MIN_STARS = int(os.getenv("MIN_STARS", "50"))
RUN_ONCE = os.getenv("RUN_ONCE", "false").lower() == "true"
GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", "")
SYNC_ISSUES = os.getenv("SYNC_ISSUES", "false").lower() == "true"
SYNC_REPOSITORIES = int(os.getenv("SYNC_REPOSITORIES", "10"))
INGESTION_PORT = int(os.getenv("INGESTION_PORT", "8000"))
GRAPHQL_URL = "https://api.github.com/graphql"
sync_lock = threading.Lock()

ISSUES_QUERY = """
query RepositoryIssues($owner: String!, $name: String!, $after: String, $since: DateTime) {
    repository(owner: $owner, name: $name) {
        issues(first: 50, after: $after, states: OPEN, orderBy: {field: UPDATED_AT, direction: DESC}, filterBy: {since: $since}) {
            nodes {
                databaseId
                number
                title
                body
                url
                state
                updatedAt
                comments { totalCount }
                labels(first: 10) { nodes { name } }
            }
            pageInfo { hasNextPage endCursor }
        }
        stargazerCount
    }
    rateLimit { limit remaining used resetAt cost }
}
"""


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


def difficulty_for_issue(issue: dict[str, Any], repository: dict[str, Any]) -> tuple[int, str, list[str], list[str], list[str]]:
    labels = [text(label.get("name")) for label in issue.get("labels", {}).get("nodes", [])]
    label_text = " ".join(labels).lower()
    body = text(issue.get("body"))
    score = 4
    if any(word in label_text for word in ("good first issue", "beginner", "easy")):
        score -= 2
    if any(word in label_text for word in ("help wanted", "intermediate")):
        score += 1
    if any(word in label_text for word in ("advanced", "complex", "hard")):
        score += 2
    if len(body) > 1800:
        score += 1
    if issue.get("comments", {}).get("totalCount", 0) > 10:
        score += 1
    if repository.get("stars", 0) > 100000:
        score += 1
    score = max(1, min(10, score))
    difficulty = "BEGINNER" if score <= 3 else "INTERMEDIATE" if score <= 6 else "ADVANCED"
    skills = []
    language = text(repository.get("language"))
    if language:
        skills.append(language)
    if any(word in label_text or word in body.lower() for word in ("api", "http", "rest")):
        skills.append("REST APIs")
    if any(word in label_text or word in body.lower() for word in ("docs", "documentation")):
        skills.append("Technical Writing")
    technologies = [language] if language else []
    tags = labels[:5] or ["open source", "software development"]
    return score, difficulty, list(dict.fromkeys(skills)), technologies, tags


def stable_github_id(full_name: str) -> int:
    """Create a stable numeric key when the export does not include GitHub's node id."""
    return int.from_bytes(hashlib.sha256(full_name.encode("utf-8")).digest()[:8], "big", signed=False) & ((1 << 63) - 1)


def clean_repository(record: dict[str, Any], min_stars: int = MIN_STARS) -> dict[str, Any] | None:
    full_name = text(record.get("nameWithOwner")) or "/".join(
        part for part in (text(record.get("owner")), text(record.get("name"))) if part
    )
    license_name = text(record.get("license"))
    if license_name.lower() in {"null", "none", "n/a"}:
        license_name = ""
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
    try:
        import pandas as pd
        import ijson
    except ModuleNotFoundError as error:
        raise RuntimeError("pandas and ijson are required for repository import; use the ingestion Docker image or install ingestion requirements") from error
    wait_for_database()
    imported = 0
    batch_size = 25_000
    scanned = qualified_count = 0
    raw_candidates: list[dict[str, Any]] = []
    with METADATA_PATH.open("rb") as metadata_file:
        batch: list[dict[str, Any]] = []
        for record in ijson.items(metadata_file, "item"):
            batch.append(record)
            if len(batch) < batch_size:
                continue
            scanned, qualified_count = _filter_batch(pd, batch, scanned, qualified_count, raw_candidates)
            batch.clear()
        if batch:
            scanned, qualified_count = _filter_batch(pd, batch, scanned, qualified_count, raw_candidates)

    if raw_candidates:
        df = pd.DataFrame.from_records(raw_candidates)
        df["stars"] = pd.to_numeric(df["stars"], errors="coerce").fillna(0)
        df = df.sort_values(by="stars", ascending=False)
        df["full_name_clean"] = df["nameWithOwner"].fillna("").astype(str)
        df = df.drop_duplicates(subset=["full_name_clean"])
        sample = df.head(MAX_REPOSITORIES).to_dict(orient="records")
    else:
        sample = []

    logger.info("pandas catalog filter & sort complete scanned=%s qualified=%s sample=%s", scanned, qualified_count, len(sample))
    with psycopg.connect(DATABASE_URL) as connection:
        with connection.cursor() as cursor:
            if RESET_CATALOG:
                cursor.execute("DELETE FROM contributions")
                cursor.execute("DELETE FROM saved_issues")
                cursor.execute("DELETE FROM issues")
                cursor.execute("DELETE FROM repositories")
                logger.warning("repository catalog reset enabled; existing repository, issue, and contribution data was removed")
            for record in sample:
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
        connection.commit()
        created_issues = ensure_default_issues_for_repositories(connection)
    logger.info("repository import complete scanned=%s qualified=%s imported=%s sample_size=%s created_issues=%s", scanned, qualified_count, imported, MAX_REPOSITORIES, created_issues)
    return imported


def _filter_batch(pd: Any, records: list[dict[str, Any]], scanned: int, qualified_count: int, raw_candidates: list[dict[str, Any]]) -> tuple[int, int]:
    frame = pd.DataFrame.from_records(records)
    stars_values = frame["stars"] if "stars" in frame else pd.Series(0, index=frame.index)
    archived_values = frame["isArchived"] if "isArchived" in frame else pd.Series(False, index=frame.index)
    fork_values = frame["isFork"] if "isFork" in frame else pd.Series(False, index=frame.index)
    forking_values = frame["forkingAllowed"] if "forkingAllowed" in frame else pd.Series(False, index=frame.index)
    license_values = frame["license"] if "license" in frame else pd.Series("", index=frame.index)
    frame["stars"] = pd.to_numeric(stars_values, errors="coerce").fillna(0)
    frame["license_valid"] = ~license_values.fillna("").astype(str).str.strip().str.lower().isin({"", "null", "none", "n/a"})
    mask = (
        (archived_values == False)
        & (frame["stars"] >= MIN_STARS)
        & (fork_values == False)
        & (forking_values == True)
        & frame["license_valid"]
    )
    qualified = frame.loc[mask].drop(columns=["license_valid"], errors="ignore").to_dict(orient="records")
    raw_candidates.extend(qualified)
    return scanned + len(records), qualified_count + len(qualified)


def repository_parts(full_name: str) -> tuple[str, str]:
    owner, separator, name = full_name.partition("/")
    if not separator or not owner or not name:
        raise ValueError(f"invalid GitHub repository name: {full_name}")
    return owner, name


def sync_repository_issues(repository: dict[str, Any]) -> int:
    if not GITHUB_TOKEN:
        return 0
    owner, name = repository_parts(repository["full_name"])
    state = query_sync_state(repository["id"])
    since = state.get("last_synced_at") if state else None
    cursor: str | None = None
    synced = 0
    headers = {"Authorization": f"Bearer {GITHUB_TOKEN}", "Content-Type": "application/json"}
    while True:
        variables = {"owner": owner, "name": name, "after": cursor, "since": since}
        request = urllib.request.Request(
            GRAPHQL_URL,
            data=json.dumps({"query": ISSUES_QUERY, "variables": variables}).encode("utf-8"),
            headers=headers,
            method="POST",
        )
        for attempt in range(3):
            try:
                with urllib.request.urlopen(request, timeout=15) as response:
                    payload = json.load(response)
                break
            except urllib.error.HTTPError as error:
                if error.code in (403, 429):
                    logger.warning("GitHub rate limit response repository=%s status=%s", repository["full_name"], error.code)
                    return synced
                raise
            except (TimeoutError, http.client.IncompleteRead, urllib.error.URLError) as error:
                if attempt == 2:
                    logger.warning("GitHub request abandoned repository=%s error=%s", repository["full_name"], error)
                    return synced
                delay = 2 ** attempt
                logger.warning("GitHub request retry repository=%s attempt=%s delay=%ss", repository["full_name"], attempt + 1, delay)
                time.sleep(delay)
        if payload.get("errors"):
            raise RuntimeError(f"GitHub GraphQL error for {repository['full_name']}: {payload['errors'][0].get('message')}")
        data = payload["data"]
        rate_limit = data["rateLimit"]
        update_rate_limit(repository["id"], rate_limit)
        if rate_limit["remaining"] < 100:
            logger.warning("GitHub rate limit low remaining=%s reset=%s", rate_limit["remaining"], rate_limit["resetAt"])
            break
        for issue in data["repository"]["issues"]["nodes"]:
            score, difficulty, skills, technologies, tags = difficulty_for_issue(issue, repository)
            upsert_issue(repository["id"], issue, score, difficulty, skills, technologies, tags)
            synced += 1
        page_info = data["repository"]["issues"]["pageInfo"]
        if not page_info["hasNextPage"]:
            break
        cursor = page_info["endCursor"]
    mark_repository_synced(repository["id"])
    logger.info("issue sync complete repository=%s issues=%s", repository["full_name"], synced)
    return synced


def query_sync_state(repository_id: int) -> dict[str, Any]:
    with psycopg.connect(DATABASE_URL) as connection:
        row = connection.execute("SELECT last_synced_at FROM sync_state WHERE repository_id = %s", (repository_id,)).fetchone()
    return {"last_synced_at": row[0].isoformat() if row and row[0] else None} if row else {}


def update_rate_limit(repository_id: int, rate_limit: dict[str, Any]) -> None:
    with psycopg.connect(DATABASE_URL) as connection:
        connection.execute("""
            INSERT INTO sync_state
              (repository_id, rate_limit_limit, rate_limit_remaining, rate_limit_used,
               rate_limit_reset_at, rate_limit_cost)
            VALUES (%s, %s, %s, %s, %s, %s)
            ON CONFLICT (repository_id) DO UPDATE SET
              rate_limit_limit = EXCLUDED.rate_limit_limit,
              rate_limit_remaining = EXCLUDED.rate_limit_remaining,
              rate_limit_used = EXCLUDED.rate_limit_used,
              rate_limit_reset_at = EXCLUDED.rate_limit_reset_at,
              rate_limit_cost = EXCLUDED.rate_limit_cost
        """, (
            repository_id, rate_limit.get("limit"), rate_limit.get("remaining"),
            rate_limit.get("used"), parse_timestamp(rate_limit.get("resetAt")),
            rate_limit.get("cost"),
        ))


def mark_repository_synced(repository_id: int) -> None:
    with psycopg.connect(DATABASE_URL) as connection:
        connection.execute("""
            INSERT INTO sync_state (repository_id, last_synced_at, priority)
            VALUES (%s, now(), 'MEDIUM')
            ON CONFLICT (repository_id) DO UPDATE SET last_synced_at = now()
        """, (repository_id,))


def upsert_issue(repository_id: int, issue: dict[str, Any], score: int, difficulty: str, skills: list[str], technologies: list[str], tags: list[str]) -> None:
    with psycopg.connect(DATABASE_URL) as connection:
        connection.execute("""
            INSERT INTO issues
              (github_id, repository_id, number, title, body, url, state, difficulty_score,
               difficulty, required_skills, technologies, learning_tags, comments, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            ON CONFLICT (github_id) DO UPDATE SET
              repository_id = EXCLUDED.repository_id, title = EXCLUDED.title,
              body = EXCLUDED.body, url = EXCLUDED.url, state = EXCLUDED.state,
              difficulty_score = EXCLUDED.difficulty_score, difficulty = EXCLUDED.difficulty,
              required_skills = EXCLUDED.required_skills, technologies = EXCLUDED.technologies,
              learning_tags = EXCLUDED.learning_tags, comments = EXCLUDED.comments,
              updated_at = EXCLUDED.updated_at
        """, (
            issue["databaseId"], repository_id, issue["number"], issue["title"], issue.get("body") or "",
            issue["url"], issue["state"], score, difficulty, skills, technologies, tags,
            issue.get("comments", {}).get("totalCount", 0), parse_timestamp(issue.get("updatedAt")) or datetime.now(timezone.utc),
        ))


def sync_issues() -> int:
    if not GITHUB_TOKEN:
        logger.info("GITHUB_TOKEN not configured; skipping GitHub issue synchronization")
        return 0
    with psycopg.connect(DATABASE_URL) as connection:
        repositories = connection.execute("""
            SELECT id, full_name, language, stars FROM repositories
            ORDER BY CASE WHEN last_activity_at IS NULL THEN 1 ELSE 0 END, last_activity_at DESC
            LIMIT %s
        """, (SYNC_REPOSITORIES,)).fetchall()
    total = 0
    for repository_id, full_name, language, stars in repositories:
        total += sync_repository_issues({"id": repository_id, "full_name": full_name, "language": language, "stars": stars})
    return total


class IngestionHandler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path != "/health":
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"status":"ok","service":"ingestion"}')

    def do_POST(self) -> None:
        if self.path not in ("/sync/repositories", "/sync/issues"):
            self.send_error(404)
            return
        if not sync_lock.acquire(blocking=False):
            self.send_response(409)
            self.end_headers()
            return
        try:
            result = import_repositories() if self.path.endswith("repositories") else sync_issues()
            payload = json.dumps({"accepted": True, "imported": result}).encode("utf-8")
            self.send_response(202)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(payload)
        except Exception as error:
            logger.exception("scheduled sync failed")
            self.send_error(500, str(error))
        finally:
            sync_lock.release()

    def log_message(self, format: str, *args: Any) -> None:
        logger.info("scheduler request " + format, *args)


def serve_scheduler() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", INGESTION_PORT), IngestionHandler)
    logger.info("ingestion scheduler listening on port %s", INGESTION_PORT)
    server.serve_forever()


if __name__ == "__main__":
    if METADATA_PATH.is_file():
        import_repositories()
    else:
        logger.warning("repository metadata not found; serving sync endpoints with seed data only: %s", METADATA_PATH)
    if SYNC_ISSUES:
        sync_issues()
    if RUN_ONCE:
        raise SystemExit(0)
    serve_scheduler()
