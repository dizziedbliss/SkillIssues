#!/usr/bin/env python3

"""

SkillIssues Importer v6

GitHub GraphQL -> metadata extraction (port of sync/classify.go) -> PostgreSQL upsert.



Safety:

- Read-only against repositories.

- No schema migrations or deletes.

- Dry-run is the default; pass --write to persist issue rows and checkpoints.

- Checkpoints advance only after the corresponding database transaction commits.

- Any GraphQL errors invalidate the whole response batch, even when HTTP status is 200.

- This is a benchmark/importer, not a production crawler. Run against a small sample first.

"""

from __future__ import annotations



import argparse

import json

import os

import re

import sys

import time

from datetime import datetime, timezone

from pathlib import Path

from typing import Any



import requests



try:

    import psycopg2

    from psycopg2.extras import execute_values

except ImportError:

    psycopg2 = None

    execute_values = None



BEGINNER_LABELS = {

    "good first issue", "beginner-friendly", "easy", "starter", "starter-bug",

    "first-timers-only", "newbie", "up-for-grabs", "low-hanging-fruit",

    "bitesize", "trivial", "easy-fix", "good-for-beginner", "documentation",

    "docs", "typo", "good-first-pr",

}

INTERMEDIATE_LABELS = {

    "help wanted", "contributions-welcome", "seeking-contributors", "medium",

    "intermediate", "size/m", "effort/medium", "difficulty/medium",

    "enhancement", "feature", "refactoring", "unit-test",

}

ADVANCED_LABELS = {

    "hard", "advanced", "complex", "size/l", "size/xl", "difficulty/hard",

    "breaking-change", "architecture", "security", "performance",

    "optimization", "critical", "high-priority", "blocker", "p1",

}





def starts_with_any(value: str, *prefixes: str) -> bool:

    return any(value.startswith(prefix) for prefix in prefixes)





def classify_labels(labels: list[str]) -> tuple[bool, str, int]:

    normalized = [s.strip().lower() for s in labels if s and s.strip()]

    for label in normalized:

        if label in BEGINNER_LABELS or starts_with_any(

            label, "difficulty/easy", "difficulty/beginner", "size/small",

            "size/xs", "effort/low", "exp/beginner"

        ):

            return True, "BEGINNER", 2

    for label in normalized:

        if label in INTERMEDIATE_LABELS or starts_with_any(

            label, "difficulty/medium", "size/m", "effort/medium", "exp/intermediate"

        ):

            return True, "INTERMEDIATE", 5

    for label in normalized:

        if label in ADVANCED_LABELS or starts_with_any(

            label, "difficulty/hard", "difficulty/expert", "size/l", "size/xl",

            "effort/high", "exp/expert"

        ):

            return True, "ADVANCED", 8

    return False, "BEGINNER", 3





def extract_metadata(labels: list[str], body: str, repo_language: str):

    has_valid, difficulty, score = classify_labels(labels)

    label_names = [s.strip() for s in labels if s and s.strip()]

    label_text = " ".join(label_names).lower()

    body_lower = (body or "").lower()



    skills: list[str] = []

    seen: set[str] = set()



    def add_skill(value: str):

        value = value.strip()

        if value and value not in seen:

            seen.add(value)

            skills.append(value)



    technologies: list[str] = []

    if repo_language:

        add_skill(repo_language)

        technologies = [repo_language]



    if any(k in label_text for k in ("api", "http", "rest")) or any(

        k in body_lower for k in ("api", "http", "rest")

    ):

        add_skill("REST APIs")

    if any(k in label_text for k in ("docs", "documentation")) or any(

        k in body_lower for k in ("docs", "documentation")

    ):

        add_skill("Technical Writing")



    tags = label_names[:5] if label_names else ["open source", "software development"]

    return score, difficulty, has_valid, skills, technologies, tags





def gql_escape(value: str) -> str:

    return json.dumps(value, ensure_ascii=False)





def build_query(repos: list[dict[str, Any]], page_size: int) -> str:

    fields = []

    for i, repo in enumerate(repos):

        alias = f"r{i}"

        cursor = repo.get("cursor")

        after = f", after: {gql_escape(cursor)}" if cursor else ""

        fields.append(

            f"""{alias}: repository(owner: {gql_escape(repo['owner'])}, name: {gql_escape(repo['name'])}) {{

              nameWithOwner

              issues(first: {page_size}, states: [OPEN], orderBy: {{field: UPDATED_AT, direction: DESC}}{after}) {{

                totalCount

                pageInfo {{ hasNextPage endCursor }}

                nodes {{

                  databaseId number title body url state updatedAt

                  author {{ login }}





                  labels(first: 100) {{ nodes {{ name }} pageInfo {{ hasNextPage }} }}

                }}

              }}

            }}"""

        )

    return "query SkillIssuesImporterV6 { " + " ".join(fields) + " rateLimit { cost remaining resetAt } }"





def atomic_write_json(path: Path, data: dict[str, Any]) -> None:

    path.parent.mkdir(parents=True, exist_ok=True)

    tmp = path.with_suffix(path.suffix + ".tmp")

    tmp.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")

    tmp.replace(path)





def main():

    parser = argparse.ArgumentParser(description="Benchmark GitHub issue sync into SkillIssues PostgreSQL.")

    parser.add_argument("--repo-limit", type=int, default=20, help="Maximum repositories selected from DB (default: 20)")

    parser.add_argument("--batch-size", type=int, default=50, help="Repositories per GraphQL request (default: 50)")

    parser.add_argument("--page-size", type=int, default=100, help="Issues per repository page, 1-100 (default: 100)")

    parser.add_argument("--max-pages-per-repo", type=int, default=1, help="Pages per repository in the initial pass (default: 1)")

    parser.add_argument("--pause", type=float, default=0.25, help="Seconds between GraphQL requests (default: 0.25)")

    parser.add_argument("--checkpoint", default="skillissues_v5_checkpoint.json")

    parser.add_argument("--metrics", default="skillissues_v5_metrics.jsonl")

    parser.add_argument("--write", action="store_true", help="Actually upsert issues and persist cursors. Without this, dry-run only.")

    parser.add_argument("--resume", action="store_true", help="Resume from existing checkpoint file.")

    args = parser.parse_args()



    if args.repo_limit < 1 or args.batch_size < 1 or args.page_size < 1 or args.page_size > 100:

        parser.error("--repo-limit and --batch-size must be >=1; --page-size must be 1..100")

    if args.max_pages_per_repo < 1:

        parser.error("--max-pages-per-repo must be >=1")

    if not args.write:

        print("DRY-RUN: no issue rows or checkpoints will be written. Pass --write to persist.", flush=True)



    db_url = os.environ.get("DATABASE_URL")

    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")

    if not db_url:

        sys.exit("Missing DATABASE_URL environment variable.")

    if not token:

        sys.exit("Missing GITHUB_TOKEN (or GH_TOKEN) environment variable.")

    if psycopg2 is None:

        sys.exit("Missing Python dependencies. Install: python -m pip install requests psycopg2-binary")



    checkpoint_path = Path(args.checkpoint)

    checkpoint: dict[str, Any] = {}

    if args.resume and checkpoint_path.exists():

        checkpoint = json.loads(checkpoint_path.read_text(encoding="utf-8"))



    conn = psycopg2.connect(db_url)

    conn.autocommit = False

    try:

        with conn.cursor() as cur:

            cur.execute("""

                SELECT id, full_name, language

                FROM repositories

                ORDER BY stars DESC, id ASC

                LIMIT %s

            """, (args.repo_limit,))

            db_repos = cur.fetchall()



        if not db_repos:

            sys.exit("No repositories found in repositories table.")



        repos: list[dict[str, Any]] = []

        for repo_id, full_name, language in db_repos:

            if "/" not in full_name:

                print(f"Skipping invalid repository full_name: {full_name}", file=sys.stderr)

                continue

            owner, name = full_name.split("/", 1)

            state = checkpoint.get(full_name, {}) if args.resume else {}

            repos.append({

                "id": repo_id, "full_name": full_name, "owner": owner, "name": name,

                "language": language or "", "cursor": state.get("cursor"),

                "pages_done": int(state.get("pages_done", 0)),

                "done": bool(state.get("done", False)),

            })



        headers = {

            "Authorization": f"Bearer {token}",

            "Accept": "application/vnd.github+json",

            "Content-Type": "application/json",

            "User-Agent": "SkillIssues-Importer-v6",

        }

        session = requests.Session()

        session.headers.update(headers)

        metrics_path = Path(args.metrics)

        run_started = time.perf_counter()

        total_pages = total_issues = total_db_rows = total_points = total_bytes = 0

        failed_batches = 0
        current_batch_size = args.batch_size
        deferred_this_run: set[str] = set()

        # A repo is included at most once per request. Keep each repo's cursor independent.
        while True:
            active = [
                r for r in repos
                if not r["done"]
                and r["pages_done"] < args.max_pages_per_repo
                and r["full_name"] not in deferred_this_run
            ]

            if not active:

                break

            batch = active[:current_batch_size]

            query = build_query(batch, args.page_size)

            req_started = time.perf_counter()

            try:

                response = session.post(

                    "https://api.github.com/graphql",

                    json={"query": query},

                    timeout=(10, 45),

                )

                elapsed = time.perf_counter() - req_started

                total_bytes += len(response.content)

                if response.status_code != 200:

                    failed_batches += 1

                    print(f"FAILED HTTP {response.status_code}: {response.text[:500]}", file=sys.stderr, flush=True)

                    break

                payload = response.json()

                errors = payload.get("errors") or []

                if errors:

                    # A deleted/renamed/private repository should not block the other aliases.
                    missing_indexes = set()
                    for error in errors:
                        if error.get("type") != "NOT_FOUND":
                            continue
                        path = error.get("path") or []
                        if path and isinstance(path[0], str) and path[0].startswith("r") and path[0][1:].isdigit():
                            missing_indexes.add(int(path[0][1:]))

                    if missing_indexes:
                        unavailable = []
                        for index in sorted(missing_indexes):
                            if 0 <= index < len(batch):
                                missing_repo = batch[index]
                                missing_repo["done"] = True
                                missing_repo["unavailable"] = True
                                unavailable.append(missing_repo["full_name"])
                                if args.write:
                                    checkpoint[missing_repo["full_name"]] = {
                                        "cursor": None,
                                        "pages_done": missing_repo["pages_done"],
                                        "done": True,
                                        "unavailable": True,
                                        "reason": "GitHub GraphQL NOT_FOUND",
                                    }
                        if args.write and unavailable:
                            atomic_write_json(checkpoint_path, checkpoint)
                        print(
                            "SKIPPING unavailable repository/repositories: " + ", ".join(unavailable) +
                            ". Retrying the remaining repositories in this batch.",
                            file=sys.stderr, flush=True,
                        )
                        # Do not commit any partial GraphQL data from the errored response.
                        # Valid repositories remain active and will be queried again next loop.
                        continue

                    resource_errors = [
                        error for error in errors
                        if error.get("type") == "RESOURCE_LIMITS_EXCEEDED"
                    ]
                    if resource_errors:
                        failed_batches += 1
                        if len(batch) > 1:
                            previous_size = current_batch_size
                            current_batch_size = max(1, len(batch) // 2)
                            print(
                                f"RESOURCE LIMIT: batch of {len(batch)} repos failed; "
                                f"retrying with {current_batch_size}. Successful batches "
                                f"will return to configured size {args.batch_size}.",
                                file=sys.stderr, flush=True,
                            )
                            continue

                        # A single repo still exceeds query limits. Defer it for this
                        # run, but do not mark it permanently complete: the next --resume
                        # run can retry it after other work has progressed.
                        difficult_repo = batch[0]
                        deferred_this_run.add(difficult_repo["full_name"])
                        print(
                            f"DEFER RESOURCE-LIMITED REPOSITORY: {difficult_repo['full_name']}; "
                            "continuing with the rest. It remains retryable on a later run.",
                            file=sys.stderr, flush=True,
                        )
                        if args.write:
                            checkpoint[difficult_repo["full_name"]] = {
                                "cursor": difficult_repo.get("cursor"),
                                "pages_done": difficult_repo["pages_done"],
                                "done": False,
                                "deferred": True,
                                "reason": "RESOURCE_LIMITS_EXCEEDED for a single-repository query",
                            }
                            atomic_write_json(checkpoint_path, checkpoint)
                        # Don't repeatedly select this repo in this process. Other repos
                        # continue at the configured batch size after a successful request.
                        current_batch_size = args.batch_size
                        continue

                    failed_batches += 1
                    print("FAILED GraphQL batch; no data/checkpoints committed: " +
                          json.dumps(errors[:3], ensure_ascii=False)[:1200], file=sys.stderr, flush=True)
                    break

                # A successful request restores the normal configured batch size.
                current_batch_size = args.batch_size
                data = payload.get("data") or {}

                rate = data.get("rateLimit") or {}

                points = int(rate.get("cost") or 0)

                total_points += points



                batch_records: list[tuple] = []

                batch_progress: dict[str, dict[str, Any]] = {}

                batch_issue_count = 0

                batch_pages = 0

                for i, repo in enumerate(batch):

                    node = data.get(f"r{i}")

                    if node is None:

                        raise RuntimeError(f"GraphQL returned null repository for {repo['full_name']}; aborting batch.")

                    issue_conn = node.get("issues") or {}

                    page_info = issue_conn.get("pageInfo") or {}

                    nodes = issue_conn.get("nodes") or []

                    if len(nodes) > args.page_size:

                        raise RuntimeError("Returned more issues than requested; refusing to commit.")

                    for issue in nodes:

                        issue_id = issue.get("databaseId")

                        issue_number = issue.get("number")

                        if issue_id is None or issue_number is None:

                            raise RuntimeError(f"Missing numeric issue ID/number in {repo['full_name']}; refusing batch.")

                        labels_conn = issue.get("labels") or {}

                        labels = [x["name"] for x in (labels_conn.get("nodes") or []) if x and x.get("name")]

                        if (labels_conn.get("pageInfo") or {}).get("hasNextPage"):

                            print(f"WARNING: {repo['full_name']} issue #{issue_number} has >100 labels; only first 100 were read.")

                        body = issue.get("body") or ""

                        score, difficulty, has_valid, skills, technologies, tags = extract_metadata(

                            labels, body, repo["language"]

                        )

                        author = (issue.get("author") or {}).get("login")

                        updated_at = issue.get("updatedAt")

                        if not updated_at:

                            raise RuntimeError(f"Missing updatedAt for {repo['full_name']}#{issue_number}")

                        batch_records.append((

                            int(issue_id), repo["id"], int(issue_number), issue.get("title") or "",

                            body, issue.get("url") or "", issue.get("state") or "OPEN",

                            score, difficulty, skills, technologies, tags,

                            int((issue.get("comments") or {}).get("totalCount") or 0),

                            bool(has_valid), author, updated_at,

                        ))

                    batch_issue_count += len(nodes)

                    batch_pages += 1

                    has_next = bool(page_info.get("hasNextPage"))

                    next_cursor = page_info.get("endCursor")

                    if has_next and not next_cursor:

                        raise RuntimeError(f"hasNextPage true but no endCursor for {repo['full_name']}")

                    batch_progress[repo["full_name"]] = {

                        "cursor": next_cursor if has_next else None,

                        "pages_done": repo["pages_done"] + 1,

                        "done": not has_next,

                        "last_total_count": int(issue_conn.get("totalCount") or 0),

                    }



                db_rows = 0

                if args.write:

                    try:

                        with conn.cursor() as cur:

                            if batch_records:

                                execute_values(cur, """

                                    INSERT INTO issues (

                                      github_id, repository_id, number, title, body, url, state,

                                      difficulty_score, difficulty, required_skills, technologies,

                                      learning_tags, comments, has_difficulty_label, author, updated_at

                                    ) VALUES %s

                                    ON CONFLICT (repository_id, number) DO UPDATE SET

                                      github_id = EXCLUDED.github_id,

                                      title = EXCLUDED.title,

                                      body = EXCLUDED.body,

                                      url = EXCLUDED.url,

                                      state = EXCLUDED.state,

                                      difficulty_score = EXCLUDED.difficulty_score,

                                      difficulty = EXCLUDED.difficulty,

                                      required_skills = EXCLUDED.required_skills,

                                      technologies = EXCLUDED.technologies,

                                      learning_tags = EXCLUDED.learning_tags,

                                      comments = EXCLUDED.comments,

                                      has_difficulty_label = EXCLUDED.has_difficulty_label,

                                      author = EXCLUDED.author,

                                      updated_at = EXCLUDED.updated_at

                                """, batch_records, page_size=500)

                                db_rows = len(batch_records)

                        conn.commit()

                    except Exception:

                        conn.rollback()

                        raise

                    # Checkpoint only after DB commit. If checkpoint write fails, re-running is idempotent.

                    for repo in batch:

                        prog = batch_progress[repo["full_name"]]

                        repo.update(prog)

                        checkpoint[repo["full_name"]] = prog

                    atomic_write_json(checkpoint_path, checkpoint)

                else:

                    # Dry run intentionally does not advance cursors; rerun cannot silently skip unpersisted data.

                    for repo in batch:

                        repo.update(batch_progress[repo["full_name"]])



                total_pages += batch_pages

                total_issues += batch_issue_count

                total_db_rows += db_rows

                metric = {

                    "timestamp": datetime.now(timezone.utc).isoformat(),

                    "mode": "write" if args.write else "dry-run",

                    "request_seconds": round(elapsed, 4),

                    "repos_in_batch": len(batch),

                    "pages": batch_pages,

                    "issues_fetched": batch_issue_count,

                    "rows_upserted": db_rows,

                    "response_bytes": len(response.content),

                    "graphql_cost": points,

                    "rate_remaining": rate.get("remaining"),

                    "graphql_errors": 0,

                    "total_pages_run": total_pages,

                    "total_issues_run": total_issues,

                }

                with metrics_path.open("a", encoding="utf-8") as mf:

                    mf.write(json.dumps(metric) + "\n")

                print(

                    f"OK repos={len(batch)} "

                    f"pages={batch_pages} issues={batch_issue_count} upserts={db_rows} "

                    f"cost={points} remaining={rate.get('remaining')} "

                    f"seconds={elapsed:.2f} bytes={len(response.content)}",

                    flush=True,

                )

                if args.pause:

                    time.sleep(args.pause)

            except Exception as exc:

                conn.rollback()

                failed_batches += 1

                print(f"FAILED batch: {exc}", file=sys.stderr, flush=True)

                break



        duration = time.perf_counter() - run_started

        print("\n=== SkillIssues Importer v6 summary ===")

        print(f"mode: {'WRITE' if args.write else 'DRY-RUN'}")

        print(f"repositories selected: {len(repos)}")

        print(f"pages processed: {total_pages}")

        print(f"issues fetched: {total_issues}")

        print(f"rows upserted: {total_db_rows}")

        print(f"GraphQL points reported: {total_points}")

        print(f"response bytes: {total_bytes}")

        print(f"failed batches: {failed_batches}")

        print(f"wall time: {duration:.2f}s")

        print(f"issue throughput: {total_issues / duration:.2f} issues/s" if duration else "issue throughput: n/a")

        print(f"checkpoint: {checkpoint_path.resolve() if args.write else '(not changed in dry-run)'}")

        print(f"metrics: {metrics_path.resolve()}")

    finally:

        conn.close()





if __name__ == "__main__":

    main()
