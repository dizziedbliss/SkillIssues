from contextlib import asynccontextmanager
import hashlib
import hmac
import json
import secrets
from pathlib import Path
import os
import re
from urllib.parse import urlencode, urlparse
from typing import Any

import httpx
import psycopg
from fastapi import FastAPI, HTTPException, Query, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import RedirectResponse
from pydantic import BaseModel, Field

DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://skillissues:skillissues@localhost:5432/skillissues")
GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", "")
GITHUB_WEBHOOK_SECRET = os.getenv("GITHUB_WEBHOOK_SECRET", "")
DEMO_MODE = os.getenv("DEMO_MODE", "false").lower() == "true"
RECOMMENDATION_URL = os.getenv("RECOMMENDATION_URL", "http://recommendation:8000")
CONTRIBUTION_URL = os.getenv("CONTRIBUTION_URL", "http://contribution:8000")
ADMIN_SYNC_TOKEN = os.getenv("ADMIN_SYNC_TOKEN", "")
GITHUB_REDIRECT_URI = os.getenv("GITHUB_REDIRECT_URI", "http://localhost:8000/auth/github/callback")
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


def token_digest(token: str) -> str:
    return hashlib.sha256(token.encode("utf-8")).hexdigest()


def base64url(value: bytes) -> str:
    import base64
    return base64.urlsafe_b64encode(value).rstrip(b"=").decode("ascii")


def initialize_database() -> None:
    schema = SCHEMA.read_text(encoding="utf-8")
    seed = SEED.read_text(encoding="utf-8")
    with db() as connection:
        with connection.cursor() as cursor:
            cursor.execute(schema)
            cursor.execute("ALTER TABLE issues ADD COLUMN IF NOT EXISTS has_difficulty_label BOOLEAN NOT NULL DEFAULT false;")
            cursor.execute("ALTER TABLE contributions ADD COLUMN IF NOT EXISTS xp_awarded BOOLEAN NOT NULL DEFAULT false;")
            cursor.execute(seed)
            cursor.execute("UPDATE issues SET has_difficulty_label = true WHERE github_id IN (2001, 2002, 2003, 999002) OR repository_id IN (SELECT id FROM repositories WHERE full_name = 'dizziedbliss/listtty');")


@asynccontextmanager
async def lifespan(_: FastAPI):
    initialize_database()
    if not DEMO_MODE and not GITHUB_WEBHOOK_SECRET:
        raise RuntimeError("GITHUB_WEBHOOK_SECRET is required when DEMO_MODE=false")
    if not DEMO_MODE and not ADMIN_SYNC_TOKEN:
        raise RuntimeError("ADMIN_SYNC_TOKEN is required when DEMO_MODE=false")
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


class SavedIssue(BaseModel):
    issue_id: int


class ForkRequest(BaseModel):
    repository: str


class PullRequestSubmit(BaseModel):
    branch: str = Field(min_length=1, max_length=100)


def authenticated_user_id(request: Request) -> int:
    authorization = request.headers.get("Authorization", "")
    token = authorization.removeprefix("Bearer ").strip() if authorization.startswith("Bearer ") else request.cookies.get("skillissues_session", "")
    if token:
        rows = query("SELECT user_id FROM auth_sessions WHERE token = %s AND expires_at > now()", (token_digest(token),))
        if rows:
            return rows[0]["user_id"]
    raise HTTPException(401, "Authentication required")


@app.get("/health")
def health() -> dict[str, str]:
    query("SELECT 1")
    return {"status": "ok", "service": "api"}


@app.get("/health/detail")
def health_detail() -> dict[str, Any]:
    counts = query("""
        SELECT (SELECT count(*) FROM repositories) AS repositories,
               (SELECT count(*) FROM issues) AS issues,
               (SELECT count(*) FROM contributions) AS contributions
    """)[0]
    return {"status": "ok", "service": "api", "database": "ok", "counts": counts}


@app.post("/auth/demo")
def demo_login(response: Response) -> dict[str, Any]:
    if not DEMO_MODE:
        raise HTTPException(404, "Demo authentication is disabled")
    user = query("""
        INSERT INTO users (github_id, username, avatar_url)
        VALUES ('demo-1', 'alex-dev', 'https://github.com/identicons/alex-dev.png')
        ON CONFLICT (github_id) DO UPDATE SET username = EXCLUDED.username
        RETURNING id, username, avatar_url
    """)[0]
    execute("INSERT INTO developer_profiles (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user["id"],))
    execute("INSERT INTO developer_preferences (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user["id"],))
    token = secrets.token_urlsafe(32)
    execute("INSERT INTO auth_sessions (token, user_id, provider) VALUES (%s, %s, 'demo')", (token_digest(token), user["id"]))
    response.set_cookie("skillissues_session", token, httponly=True, samesite="lax", max_age=604800)
    return {"user": user}


@app.get("/auth/session")
def auth_session(request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("SELECT id, username, avatar_url FROM users WHERE id = %s", (user_id,))
    if not rows:
        raise HTTPException(401, "Session user not found")
    return {"authenticated": True, "user": rows[0]}


@app.post("/auth/logout")
def logout(request: Request) -> dict[str, bool]:
    authorization = request.headers.get("Authorization", "")
    if authorization.startswith("Bearer "):
        execute("DELETE FROM auth_sessions WHERE token = %s", (token_digest(authorization.removeprefix("Bearer ").strip()),))
    if request.cookies.get("skillissues_session"):
        execute("DELETE FROM auth_sessions WHERE token = %s", (token_digest(request.cookies["skillissues_session"]),))
    return {"logged_out": True}


@app.post("/auth/purge")
def purge_user(request: Request) -> dict[str, bool]:
    try:
        user_id = authenticated_user_id(request)
        execute("DELETE FROM auth_sessions WHERE user_id = %s", (user_id,))
        execute("DELETE FROM users WHERE id = %s AND github_id != 'demo-1'", (user_id,))
    except Exception:
        pass
    return {"purged": True}


@app.get("/auth/github")
@app.post("/auth/github")
def github_login(response: Response) -> dict[str, Any]:
    client_id = os.getenv("GITHUB_CLIENT_ID", "")
    if not client_id:
        return {"enabled": False, "message": "OAuth Client ID not configured. Use Personal Access Token option to Connect GitHub."}
    state = secrets.token_urlsafe(32)
    verifier = secrets.token_urlsafe(64)
    challenge = base64url(hashlib.sha256(verifier.encode("ascii")).digest())
    execute("INSERT INTO oauth_states (state, code_verifier, redirect_uri) VALUES (%s, %s, %s)", (state, verifier, GITHUB_REDIRECT_URI))
    response.set_cookie("skillissues_oauth_state", state, httponly=True, samesite="lax", max_age=600)
    params = urlencode({"client_id": client_id, "redirect_uri": GITHUB_REDIRECT_URI, "scope": "read:user user:email public_repo", "state": state, "code_challenge": challenge, "code_challenge_method": "S256", "prompt": "consent"})
    return {
        "enabled": True,
        "authorize_url": f"https://github.com/login/oauth/authorize?{params}",
        "redirect_uri": GITHUB_REDIRECT_URI,
    }


class TokenAuthRequest(BaseModel):
    token: str = Field(min_length=10, max_length=200)


@app.post("/auth/github/token")
async def github_token_login(payload: TokenAuthRequest, response: Response) -> dict[str, Any]:
    token = payload.token.strip()
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}
    async with httpx.AsyncClient(timeout=10) as client:
        user_response = await client.get("https://api.github.com/user", headers=headers)
        if user_response.status_code != 200:
            raise HTTPException(401, "Invalid GitHub Personal Access Token. Ensure the token has public_repo scope.")
        github_user = user_response.json()
    user = query("""INSERT INTO users (github_id, username, avatar_url, github_access_token) VALUES (%s, %s, %s, %s)
        ON CONFLICT (github_id) DO UPDATE SET username = EXCLUDED.username, avatar_url = EXCLUDED.avatar_url, github_access_token = EXCLUDED.github_access_token
        RETURNING id, username, avatar_url""", (str(github_user["id"]), github_user["login"], github_user.get("avatar_url"), token))[0]
    execute("UPDATE users SET github_access_token = %s WHERE id = %s", (token, user["id"]))
    execute("INSERT INTO developer_profiles (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user["id"],))
    execute("INSERT INTO developer_preferences (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user["id"],))
    session_token = secrets.token_urlsafe(32)
    execute("INSERT INTO auth_sessions (token, user_id, provider) VALUES (%s, %s, 'github')", (token_digest(session_token), user["id"]))
    response.set_cookie("skillissues_session", session_token, httponly=True, samesite="lax", max_age=604800)
    return {"authenticated": True, "user": user, "session_token": session_token}


@app.get("/auth/github/callback")
async def github_callback(request: Request, code: str | None = None, state: str | None = None, error: str | None = None) -> Response:
    if error:
        raise HTTPException(400, f"GitHub authorization failed: {error}")
    if not code or not state:
        raise HTTPException(400, "Invalid request: code and state are required")
    state_rows = query("DELETE FROM oauth_states WHERE state = %s AND expires_at > now() RETURNING code_verifier, redirect_uri", (state,))
    if not state_rows or not os.getenv("GITHUB_CLIENT_ID") or not os.getenv("GITHUB_CLIENT_SECRET"):
        raise HTTPException(400, "Invalid or expired OAuth state, or OAuth is not configured")
    async with httpx.AsyncClient(timeout=10) as client:
        response = await client.post("https://github.com/login/oauth/access_token", data={
            "client_id": os.environ["GITHUB_CLIENT_ID"],
            "client_secret": os.environ["GITHUB_CLIENT_SECRET"],
            "code": code,
            "redirect_uri": state_rows[0]["redirect_uri"],
            "code_verifier": state_rows[0]["code_verifier"],
        }, headers={"Accept": "application/json"})
        response.raise_for_status()
        token_data = response.json()
        access_token = token_data.get("access_token")
        if not access_token:
            raise HTTPException(400, "GitHub did not return an access token")
        user_response = await client.get("https://api.github.com/user", headers={"Authorization": f"Bearer {access_token}", "Accept": "application/vnd.github+json"})
        user_response.raise_for_status()
        github_user = user_response.json()
    user = query("""INSERT INTO users (github_id, username, avatar_url, github_access_token) VALUES (%s, %s, %s, %s)
        ON CONFLICT (github_id) DO UPDATE SET username = EXCLUDED.username, avatar_url = EXCLUDED.avatar_url, github_access_token = EXCLUDED.github_access_token
        RETURNING id, username, avatar_url""", (str(github_user["id"]), github_user["login"], github_user.get("avatar_url"), access_token))[0]
    execute("UPDATE users SET github_access_token = %s WHERE id = %s", (access_token, user["id"]))
    execute("INSERT INTO developer_profiles (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user["id"],))
    execute("INSERT INTO developer_preferences (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user["id"],))
    session_token = secrets.token_urlsafe(32)
    execute("INSERT INTO auth_sessions (token, user_id, provider) VALUES (%s, %s, 'github')", (token_digest(session_token), user["id"]))
    redirect = RedirectResponse(url=os.getenv("FRONTEND_URL", "http://localhost:5173"), status_code=303)
    redirect.delete_cookie("skillissues_oauth_state")
    redirect.set_cookie("skillissues_session", session_token, httponly=True, samesite="lax", max_age=604800)
    return redirect


@app.post("/github/forks")
async def create_or_get_fork(payload: ForkRequest, request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("SELECT username, github_access_token FROM users WHERE id = %s", (user_id,))
    token = (rows[0]["github_access_token"] if rows and rows[0]["github_access_token"] else None) or GITHUB_TOKEN
    if not token:
        raise HTTPException(403, "Please click 'Connect GitHub' in top bar to authorize repository forking and pushing.")
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}
    username = rows[0]["username"] if rows and rows[0]["username"] and rows[0]["username"] != "alex-dev" else None
    owner, separator, name = payload.repository.partition("/")
    if not separator:
        raise HTTPException(422, "Invalid repository name")
    async with httpx.AsyncClient(timeout=20) as client:
        if not username:
            user_res = await client.get("https://api.github.com/user", headers=headers)
            if user_res.status_code == 200:
                gh_user = user_res.json()
                username = gh_user.get("login")
                if username and user_id:
                    execute("UPDATE users SET username = %s, github_access_token = %s WHERE id = %s", (username, token, user_id))
        if not username:
            raise HTTPException(403, "Could not determine GitHub username. Please click 'Connect GitHub' to re-authorize.")
        existing = await client.get(f"https://api.github.com/repos/{username}/{name}", headers=headers)
        if existing.status_code == 200:
            fork = existing.json()
        else:
            response = await client.post(f"https://api.github.com/repos/{owner}/{name}/forks", headers=headers, json={"default_branch_only": True})
            if response.status_code in (201, 202):
                fork = response.json()
            elif response.status_code == 422:
                import asyncio
                await asyncio.sleep(1.5)
                retry_existing = await client.get(f"https://api.github.com/repos/{username}/{name}", headers=headers)
                if retry_existing.status_code == 200:
                    fork = retry_existing.json()
                else:
                    raise HTTPException(422, f"Fork for {username}/{name} is being processed on GitHub. Try again in a moment.")
            elif response.status_code in (401, 403, 404):
                raise HTTPException(403, "GitHub permission denied. Please click 'Connect GitHub' in top bar to grant public_repo permissions.")
            else:
                raise HTTPException(response.status_code, f"GitHub could not create a fork: {response.text[:200]}")
    return {"full_name": fork["full_name"], "clone_url": fork["clone_url"], "html_url": fork["html_url"]}


@app.get("/github/git-token")
def github_git_token(request: Request) -> dict[str, str]:
    rows = query("SELECT github_access_token FROM users WHERE id = %s", (authenticated_user_id(request),))
    token = (rows[0]["github_access_token"] if rows and rows[0]["github_access_token"] else None) or GITHUB_TOKEN
    if not token:
        raise HTTPException(403, "Connect GitHub before pushing changes.")
    return {"token": token}


@app.post("/contributions/{contribution_id}/submit")
async def submit_pull_request(contribution_id: int, payload: PullRequestSubmit, request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("""
        SELECT c.id, c.issue_id, u.username, u.github_access_token, i.title, i.body,
               i.number, r.full_name
        FROM contributions c JOIN users u ON u.id = c.user_id
        JOIN issues i ON i.id = c.issue_id JOIN repositories r ON r.id = i.repository_id
        WHERE c.id = %s AND c.user_id = %s
    """, (contribution_id, user_id))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    token = (rows[0]["github_access_token"] if rows and rows[0]["github_access_token"] else None) or GITHUB_TOKEN
    if not token:
        raise HTTPException(403, "Connect GitHub before submitting a pull request.")
    contribution = rows[0]
    username = contribution["username"] if contribution["username"] and contribution["username"] != "alex-dev" else None
    owner, _, name = contribution["full_name"].partition("/")
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}

    async with httpx.AsyncClient(timeout=20) as client:
        if not username:
            user_res = await client.get("https://api.github.com/user", headers=headers)
            if user_res.status_code == 200:
                username = user_res.json().get("login")
        repository_response = await client.get(f"https://api.github.com/repos/{owner}/{name}", headers=headers)
        base = repository_response.json().get("default_branch", "main") if repository_response.status_code == 200 else "main"

        # 1. Try creating PR on upstream repo
        pr_payload = {
            "title": contribution["title"],
            "head": f"{username}:{payload.branch}" if username else payload.branch,
            "base": base,
            "body": contribution["body"] or f"SkillIssues contribution for issue #{contribution['number']}",
        }
        response = await client.post(f"https://api.github.com/repos/{owner}/{name}/pulls", headers=headers, json=pr_payload)

        if response.status_code == 201:
            pull_request = response.json()
            pr_url = pull_request["html_url"]
            pr_number = pull_request["number"]
        else:
            # 2. Try creating PR on user's fork
            fork_pr_res = await client.post(f"https://api.github.com/repos/{username}/{name}/pulls", headers=headers, json={
                "title": contribution["title"],
                "head": payload.branch,
                "base": base,
                "body": contribution["body"] or f"SkillIssues contribution for issue #{contribution['number']}",
            }) if username else None

            if fork_pr_res and fork_pr_res.status_code == 201:
                pull_request = fork_pr_res.json()
                pr_url = pull_request["html_url"]
                pr_number = pull_request["number"]
            else:
                # 3. Fallback: generate GitHub 1-click Compare & PR web URL
                pr_url = f"https://github.com/{owner}/{name}/compare/{base}...{username}:{payload.branch}?expand=1" if username else f"https://github.com/{owner}/{name}"
                pr_number = contribution["number"]
                pull_request = {"html_url": pr_url, "number": pr_number, "manual_compare": True}

    updated = query("UPDATE contributions SET pr_url = %s, pr_number = %s, state = 'PR_OPEN', updated_at = now() WHERE id = %s RETURNING *", (pr_url, pr_number, contribution_id))[0]
    return {"contribution": updated, "pull_request": pull_request, "web_url": pr_url}


@app.post("/webhooks/github")
async def github_webhook(request: Request) -> dict[str, Any]:
    body = await request.body()
    signature = request.headers.get("x-hub-signature-256", "")
    delivery = request.headers.get("x-github-delivery", secrets.token_hex(8))
    event_name = request.headers.get("x-github-event", "unknown")
    if not GITHUB_WEBHOOK_SECRET:
        if not DEMO_MODE:
            raise HTTPException(503, "Webhook verification is not configured")
    else:
        expected = "sha256=" + hmac.new(GITHUB_WEBHOOK_SECRET.encode(), body, hashlib.sha256).hexdigest()
        if not hmac.compare_digest(signature, expected):
            raise HTTPException(401, "Invalid webhook signature")
    inserted = query("""INSERT INTO webhook_events (delivery_id, event_name) VALUES (%s, %s)
        ON CONFLICT (delivery_id) DO NOTHING RETURNING delivery_id""", (delivery, event_name))
    if inserted and event_name == "pull_request":
        payload = json.loads(body or b"{}")
        pull_request = payload.get("pull_request", {})
        if pull_request.get("merged") and pull_request.get("html_url"):
            apply_merged_pr(pull_request["html_url"])
    return {"accepted": True, "delivery_id": delivery, "event": event_name}


@app.post("/admin/sync/repository/{repository_id}", status_code=202)
def request_repository_sync(repository_id: int, request: Request) -> dict[str, Any]:
    if ADMIN_SYNC_TOKEN:
        authorization = request.headers.get("Authorization", "")
        supplied = authorization.removeprefix("Bearer ").strip() if authorization.startswith("Bearer ") else ""
        if not supplied or not hmac.compare_digest(supplied, ADMIN_SYNC_TOKEN):
            raise HTTPException(401, "Admin authentication required")
    elif not DEMO_MODE:
        raise HTTPException(503, "Admin sync is not configured")
    rows = query("SELECT id, full_name, last_synced_at FROM repositories WHERE id = %s", (repository_id,))
    if not rows:
        raise HTTPException(404, "Repository not found")
    execute("""
        INSERT INTO sync_state (repository_id, priority) VALUES (%s, 'HIGH')
        ON CONFLICT (repository_id) DO UPDATE SET priority = 'HIGH'
    """, (repository_id,))
    return {"accepted": True, "repository": rows[0], "message": "Priority refresh queued for the worker."}


@app.get("/me")
def me(request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("""
        SELECT u.id, u.username, u.avatar_url, p.experience_level, p.target_level,
               p.progress_score, p.completed_count, p.bio, p.demonstrated_skills,
               pref.interests, pref.preferred_languages
        FROM users u JOIN developer_profiles p ON p.user_id = u.id
        JOIN developer_preferences pref ON pref.user_id = u.id
        WHERE u.id = %s
    """, (user_id,))
    if not rows:
        raise HTTPException(401, "Profile not found")
    return rows[0]


@app.get("/profile")
def profile(request: Request) -> dict[str, Any]:
    return me(request)


@app.put("/profile")
def update_profile(payload: ProfileUpdate, request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    execute("""
        UPDATE developer_profiles SET bio = %s, experience_level = %s, target_level = %s
        WHERE user_id = %s
    """, (payload.bio, payload.experience_level, payload.target_level, user_id))
    execute("""
        UPDATE developer_preferences SET interests = %s, preferred_languages = %s WHERE user_id = %s
    """, (payload.interests, payload.preferred_languages, user_id))
    return me(request)


@app.get("/preferences")
def preferences(request: Request) -> dict[str, Any]:
    user = me(request)
    return {"interests": user["interests"], "preferred_languages": user["preferred_languages"]}


@app.put("/preferences")
def update_preferences(payload: ProfileUpdate, request: Request) -> dict[str, Any]:
    return update_profile(payload, request)


def issue_query(where: str = "", params: tuple[Any, ...] = ()) -> list[dict[str, Any]]:
    return query(f"""
        SELECT i.id, i.github_id, i.number, i.title, i.body, i.url, i.state, i.updated_at::text AS updated_at,
               i.difficulty_score, i.difficulty, i.required_skills, i.technologies,
               i.learning_tags, i.comments, r.full_name AS repository,
               r.url AS repository_url, r.description AS repository_description,
               r.language, r.stars, r.license
        FROM issues i JOIN repositories r ON r.id = i.repository_id
        {where} ORDER BY i.difficulty_score, i.updated_at DESC
    """, params)


BEGINNER_LABELS = {
    "good first issue", "beginner-friendly", "easy", "starter", "starter-bug",
    "first-timers-only", "newbie", "up-for-grabs",
    "low-hanging-fruit", "bitesize", "trivial", "easy-fix", "good-for-beginner",
    "documentation", "docs", "typo", "good-first-pr"
}

INTERMEDIATE_LABELS = {
    "help wanted", "contributions-welcome", "seeking-contributors",
    "medium", "intermediate", "size/m", "effort/medium", "difficulty/medium",
    "enhancement", "feature", "refactoring", "unit-test"
}

ADVANCED_LABELS = {
    "hard", "advanced", "complex", "size/l", "size/xl", "difficulty/hard",
    "breaking-change", "architecture", "security", "performance", "optimization",
    "critical", "high-priority", "blocker", "p1"
}


def classify_issue_labels(labels: list[str]) -> tuple[bool, str, int]:
    normalized = [str(l).strip().lower() for l in labels if l]
    for l in normalized:
        if l in BEGINNER_LABELS or any(l.startswith(p) for p in ("difficulty/easy", "difficulty/beginner", "size/small", "size/xs", "effort/low", "exp/beginner")):
            return True, "BEGINNER", 2
    for l in normalized:
        if l in INTERMEDIATE_LABELS or any(l.startswith(p) for p in ("difficulty/medium", "size/m", "effort/medium", "exp/intermediate")):
            return True, "INTERMEDIATE", 5
    for l in normalized:
        if l in ADVANCED_LABELS or any(l.startswith(p) for p in ("difficulty/hard", "difficulty/expert", "size/l", "size/xl", "effort/high", "exp/expert")):
            return True, "ADVANCED", 8
    return False, "BEGINNER", 3


last_github_sync_time: dict[str, float] = {}


USER_ISSUES_GRAPHQL_QUERY = """
query UserReposAndIssues($username: String!) {
    repositoryOwner(login: $username) {
        repositories(first: 10, orderBy: {field: UPDATED_AT, direction: DESC}, isFork: false) {
            nodes {
                databaseId
                nameWithOwner
                url
                description
                primaryLanguage { name }
                stargazerCount
                licenseInfo { spdxId }
                issues(first: 20, states: OPEN, orderBy: {field: UPDATED_AT, direction: DESC}) {
                    nodes {
                        databaseId
                        number
                        title
                        body
                        url
                        comments { totalCount }
                        labels(first: 10) { nodes { name } }
                    }
                }
            }
        }
    }
}
"""


async def sync_github_issues_for_user(user_id: int) -> int:
    import time
    now = time.time()
    user_key = f"user_{user_id}"
    if now - last_github_sync_time.get(user_key, 0) < 15:
        return 0
    last_github_sync_time[user_key] = now

    rows = query("SELECT username, github_access_token FROM users WHERE id = %s", (user_id,))
    if not rows:
        return 0
    username = rows[0]["username"]
    token = (rows[0]["github_access_token"] if rows[0]["github_access_token"] else None) or GITHUB_TOKEN
    if not username or username == "alex-dev":
        username = "dizziedbliss"
    if not token:
        token = GITHUB_TOKEN
    if not token:
        return 0

    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    synced_count = 0

    try:
        async with httpx.AsyncClient(timeout=10) as client:
            graphql_res = await client.post(
                "https://api.github.com/graphql",
                headers=headers,
                json={"query": USER_ISSUES_GRAPHQL_QUERY, "variables": {"username": username}},
            )
            if graphql_res.status_code != 200:
                return 0
            payload = graphql_res.json()
            if payload.get("errors") or not payload.get("data"):
                return 0

            owner_data = payload["data"].get("repositoryOwner")
            if not owner_data or not owner_data.get("repositories"):
                return 0

            for repo_node in owner_data["repositories"]["nodes"]:
                full_name = repo_node.get("nameWithOwner")
                if not full_name:
                    continue
                primary_lang = (repo_node.get("primaryLanguage") or {}).get("name") or "Python"
                license_spdx = (repo_node.get("licenseInfo") or {}).get("spdxId") or "MIT"

                repo_rows = query("""
                    INSERT INTO repositories (github_id, full_name, url, description, language, stars, license)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (full_name) DO UPDATE SET
                      description = EXCLUDED.description, language = EXCLUDED.language, stars = EXCLUDED.stars
                    RETURNING id
                """, (
                    repo_node.get("databaseId"), full_name, repo_node.get("url"),
                    repo_node.get("description") or "", primary_lang,
                    repo_node.get("stargazerCount", 0), license_spdx
                ))
                repo_id = repo_rows[0]["id"]

                raw_issues = (repo_node.get("issues") or {}).get("nodes", [])
                for gh_issue in raw_issues:
                    number = gh_issue.get("number")
                    title = gh_issue.get("title")
                    body = gh_issue.get("body") or ""
                    issue_url = gh_issue.get("url")
                    comments = (gh_issue.get("comments") or {}).get("totalCount", 0)
                    labels = [l.get("name", "") for l in (gh_issue.get("labels") or {}).get("nodes", []) if isinstance(l, dict)]
                    has_valid, difficulty, score = classify_issue_labels(labels)
                    if full_name.lower() == "dizziedbliss/listtty" or full_name.lower().startswith(username.lower() + "/"):
                        has_valid = True

                    skills = [primary_lang]
                    technologies = [primary_lang]
                    tags = labels[:4] if labels else ["open-source", "demo"]

                    execute("""
                        INSERT INTO issues
                          (github_id, repository_id, number, title, body, url, state, difficulty_score,
                           difficulty, required_skills, technologies, learning_tags, comments, has_difficulty_label, updated_at)
                        VALUES (%s, %s, %s, %s, %s, %s, 'OPEN', %s, %s, %s, %s, %s, %s, %s, now())
                        ON CONFLICT (github_id) DO UPDATE SET
                          title = EXCLUDED.title, body = EXCLUDED.body, url = EXCLUDED.url,
                          difficulty_score = EXCLUDED.difficulty_score, difficulty = EXCLUDED.difficulty,
                          comments = EXCLUDED.comments, has_difficulty_label = EXCLUDED.has_difficulty_label, updated_at = now()
                    """, (
                        gh_issue.get("databaseId"), repo_id, number, title, body, issue_url,
                        score, difficulty, skills, technologies, tags, comments, has_valid
                    ))
                    synced_count += 1
    except Exception:
        pass

    return synced_count


@app.post("/issues/sync")
async def sync_issues_endpoint(request: Request) -> dict[str, Any]:
    try:
        user_id = authenticated_user_id(request)
        count = await sync_github_issues_for_user(user_id)
        return {"synced": True, "count": count, "message": f"Synced {count} live issues from GitHub repositories."}
    except Exception as error:
        return {"synced": False, "error": str(error)}


@app.get("/issues")
async def issues(request: Request, difficulty: str | None = Query(default=None), language: str | None = Query(default=None), search: str | None = Query(default=None), limit: int = Query(default=20, le=50)) -> list[dict[str, Any]]:
    try:
        user_id = authenticated_user_id(request)
        await sync_github_issues_for_user(user_id)
    except Exception:
        pass
    where = "WHERE i.state = 'OPEN' AND (i.has_difficulty_label = true OR r.full_name = 'dizziedbliss/listtty')"
    params: tuple[Any, ...] = ()
    if difficulty:
        where += " AND i.difficulty = %s"
        params += (difficulty.upper(),)
    if language:
        where += " AND lower(r.language) = lower(%s)"
        params += (language,)
    if search:
        where += " AND (i.title ILIKE %s OR i.body ILIKE %s OR r.full_name ILIKE %s)"
        term = f"%{search}%"
        params += (term, term, term)
    return issue_query(where, params)[:limit]


@app.get("/issues/{issue_id}")
def issue(issue_id: int) -> dict[str, Any]:
    rows = issue_query("WHERE i.id = %s", (issue_id,))
    if not rows:
        raise HTTPException(404, "Issue not found")
    return rows[0]


@app.get("/saved")
def saved_issues(request: Request) -> list[dict[str, Any]]:
    return issue_query("WHERE i.id IN (SELECT issue_id FROM saved_issues WHERE user_id = %s)", (authenticated_user_id(request),))


@app.post("/saved")
def save_issue(payload: SavedIssue, request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    if not query("SELECT id FROM issues WHERE id = %s", (payload.issue_id,)):
        raise HTTPException(404, "Issue not found")
    execute("INSERT INTO saved_issues (user_id, issue_id) VALUES (%s, %s) ON CONFLICT DO NOTHING", (user_id, payload.issue_id))
    return {"saved": True, "issue_id": payload.issue_id}


@app.delete("/saved/{issue_id}")
def unsave_issue(issue_id: int, request: Request) -> dict[str, Any]:
    execute("DELETE FROM saved_issues WHERE user_id = %s AND issue_id = %s", (authenticated_user_id(request), issue_id))
    return {"saved": False, "issue_id": issue_id}


@app.get("/recommendations")
async def recommendations(request: Request, limit: int = Query(default=3, le=50)) -> list[dict[str, Any]]:
    user = me(request)
    try:
        await sync_github_issues_for_user(user["id"])
    except Exception:
        pass
    candidates = issue_query("WHERE i.state = 'OPEN' AND (i.has_difficulty_label = true OR r.full_name = 'dizziedbliss/listtty')", ())
    preferred_set = {x.lower() for x in (user.get("preferred_languages") or [])}
    user_skills = {x.lower() for x in (user.get("demonstrated_skills") or [])}
    interests = {x.lower() for x in (user.get("interests") or [])}
    target_map = {"BEGINNER": 1, "INTERMEDIATE": 3, "ADVANCED": 5}
    target = target_map.get(user.get("target_level", "INTERMEDIATE"), 3)

    if preferred_set:
        filtered = [
            item for item in candidates
            if (item.get("language") and item["language"].lower() in preferred_set)
            or bool(preferred_set & ({t.lower() for t in (item.get("technologies") or [])} | {s.lower() for s in (item.get("required_skills") or [])} | {g.lower() for g in (item.get("learning_tags") or [])}))
        ]
        if filtered:
            candidates = filtered

    try:
        response = httpx.post(f"{RECOMMENDATION_URL}/score", json={"user": user, "issues": candidates}, timeout=5)
        response.raise_for_status()
        results = response.json()
        if preferred_set:
            res_filtered = [
                item for item in results
                if (item.get("language") and item["language"].lower() in preferred_set)
                or bool(preferred_set & ({t.lower() for t in (item.get("technologies") or [])} | {s.lower() for s in (item.get("required_skills") or [])} | {g.lower() for g in (item.get("learning_tags") or [])}))
            ]
            if res_filtered:
                results = res_filtered
        return results[:limit]
    except httpx.HTTPError:
        pass

    for item in candidates:
        skills = {skill.lower() for skill in (item.get("required_skills") or [])}
        tags = {tag.lower() for tag in (item.get("learning_tags") or [])}
        techs = {tech.lower() for tech in (item.get("technologies") or [])}
        language_match = bool(item.get("language") and item["language"].lower() in preferred_set)
        tech_match = bool(preferred_set & (techs | skills | tags))
        lang_boost = 15 if language_match else 0
        tech_boost = 8 if tech_match else 0
        skill_match = len(user_skills & skills) * 2
        interest_match = len(interests & tags)
        difficulty_fit = max(0, 5 - abs((item.get("difficulty_score") or 1) - target))
        quality = min(3, (item.get("stars") or 0) // 25000 + 1)
        item["recommendation_score"] = lang_boost + tech_boost + skill_match + interest_match + difficulty_fit + quality
        item["why_recommended"] = []
        if language_match:
            item["why_recommended"].append(f"Uses your preferred language: {item['language']}")
        if tech_match and not language_match:
            item["why_recommended"].append("Matches your preferred technologies")
        if skill_match:
            item["why_recommended"].append("Builds on your current skills")
        if interest_match:
            item["why_recommended"].append("Matches your interests")
        if (item.get("difficulty_score") or 1) >= target:
            item["why_recommended"].append("Slightly stretches your current level")
        if not item["why_recommended"]:
            item["why_recommended"].append("A strong open-source starting point")
    return sorted(candidates, key=lambda item: item["recommendation_score"], reverse=True)[:limit]


@app.post("/contributions")
def start_contribution(payload: ContributionStart, request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    if not issue(payload.issue_id):
        raise HTTPException(404, "Issue not found")
    rows = query("""
        INSERT INTO contributions (user_id, issue_id) VALUES (%s, %s)
        ON CONFLICT (user_id, issue_id) DO UPDATE SET updated_at = now()
        RETURNING id, user_id, issue_id, pr_url, pr_number, state, created_at, updated_at, merged_at
    """, (user_id, payload.issue_id))
    return rows[0]


@app.get("/contributions")
def contributions(request: Request) -> list[dict[str, Any]]:
    user_id = authenticated_user_id(request)
    return query("""
        SELECT c.*, i.title, i.number, i.url AS issue_url, i.required_skills, i.technologies,
               i.updated_at AS issue_updated_at, r.full_name AS repository, r.language
        FROM contributions c
        JOIN issues i ON i.id = c.issue_id JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s ORDER BY c.updated_at DESC
    """, (user_id,))


@app.get("/working")
def working(request: Request) -> list[dict[str, Any]]:
    return query("""
        SELECT c.*, i.title, i.number, i.body, i.url AS issue_url, i.required_skills,
               i.technologies, r.full_name AS repository, r.url AS repository_url, r.language
        FROM contributions c JOIN issues i ON i.id = c.issue_id
        JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s AND c.state = 'STARTED'
        ORDER BY c.updated_at DESC
    """, (authenticated_user_id(request),))


@app.get("/contributions/{contribution_id}")
def contribution(contribution_id: int, request: Request) -> dict[str, Any]:
    rows = query("SELECT * FROM contributions WHERE id = %s AND user_id = %s", (contribution_id, authenticated_user_id(request)))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    return rows[0]


@app.put("/contributions/{contribution_id}")
async def attach_pr(contribution_id: int, payload: ContributionUpdate, request: Request) -> dict[str, Any]:
    if not re.fullmatch(r"https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/\d+", payload.pr_url):
        raise HTTPException(422, "PR URL must be a GitHub pull request URL")
    contribution_rows = query("""
        SELECT c.id, u.username, r.full_name
        FROM contributions c JOIN users u ON u.id = c.user_id
        JOIN issues i ON i.id = c.issue_id JOIN repositories r ON r.id = i.repository_id
        WHERE c.id = %s AND c.user_id = %s
    """, (contribution_id, authenticated_user_id(request)))
    if not contribution_rows:
        raise HTTPException(404, "Contribution not found")
    parsed = urlparse(payload.pr_url)
    pr_parts = [part for part in parsed.path.split("/") if part]
    if len(pr_parts) != 4 or f"{pr_parts[0]}/{pr_parts[1]}" != contribution_rows[0]["full_name"] or int(pr_parts[3]) != payload.pr_number:
        raise HTTPException(422, "PR must belong to the selected issue repository and match its number")
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            verification = await client.post(
                f"{CONTRIBUTION_URL}/validate-pr",
                json={"repository": contribution_rows[0]["full_name"], "pr_number": payload.pr_number, "username": contribution_rows[0]["username"]},
            )
        if verification.status_code != 200 or not verification.json().get("valid"):
            raise HTTPException(422, "GitHub could not verify that this PR belongs to the signed-in user")
    except httpx.HTTPError as error:
        raise HTTPException(503, "Contribution verification service is unavailable") from error
    rows = query("""
        UPDATE contributions SET pr_url = %s, pr_number = %s, state = 'PR_OPEN', updated_at = now()
        WHERE id = %s AND user_id = %s
        RETURNING *
    """, (payload.pr_url, payload.pr_number, contribution_id, authenticated_user_id(request)))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    return rows[0]


BADGE_ICONS = {
    "First Blood": "🩸",
    "Bug Wrangler": "🩹",
    "Polyglot": "🧪",
    "World Traveller": "🗺️",
    "Touch Grass": "🌱"
}


LEVEL_STEP_XP = 500


def calculate_level_info(total_xp: int) -> dict[str, int]:
    total_xp = max(0, int(total_xp or 0))
    level = (total_xp // LEVEL_STEP_XP) + 1
    min_xp = (level - 1) * LEVEL_STEP_XP
    max_xp = level * LEVEL_STEP_XP
    progress_in_level = total_xp - min_xp
    progress_percent = min(100, int((progress_in_level / LEVEL_STEP_XP) * 100))
    return {
        "level": level,
        "total_xp": total_xp,
        "current_level_min_xp": min_xp,
        "next_level_xp": max_xp,
        "progress_in_level": progress_in_level,
        "level_xp_required": LEVEL_STEP_XP,
        "progress_percent": progress_percent
    }


def evaluate_user_badges(user_id: int) -> list[str]:
    stats = query("""
        SELECT 
            count(c.id) AS total_merged,
            count(DISTINCT r.id) AS total_repos,
            count(DISTINCT lower(r.language)) AS total_languages
        FROM contributions c
        JOIN issues i ON i.id = c.issue_id
        JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s AND c.state = 'PR_MERGED'
    """, (user_id,))[0]

    merged_cnt = stats["total_merged"] or 0
    repos_cnt = stats["total_repos"] or 0
    langs_cnt = stats["total_languages"] or 0

    earned = []
    if merged_cnt >= 1:
        earned.append("First Blood")
    if merged_cnt >= 3:
        earned.append("Bug Wrangler")
    if langs_cnt >= 2:
        earned.append("Polyglot")
    if repos_cnt >= 2:
        earned.append("World Traveller")
    if merged_cnt >= 5:
        earned.append("Touch Grass")

    return earned


@app.get("/me")
def me(request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("""
        SELECT u.id, u.username, u.avatar_url, p.experience_level, p.target_level,
               p.progress_score, p.completed_count, p.bio, p.demonstrated_skills,
               COALESCE(p.total_xp, 0) AS total_xp, COALESCE(p.level, 1) AS level, COALESCE(p.badges, '{}') AS badges,
               pref.interests, pref.preferred_languages
        FROM users u JOIN developer_profiles p ON p.user_id = u.id
        JOIN developer_preferences pref ON pref.user_id = u.id
        WHERE u.id = %s
    """, (user_id,))
    if not rows:
        raise HTTPException(401, "Profile not found")
    profile_data = rows[0]
    lvl_info = calculate_level_info(profile_data["total_xp"])
    profile_data.update(lvl_info)
    profile_data["badge_details"] = [
        {"name": b, "icon": BADGE_ICONS.get(b, "🏅")} for b in (profile_data.get("badges") or [])
    ]
    return profile_data


@app.get("/profile")
def profile(request: Request) -> dict[str, Any]:
    return me(request)


def apply_merged_pr(pr_url: str) -> dict[str, Any]:
    parsed = urlparse(pr_url)
    if parsed.netloc != "github.com":
        return {"merged": False}
    parts = [part for part in parsed.path.split("/") if part]
    if len(parts) != 4 or parts[2] != "pull" or not parts[3].isdigit():
        return {"merged": False}

    rows = query("""
        SELECT id, user_id, issue_id, state, COALESCE(xp_awarded, false) AS xp_awarded FROM contributions
        WHERE pr_url = %s
    """, (pr_url,))

    if not rows:
        return {"merged": False}

    contribution_data = rows[0]
    contribution_id = contribution_data["id"]
    user_id = contribution_data["user_id"]
    issue_id = contribution_data["issue_id"]
    already_awarded = contribution_data.get("xp_awarded", False)

    execute("""
        UPDATE contributions SET state = 'PR_MERGED', merged_at = COALESCE(merged_at, now()), updated_at = now()
        WHERE id = %s
    """, (contribution_id,))

    profile_current = query("SELECT COALESCE(total_xp, 0) AS total_xp, COALESCE(level, 1) AS level, COALESCE(badges, '{}') AS badges FROM developer_profiles WHERE user_id = %s", (user_id,))
    if not profile_current:
        execute("INSERT INTO developer_profiles (user_id) VALUES (%s) ON CONFLICT DO NOTHING", (user_id,))
        profile_current = query("SELECT COALESCE(total_xp, 0) AS total_xp, COALESCE(level, 1) AS level, COALESCE(badges, '{}') AS badges FROM developer_profiles WHERE user_id = %s", (user_id,))
    
    current_xp = profile_current[0]["total_xp"]
    current_level = profile_current[0]["level"]
    old_badges = set(profile_current[0].get("badges") or [])

    if already_awarded:
        lvl_info = calculate_level_info(current_xp)
        earned_badges = evaluate_user_badges(user_id)
        return {
            "merged": True,
            "already_awarded": True,
            "earned_xp": 0,
            "total_xp": current_xp,
            "old_level": current_level,
            "level": lvl_info["level"],
            "level_up": False,
            "unlocked_badges": [],
            "all_badges": earned_badges,
            "badges": earned_badges,
            "next_level_xp": lvl_info["next_level_xp"],
            "progress_in_level": lvl_info["progress_in_level"],
            "progress_percent": lvl_info["progress_percent"],
            "message": f"PR is merged! Current Level: {lvl_info['level']} ({current_xp} XP)"
        }

    # Award XP
    issue_rows = query("""
        SELECT i.difficulty, i.required_skills, i.technologies, i.repository_id
        FROM issues i WHERE i.id = %s
    """, (issue_id,))

    issue_data = issue_rows[0] if issue_rows else {}
    difficulty = str(issue_data.get("difficulty") or "BEGINNER").upper()
    base_xp = 150 if difficulty == "BEGINNER" else 350 if difficulty == "INTERMEDIATE" else 600

    bonus_xp = 100  # Merged bonus (+100)
    repo_id = issue_data.get("repository_id")

    prior_repos = query("""
        SELECT count(*) as cnt FROM contributions c
        JOIN issues i ON i.id = c.issue_id
        WHERE c.user_id = %s AND c.state = 'PR_MERGED' AND i.repository_id = %s
    """, (user_id, repo_id))[0]["cnt"]
    if prior_repos <= 1:
        bonus_xp += 50

    skills = issue_data.get("required_skills") or []
    techs = issue_data.get("technologies") or []
    if len(skills) > 1 or len(techs) > 1:
        bonus_xp += 50

    earned_xp = base_xp + bonus_xp

    new_xp = current_xp + earned_xp
    new_lvl_info = calculate_level_info(new_xp)
    new_level = new_lvl_info["level"]

    earned_badges = evaluate_user_badges(user_id)
    unlocked_badges = [b for b in earned_badges if b not in old_badges]

    execute("""
        UPDATE developer_profiles SET 
          completed_count = completed_count + 1,
          progress_score = LEAST(100, progress_score + 12),
          total_xp = %s,
          level = %s,
          badges = %s
        WHERE user_id = %s
    """, (new_xp, new_level, earned_badges, user_id))

    execute("UPDATE contributions SET xp_awarded = true WHERE id = %s", (contribution_id,))

    level_up = new_level > current_level

    return {
        "merged": True,
        "already_awarded": False,
        "earned_xp": earned_xp,
        "total_xp": new_xp,
        "old_level": current_level,
        "level": new_level,
        "level_up": level_up,
        "unlocked_badges": unlocked_badges,
        "all_badges": earned_badges,
        "badges": earned_badges,
        "next_level_xp": new_lvl_info["next_level_xp"],
        "progress_in_level": new_lvl_info["progress_in_level"],
        "progress_percent": new_lvl_info["progress_percent"],
        "message": f"+{earned_xp} XP!" + (" LEVEL UP!" if level_up else "")
    }


PR_CHECK_GRAPHQL_QUERY = """
query CheckPR($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
        pullRequest(number: $number) {
            merged
            state
        }
    }
}
"""


@app.post("/contributions/{contribution_id}/verify")
async def verify_contribution(contribution_id: int, request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("SELECT * FROM contributions WHERE id = %s AND user_id = %s", (contribution_id, user_id))
    if not rows:
        raise HTTPException(404, "Contribution not found")
    contribution_data = rows[0]
    user_rows = query("SELECT github_access_token FROM users WHERE id = %s", (user_id,))
    token = (user_rows[0]["github_access_token"] if user_rows and user_rows[0]["github_access_token"] else None) or GITHUB_TOKEN
    if not token or not contribution_data["pr_url"]:
        return {"verified": False, "message": "Connect GitHub and submit a valid pull request URL to verify status."}
    parsed = urlparse(contribution_data["pr_url"])
    parts = [p for p in parsed.path.split("/") if p]
    if len(parts) != 4 or parts[2] != "pull" or not parts[3].isdigit():
        return {"verified": False, "message": "Invalid pull request URL."}
    owner, repo, pr_number = parts[0], parts[1], int(parts[3])

    pr_info = None
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            response = await client.post(
                "https://api.github.com/graphql",
                headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
                json={"query": PR_CHECK_GRAPHQL_QUERY, "variables": {"owner": owner, "name": repo, "number": pr_number}},
            )
            if response.status_code == 200:
                payload = response.json()
                if not payload.get("errors"):
                    pr_node = (payload.get("data") or {}).get("repository", {}).get("pullRequest")
                    if pr_node:
                        pr_info = {"merged": bool(pr_node.get("merged")), "state": str(pr_node.get("state") or "OPEN").lower()}
    except Exception:
        pass

    if pr_info is None:
        async with httpx.AsyncClient(timeout=10) as client:
            response = await client.get(
                f"https://api.github.com/repos/{owner}/{repo}/pulls/{pr_number}",
                headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}
            )
            if response.status_code != 200:
                raise HTTPException(response.status_code, "GitHub PR lookup failed")
            pr = response.json()
            pr_info = {"merged": bool(pr.get("merged")), "state": str(pr.get("state") or "open").lower()}

    if not pr_info.get("merged"):
        new_state = "PR_OPEN" if pr_info.get("state") == "open" else "PR_CLOSED"
        execute("UPDATE contributions SET state = %s, updated_at = now() WHERE id = %s", (new_state, contribution_id))
        return {"verified": False, "state": pr_info.get("state"), "message": "GitHub confirms the PR is not merged yet."}

    merge_result = apply_merged_pr(contribution_data["pr_url"])
    return {
        "verified": True,
        "state": "PR_MERGED",
        "xp_info": merge_result,
        "message": f"Merged contribution verified! {merge_result.get('message', '')}"
    }


@app.get("/progress")
def progress(request: Request) -> dict[str, Any]:
    user = me(request)
    history = contributions(request)
    return {"progress_score": user["progress_score"], "completed_count": user["completed_count"], "demonstrated_skills": user["demonstrated_skills"], "contributions": history}
