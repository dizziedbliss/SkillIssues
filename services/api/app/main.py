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
            cursor.execute(seed)


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


LEVEL_THRESHOLDS = [
    (1, "Code Novice", 0, 250),
    (2, "PR Apprentice", 250, 550),
    (3, "Bug Hunter", 550, 950),
    (4, "Feature Crafter", 950, 1450),
    (5, "System Architect", 1450, 2050),
    (6, "Open Source Veteran", 2050, 2800),
    (7, "Code Master", 2800, 3700),
    (8, "Repo Legend", 3700, 4800),
]

BADGE_DEFINITIONS = {
    "First Blood": {"icon": "🩸", "name": "First Blood", "desc": "Completed 1st merged PR"},
    "Bug Wrangler": {"icon": "🤠", "name": "Bug Wrangler", "desc": "Completed 3+ merged PRs"},
    "Polyglot": {"icon": "🌐", "name": "Polyglot", "desc": "Contributed in 2+ languages"},
    "World Traveller": {"icon": "🌍", "name": "World Traveller", "desc": "Contributed across 3+ repos"},
    "Touch Grass": {"icon": "🌲", "name": "Touch Grass", "desc": "Reached Level 5 or higher"}
}


def calculate_level_info(total_xp: int) -> dict[str, Any]:
    for level, title, base, req in LEVEL_THRESHOLDS:
        if total_xp < req:
            return {
                "level": level,
                "title": title,
                "current_xp": total_xp,
                "base_xp": base,
                "next_xp": req,
                "xp_in_level": total_xp - base,
                "level_xp_needed": req - base,
                "progress_percent": min(100, int(((total_xp - base) / max(1, req - base)) * 100))
            }
    lvl = int((total_xp / 100) ** 0.5) + 1
    base = (lvl - 1) * (lvl - 1) * 100
    req = lvl * lvl * 100
    return {
        "level": lvl,
        "title": "Grandmaster",
        "current_xp": total_xp,
        "base_xp": base,
        "next_xp": req,
        "xp_in_level": total_xp - base,
        "level_xp_needed": req - base,
        "progress_percent": min(100, int(((total_xp - base) / max(1, req - base)) * 100))
    }


def evaluate_badges(user_id: int, current_level: int) -> list[str]:
    stats = query("""
        SELECT count(c.id) AS total_merged,
               count(DISTINCT r.language) AS languages_count,
               count(DISTINCT r.id) AS repos_count
        FROM contributions c
        JOIN issues i ON i.id = c.issue_id
        JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s AND c.state = 'PR_MERGED'
    """, (user_id,))
    total_merged = stats[0]["total_merged"] if stats else 0
    languages_count = stats[0]["languages_count"] if stats else 0
    repos_count = stats[0]["repos_count"] if stats else 0
    badges = []
    if total_merged >= 1:
        badges.append("First Blood")
    if total_merged >= 3:
        badges.append("Bug Wrangler")
    if languages_count >= 2:
        badges.append("Polyglot")
    if repos_count >= 3:
        badges.append("World Traveller")
    if current_level >= 5:
        badges.append("Touch Grass")
    return badges


@app.get("/me")
def me(request: Request) -> dict[str, Any]:
    user_id = authenticated_user_id(request)
    rows = query("""
        SELECT u.id, u.username, u.avatar_url, p.experience_level, p.target_level,
               p.progress_score, p.completed_count, p.bio, p.demonstrated_skills,
               p.total_xp, p.level, p.streak_days, p.badges,
               pref.interests, pref.preferred_languages
        FROM users u JOIN developer_profiles p ON p.user_id = u.id
        JOIN developer_preferences pref ON pref.user_id = u.id
        WHERE u.id = %s
    """, (user_id,))
    if not rows:
        raise HTTPException(401, "Profile not found")
    user = rows[0]
    total_xp = user.get("total_xp") or 0
    level_info = calculate_level_info(total_xp)

    skill_bars = []
    langs = user.get("preferred_languages") or ["Python", "Git", "React"]
    if "Python" not in langs:
        langs.append("Python")
    if "Git" not in langs:
        langs.append("Git")
    if "React" not in langs:
        langs.append("React")

    for i, lang in enumerate(langs[:3]):
        score = max(4, min(10, 8 - i + (user.get("completed_count", 0) * 2)))
        skill_bars.append({
            "name": lang,
            "score": score,
            "bar": ("█" * score) + ("░" * (10 - score))
        })

    user_badges = user.get("badges") or []
    if user.get("completed_count", 0) >= 1 and "First Blood" not in user_badges:
        user_badges.append("First Blood")

    badge_objects = []
    for b_name, b_info in BADGE_DEFINITIONS.items():
        is_unlocked = b_name in user_badges
        badge_objects.append({
            "name": b_name,
            "icon": b_info["icon"],
            "desc": b_info["desc"],
            "unlocked": is_unlocked
        })

    user["level"] = level_info["level"]
    user["level_title"] = level_info["title"]
    user["total_xp"] = total_xp
    user["next_xp"] = level_info["next_xp"]
    user["base_xp"] = level_info["base_xp"]
    user["progress_percent"] = level_info["progress_percent"]
    user["streak_days"] = user.get("streak_days") or 4
    user["skills_bars"] = skill_bars
    user["badge_objects"] = badge_objects
    return user


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


last_github_sync_time: dict[str, float] = {}


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

    headers = {"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}
    synced_count = 0

    try:
        async with httpx.AsyncClient(timeout=10) as client:
            repos_res = await client.get(f"https://api.github.com/users/{username}/repos?sort=updated&per_page=15", headers=headers)
            if repos_res.status_code != 200:
                return 0
            user_repos = repos_res.json()

            for repo_data in user_repos:
                full_name = repo_data.get("full_name")
                if not full_name:
                    continue
                repo_rows = query("""
                    INSERT INTO repositories (github_id, full_name, url, description, language, stars, license)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (full_name) DO UPDATE SET
                      description = EXCLUDED.description, language = EXCLUDED.language, stars = EXCLUDED.stars
                    RETURNING id
                """, (
                    repo_data.get("id"), full_name, repo_data.get("html_url"),
                    repo_data.get("description") or "", repo_data.get("language") or "Python",
                    repo_data.get("stargazers_count", 0), repo_data.get("license", {}).get("spdx_id") if isinstance(repo_data.get("license"), dict) else "MIT"
                ))
                repo_id = repo_rows[0]["id"]

                issues_res = await client.get(f"https://api.github.com/repos/{full_name}/issues?state=open&per_page=30", headers=headers)
                if issues_res.status_code != 200:
                    continue
                raw_issues = issues_res.json()

                for gh_issue in raw_issues:
                    if "pull_request" in gh_issue:
                        continue
                    number = gh_issue.get("number")
                    title = gh_issue.get("title")
                    body = gh_issue.get("body") or ""
                    issue_url = gh_issue.get("html_url")
                    comments = gh_issue.get("comments", 0)
                    labels = [l.get("name", "") for l in gh_issue.get("labels", []) if isinstance(l, dict)]
                    label_text = " ".join(labels).lower()
                    
                    score = 4
                    if any(w in label_text for w in ("good first issue", "easy", "beginner")):
                        score -= 2
                    if any(w in label_text for w in ("advanced", "complex", "hard")):
                        score += 2
                    score = max(1, min(10, score))
                    difficulty = "BEGINNER" if score <= 3 else "INTERMEDIATE" if score <= 6 else "ADVANCED"
                    
                    lang = repo_data.get("language") or "Python"
                    skills = [lang]
                    technologies = [lang]
                    tags = labels[:4] if labels else ["open-source", "demo"]

                    execute("""
                        INSERT INTO issues
                          (github_id, repository_id, number, title, body, url, state, difficulty_score,
                           difficulty, required_skills, technologies, learning_tags, comments, updated_at)
                        VALUES (%s, %s, %s, %s, %s, %s, 'OPEN', %s, %s, %s, %s, %s, %s, now())
                        ON CONFLICT (github_id) DO UPDATE SET
                          title = EXCLUDED.title, body = EXCLUDED.body, url = EXCLUDED.url,
                          difficulty_score = EXCLUDED.difficulty_score, difficulty = EXCLUDED.difficulty,
                          comments = EXCLUDED.comments, updated_at = now()
                    """, (
                        gh_issue.get("id"), repo_id, number, title, body, issue_url,
                        score, difficulty, skills, technologies, tags, comments
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
    where = "WHERE i.state = 'OPEN'"
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
    candidates = issue_query("WHERE i.state = 'OPEN'", ())
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
async def contributions(request: Request) -> list[dict[str, Any]]:
    user_id = authenticated_user_id(request)
    rows = query("SELECT github_access_token FROM users WHERE id = %s", (user_id,))
    token = (rows[0]["github_access_token"] if rows and rows[0]["github_access_token"] else None) or GITHUB_TOKEN

    open_prs = query("""
        SELECT c.id, c.pr_url, c.pr_number, r.full_name
        FROM contributions c
        JOIN issues i ON i.id = c.issue_id
        JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s AND c.state = 'PR_OPEN' AND c.pr_url IS NOT NULL
    """, (user_id,))

    if open_prs and token:
        headers = {"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}
        async with httpx.AsyncClient(timeout=8) as client:
            for item in open_prs:
                try:
                    parsed = urlparse(item["pr_url"])
                    parts = [p for p in parsed.path.split("/") if p]
                    if len(parts) == 4 and parts[2] == "pull":
                        owner, repo, _, number = parts
                        res = await client.get(f"https://api.github.com/repos/{owner}/{repo}/pulls/{number}", headers=headers)
                        if res.status_code == 200:
                            pr_data = res.json()
                            if pr_data.get("merged"):
                                apply_merged_pr(item["pr_url"])
                            elif pr_data.get("state") == "closed":
                                execute("UPDATE contributions SET state = 'PR_CLOSED', updated_at = now() WHERE id = %s", (item["id"],))
                except Exception:
                    pass

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


def apply_merged_pr(pr_url: str) -> dict[str, Any]:
    parsed = urlparse(pr_url)
    if parsed.netloc != "github.com":
        return {"merged": False}
    parts = [part for part in parsed.path.split("/") if part]
    if len(parts) != 4 or parts[2] != "pull" or not parts[3].isdigit():
        return {"merged": False}
    rows = query("""
        UPDATE contributions SET state = 'PR_MERGED', merged_at = now(), updated_at = now()
        WHERE pr_url = %s AND state <> 'PR_MERGED'
        RETURNING id, user_id, issue_id
    """, (pr_url,))
    if not rows:
        return {"merged": False}

    summary: dict[str, Any] = {"merged": True}
    for contribution_data in rows:
        user_id = contribution_data["user_id"]
        issue_id = contribution_data["issue_id"]
        issue_rows = query("SELECT difficulty_score, required_skills FROM issues WHERE id = %s", (issue_id,))
        diff_score = issue_rows[0]["difficulty_score"] if issue_rows else 3
        req_skills = issue_rows[0]["required_skills"] if issue_rows else []

        xp_gained = 100 + (diff_score * 50)
        profile_rows = query("SELECT total_xp, level, badges FROM developer_profiles WHERE user_id = %s", (user_id,))
        old_xp = profile_rows[0]["total_xp"] if profile_rows and profile_rows[0]["total_xp"] else 0
        old_level = profile_rows[0]["level"] if profile_rows and profile_rows[0]["level"] else 1
        new_xp = old_xp + xp_gained
        level_info = calculate_level_info(new_xp)
        new_level = level_info["level"]
        new_badges = evaluate_badges(user_id, new_level)
        leveled_up = new_level > old_level

        execute("""
            UPDATE developer_profiles
            SET total_xp = %s,
                level = %s,
                completed_count = completed_count + 1,
                progress_score = LEAST(100, progress_score + 12),
                badges = %s,
                demonstrated_skills = ARRAY(SELECT DISTINCT unnest(demonstrated_skills || %s))
            WHERE user_id = %s
        """, (new_xp, new_level, new_badges, req_skills, user_id))

        summary = {
            "merged": True,
            "xp_gained": xp_gained,
            "total_xp": new_xp,
            "level": new_level,
            "level_title": level_info["title"],
            "leveled_up": leveled_up,
            "badges": new_badges,
            "unlocked_message": "You've unlocked harder Backend issues." if leveled_up else ""
        }
    return summary


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
    if len(parts) != 4 or parts[2] != "pull":
        return {"verified": False, "message": "Invalid pull request URL."}
    owner_repo = f"{parts[0]}/{parts[1]}"
    pr_number = parts[3]
    async with httpx.AsyncClient() as client:
        response = await client.get(f"https://api.github.com/repos/{owner_repo}/pulls/{pr_number}", headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"})
    if response.status_code != 200:
        raise HTTPException(response.status_code, "GitHub PR lookup failed")
    pr = response.json()
    if not pr.get("merged"):
        execute("UPDATE contributions SET state = %s, updated_at = now() WHERE id = %s", ("PR_OPEN" if pr.get("state") == "open" else "PR_CLOSED", contribution_id))
        return {"verified": False, "state": pr.get("state"), "message": "GitHub confirms the PR is not merged yet."}
    reward = apply_merged_pr(contribution_data["pr_url"])
    xp = reward.get("xp_gained", 250)
    level_msg = f" LEVEL UP to Level {reward.get('level')} ({reward.get('level_title')})!" if reward.get("leveled_up") else ""
    return {"verified": True, "state": "PR_MERGED", "message": f"+{xp} XP! Merged contribution verified by GitHub!{level_msg}", "reward": reward}


@app.get("/progress")
def progress(request: Request) -> dict[str, Any]:
    user = me(request)
    history = contributions(request)
    return {"progress_score": user["progress_score"], "completed_count": user["completed_count"], "demonstrated_skills": user["demonstrated_skills"], "contributions": history}
