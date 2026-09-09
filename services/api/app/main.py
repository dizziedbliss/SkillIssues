from contextlib import asynccontextmanager
from pathlib import Path
import os
import re
from typing import Any

import httpx
import psycopg
from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://skillissues:skillissues@localhost:5432/skillissues")
GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", "")
RECOMMENDATION_URL = os.getenv("RECOMMENDATION_URL", "http://recommendation:8000")
SCHEMA = Path(os.getenv("SCHEMA_PATH", "/database/schema.sql"))
SEED = Path(os.getenv("SEED_PATH", "/database/seed/seed.sql"))


def db() -> psycopg.Connection:
    return psycopg.connect(DATABASE_URL)


def query(sql: str, params: tuple[Any, ...] = ()) -> list[dict[str, Any]]:
    with db() as connection:
        with connection.cursor() as cursor:
            cursor.execute(sql, params)
            if cursor.description is None:
                return []
            columns = [column.name for column in cursor.description]
            return [dict(zip(columns, row)) for row in cursor.fetchall()]


def execute(sql: str, params: tuple[Any, ...] = ()) -> None:
    with db() as connection:
        with connection.cursor() as cursor:
            cursor.execute(sql, params)


def initialize_database() -> None:
    schema = SCHEMA.read_text(encoding="utf-8")
    seed = SEED.read_text(encoding="utf-8")
    with db() as connection:
        with connection.cursor() as cursor:
            cursor.execute(schema)
            cursor.execute(seed)


@asynccontextmanager
async def lifespan(_: FastAPI):
    initialize_database()
    yield


app = FastAPI(title="SkillIssues API", version="0.1.0", lifespan=lifespan)
app.add_middleware(
    CORSMiddleware,
    allow_origins=[os.getenv("FRONTEND_URL", "http://localhost:5173")],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


class ProfileUpdate(BaseModel):
    bio: str = Field(default="", max_length=500)
    experience_level: str = Field(default="BEGINNER", pattern="^(BEGINNER|INTERMEDIATE|ADVANCED)$")
    target_level: str = Field(default="INTERMEDIATE", pattern="^(BEGINNER|INTERMEDIATE|ADVANCED)$")
    interests: list[str] = Field(default_factory=list, max_length=12)
    preferred_languages: list[str] = Field(default_factory=list, max_length=12)


class ContributionStart(BaseModel):
    issue_id: int


class ContributionUpdate(BaseModel):
    pr_url: str = Field(min_length=20, max_length=500)
    pr_number: int = Field(gt=0)


@app.get("/health")
def health() -> dict[str, str]:
    query("SELECT 1")
    return {"status": "ok", "service": "api"}


@app.post("/auth/demo")
def demo_login() -> dict[str, Any]:
    user = query("SELECT id, username, avatar_url FROM users WHERE github_id = 'demo-1'")[0]
    return {"token": "demo-token", "user": user}


@app.get("/auth/github")
def github_login() -> dict[str, Any]:
    client_id = os.getenv("GITHUB_CLIENT_ID", "")
    if not client_id:
        return {"enabled": False, "message": "Demo mode is active. Configure GITHUB_CLIENT_ID for OAuth."}
    return {"enabled": True, "authorize_url": f"https://github.com/login/oauth/authorize?client_id={client_id}&scope=read:user"}


@app.post("/admin/sync/repository/{repository_id}", status_code=202)
def request_repository_sync(repository_id: int) -> dict[str, Any]:
    rows = query("SELECT id, full_name, last_synced_at FROM repositories WHERE id = %s", (repository_id,))
    if not rows:
        raise HTTPException(404, "Repository not found")
    execute("""
        INSERT INTO sync_state (repository_id, priority) VALUES (%s, 'HIGH')
        ON CONFLICT (repository_id) DO UPDATE SET priority = 'HIGH'
    """, (repository_id,))
    return {"accepted": True, "repository": rows[0], "message": "Priority refresh queued for the worker."}


@app.get("/me")
def me() -> dict[str, Any]:
    rows = query("""
        SELECT u.id, u.username, u.avatar_url, p.experience_level, p.target_level,
               p.progress_score, p.completed_count, p.bio, p.demonstrated_skills,
               pref.interests, pref.preferred_languages
        FROM users u JOIN developer_profiles p ON p.user_id = u.id
        JOIN developer_preferences pref ON pref.user_id = u.id
        WHERE u.github_id = 'demo-1'
    """)
    return rows[0]


@app.get("/profile")
def profile() -> dict[str, Any]:
    return me()


@app.put("/profile")
def update_profile(payload: ProfileUpdate) -> dict[str, Any]:
    user_id = me()["id"]
    execute("""
        UPDATE developer_profiles SET bio = %s, experience_level = %s, target_level = %s
        WHERE user_id = %s
    """, (payload.bio, payload.experience_level, payload.target_level, user_id))
    execute("""
        UPDATE developer_preferences SET interests = %s, preferred_languages = %s WHERE user_id = %s
    """, (payload.interests, payload.preferred_languages, user_id))
    return me()


@app.get("/preferences")
def preferences() -> dict[str, Any]:
    user = me()
    return {"interests": user["interests"], "preferred_languages": user["preferred_languages"]}


@app.put("/preferences")
def update_preferences(payload: ProfileUpdate) -> dict[str, Any]:
    return update_profile(payload)


def issue_query(where: str = "", params: tuple[Any, ...] = ()) -> list[dict[str, Any]]:
    return query(f"""
        SELECT i.id, i.github_id, i.number, i.title, i.body, i.url, i.state,
               i.difficulty_score, i.difficulty, i.required_skills, i.technologies,
               i.learning_tags, i.comments, r.full_name AS repository,
               r.url AS repository_url, r.description AS repository_description,
               r.language, r.stars, r.license
        FROM issues i JOIN repositories r ON r.id = i.repository_id
        {where} ORDER BY i.difficulty_score, i.updated_at DESC
    """, params)


@app.get("/issues")
def issues(difficulty: str | None = Query(default=None), limit: int = Query(default=20, le=50)) -> list[dict[str, Any]]:
    where = "WHERE i.state = 'OPEN'"
    params: tuple[Any, ...] = ()
    if difficulty:
        where += " AND i.difficulty = %s"
        params += (difficulty.upper(),)
    return issue_query(where + " LIMIT %s", params + (limit,))


@app.get("/issues/{issue_id}")
def issue(issue_id: int) -> dict[str, Any]:
    rows = issue_query("WHERE i.id = %s", (issue_id,))
    if not rows:
        raise HTTPException(404, "Issue not found")
    return rows[0]


@app.get("/recommendations")
def recommendations(limit: int = Query(default=3, le=10)) -> list[dict[str, Any]]:
    user = me()
    candidates = issue_query("WHERE i.state = 'OPEN'", ())
    try:
        response = httpx.post(f"{RECOMMENDATION_URL}/score", json={"user": user, "issues": candidates}, timeout=5)
        response.raise_for_status()
        return response.json()[:limit]
    except httpx.HTTPError:
        pass
    user_skills = {skill.lower() for skill in (user["demonstrated_skills"] or [])}
    interests = {item.lower() for item in (user["interests"] or [])}
    target = {"BEGINNER": 3, "INTERMEDIATE": 5, "ADVANCED": 7}.get(user["target_level"], 5)
    for item in candidates:
        skills = {skill.lower() for skill in item["required_skills"]}
        tags = {tag.lower() for tag in item["learning_tags"]}
        language_match = item["language"] and item["language"].lower() in {x.lower() for x in user["preferred_languages"]}
        skill_match = len(user_skills & skills) * 2
        interest_match = len(interests & tags)
        difficulty_fit = max(0, 5 - abs(item["difficulty_score"] - target))
        quality = min(3, item["stars"] // 25000 + 1)
        item["recommendation_score"] = skill_match + interest_match + difficulty_fit + quality
        item["why_recommended"] = []
        if skill_match:
            item["why_recommended"].append("Builds on your current skills")
        if interest_match:
            item["why_recommended"].append("Matches your interests")
        if language_match:
            item["why_recommended"].append(f"Uses your preferred language: {item['language']}")
        if item["difficulty_score"] >= target:
            item["why_recommended"].append("Slightly stretches your current level")
        if not item["why_recommended"]:
            item["why_recommended"].append("A strong open-source starting point")
    return sorted(candidates, key=lambda item: item["recommendation_score"], reverse=True)[:limit]


@app.post("/contributions")
def start_contribution(payload: ContributionStart) -> dict[str, Any]:
    user_id = me()["id"]
    if not issue(payload.issue_id):
        raise HTTPException(404, "Issue not found")
    rows = query("""
        INSERT INTO contributions (user_id, issue_id) VALUES (%s, %s)
        ON CONFLICT (user_id, issue_id) DO UPDATE SET updated_at = now()
        RETURNING id, user_id, issue_id, pr_url, pr_number, state, created_at, updated_at, merged_at
    """, (user_id, payload.issue_id))
    return rows[0]


@app.get("/contributions")
def contributions() -> list[dict[str, Any]]:
    return query("""
        SELECT c.*, i.title, r.full_name AS repository FROM contributions c
        JOIN issues i ON i.id = c.issue_id JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s ORDER BY c.updated_at DESC
    """, (me()["id"],))


@app.get("/contributions/{contribution_id}")
def contribution(contribution_id: int) -> dict[str, Any]:
    rows = query("SELECT * FROM contributions WHERE id = %s AND user_id = %s", (contribution_id, me()["id"]))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    return rows[0]


@app.put("/contributions/{contribution_id}")
def attach_pr(contribution_id: int, payload: ContributionUpdate) -> dict[str, Any]:
    if not re.fullmatch(r"https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/\d+", payload.pr_url):
        raise HTTPException(422, "PR URL must be a GitHub pull request URL")
    rows = query("""
        UPDATE contributions SET pr_url = %s, pr_number = %s, state = 'PR_OPEN', updated_at = now()
        WHERE id = %s AND user_id = %s
        RETURNING *
    """, (payload.pr_url, payload.pr_number, contribution_id, me()["id"]))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    return rows[0]


@app.post("/contributions/{contribution_id}/verify")
async def verify_contribution(contribution_id: int) -> dict[str, Any]:
    rows = query("SELECT * FROM contributions WHERE id = %s AND user_id = %s", (contribution_id, me()["id"]))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    contribution_data = rows[0]
    if not GITHUB_TOKEN or not contribution_data["pr_url"]:
        return {"verified": False, "message": "Add GITHUB_TOKEN and a real pull request URL to verify with GitHub."}
    owner_repo = contribution_data["pr_url"].split("github.com/")[-1].split("/pull/")[0]
    pr_number = contribution_data["pr_url"].split("/pull/")[-1].split("/")[0]
    async with httpx.AsyncClient() as client:
        response = await client.get(f"https://api.github.com/repos/{owner_repo}/pulls/{pr_number}", headers={"Authorization": f"Bearer {GITHUB_TOKEN}", "Accept": "application/vnd.github+json"})
    if response.status_code != 200:
        raise HTTPException(response.status_code, "GitHub PR lookup failed")
    pr = response.json()
    if not pr.get("merged"):
        execute("UPDATE contributions SET state = %s, updated_at = now() WHERE id = %s", ("PR_OPEN" if pr.get("state") == "open" else "PR_CLOSED", contribution_id))
        return {"verified": False, "state": pr.get("state"), "message": "GitHub confirms the PR is not merged yet."}
    updated = query("""
        UPDATE contributions SET state = 'PR_MERGED', merged_at = now(), updated_at = now()
        WHERE id = %s AND user_id = %s AND state <> 'PR_MERGED'
        RETURNING issue_id
    """, (contribution_id, me()["id"]))
    if updated:
        execute("""
            UPDATE developer_profiles SET completed_count = completed_count + 1,
              progress_score = LEAST(100, progress_score + 12),
              demonstrated_skills = ARRAY(SELECT DISTINCT unnest(demonstrated_skills || i.required_skills))
            FROM issues i WHERE i.id = %s AND user_id = %s
        """, (updated[0]["issue_id"], me()["id"]))
    return {"verified": True, "state": "PR_MERGED", "message": "Merged contribution verified by GitHub."}


@app.get("/progress")
def progress() -> dict[str, Any]:
    user = me()
    history = contributions()
    return {"progress_score": user["progress_score"], "completed_count": user["completed_count"], "demonstrated_skills": user["demonstrated_skills"], "contributions": history}
