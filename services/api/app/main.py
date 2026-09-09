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


@app.get("/auth/github")
@app.post("/auth/github")
def github_login(response: Response) -> dict[str, Any]:
    client_id = os.getenv("GITHUB_CLIENT_ID", "")
    if not client_id:
        return {"enabled": False, "message": "Demo mode is active. Configure GITHUB_CLIENT_ID for OAuth."}
    state = secrets.token_urlsafe(32)
    verifier = secrets.token_urlsafe(64)
    challenge = base64url(hashlib.sha256(verifier.encode("ascii")).digest())
    execute("INSERT INTO oauth_states (state, code_verifier, redirect_uri) VALUES (%s, %s, %s)", (state, verifier, GITHUB_REDIRECT_URI))
    response.set_cookie("skillissues_oauth_state", state, httponly=True, samesite="lax", max_age=600)
    params = urlencode({"client_id": client_id, "redirect_uri": GITHUB_REDIRECT_URI, "scope": "read:user user:email public_repo", "state": state, "code_challenge": challenge, "code_challenge_method": "S256"})
    return {
        "enabled": True,
        "authorize_url": f"https://github.com/login/oauth/authorize?{params}",
        "redirect_uri": GITHUB_REDIRECT_URI,
    }


@app.get("/auth/github/callback")
async def github_callback(request: Request, code: str | None = None, state: str | None = None, error: str | None = None) -> Response:
    if error:
        raise HTTPException(400, f"GitHub authorization failed: {error}")
    oauth_state = request.cookies.get("skillissues_oauth_state", "")
    if not code or not state or not hmac.compare_digest(state, oauth_state):
        raise HTTPException(400, "Invalid or expired OAuth state")
    state_rows = query("DELETE FROM oauth_states WHERE state = %s AND expires_at > now() RETURNING code_verifier, redirect_uri", (state,))
    if not state_rows or not os.getenv("GITHUB_CLIENT_ID") or not os.getenv("GITHUB_CLIENT_SECRET"):
        raise HTTPException(400, "GitHub OAuth is not configured or no code was provided")
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
        ON CONFLICT (github_id) DO UPDATE SET username = EXCLUDED.username, avatar_url = EXCLUDED.avatar_url
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
    if not rows or not rows[0]["github_access_token"]:
        raise HTTPException(403, "Connect GitHub with public repository access before starting a contribution workspace.")
    owner, separator, name = payload.repository.partition("/")
    if not separator:
        raise HTTPException(422, "Invalid repository name")
    headers = {"Authorization": f"Bearer {rows[0]['github_access_token']}", "Accept": "application/vnd.github+json"}
    async with httpx.AsyncClient(timeout=20) as client:
        existing = await client.get(f"https://api.github.com/repos/{rows[0]['username']}/{name}", headers=headers)
        if existing.status_code == 200:
            fork = existing.json()
        else:
            response = await client.post(f"https://api.github.com/repos/{owner}/{name}/forks", headers=headers, json={"default_branch_only": True})
            if response.status_code not in (201, 202):
                raise HTTPException(response.status_code, "GitHub could not create a fork. Check the GitHub App public_repo permission.")
            fork = response.json()
    return {"full_name": fork["full_name"], "clone_url": fork["clone_url"], "html_url": fork["html_url"]}


@app.get("/github/git-token")
def github_git_token(request: Request) -> dict[str, str]:
    rows = query("SELECT github_access_token FROM users WHERE id = %s", (authenticated_user_id(request),))
    if not rows or not rows[0]["github_access_token"]:
        raise HTTPException(403, "Connect GitHub before pushing changes.")
    return {"token": rows[0]["github_access_token"]}


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
    if not rows or not rows[0]["github_access_token"]:
        raise HTTPException(403, "Connect GitHub before submitting a pull request.")
    contribution = rows[0]
    owner, _, name = contribution["full_name"].partition("/")
    headers = {"Authorization": f"Bearer {contribution['github_access_token']}", "Accept": "application/vnd.github+json"}
    async with httpx.AsyncClient(timeout=20) as client:
        repository_response = await client.get(f"https://api.github.com/repos/{owner}/{name}", headers=headers)
        if repository_response.status_code != 200:
            raise HTTPException(repository_response.status_code, "GitHub repository details could not be loaded.")
        base = repository_response.json().get("default_branch", "main")
        response = await client.post(f"https://api.github.com/repos/{owner}/{name}/pulls", headers=headers, json={
            "title": contribution["title"],
            "head": f"{contribution['username']}:{payload.branch}",
            "base": base,
            "body": contribution["body"] or f"SkillIssues contribution for issue #{contribution['number']}",
        })
    if response.status_code not in (201,):
        raise HTTPException(response.status_code, f"GitHub could not create the pull request: {response.text[:300]}")
    pull_request = response.json()
    updated = query("UPDATE contributions SET pr_url = %s, pr_number = %s, state = 'PR_OPEN', updated_at = now() WHERE id = %s RETURNING *", (pull_request["html_url"], pull_request["number"], contribution_id))[0]
    return {"contribution": updated, "pull_request": pull_request}


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


@app.get("/issues")
def issues(difficulty: str | None = Query(default=None), language: str | None = Query(default=None), search: str | None = Query(default=None), limit: int = Query(default=20, le=50)) -> list[dict[str, Any]]:
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
def recommendations(request: Request, limit: int = Query(default=3, le=50)) -> list[dict[str, Any]]:
    user = me(request)
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
    return query("""
        SELECT c.*, i.title, i.number, i.url AS issue_url, i.required_skills, i.technologies,
               i.updated_at AS issue_updated_at, r.full_name AS repository, r.language
        FROM contributions c
        JOIN issues i ON i.id = c.issue_id JOIN repositories r ON r.id = i.repository_id
        WHERE c.user_id = %s ORDER BY c.updated_at DESC
    """, (authenticated_user_id(request),))


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


def apply_merged_pr(pr_url: str) -> bool:
    parsed = urlparse(pr_url)
    if parsed.netloc != "github.com":
        return False
    parts = [part for part in parsed.path.split("/") if part]
    if len(parts) != 4 or parts[2] != "pull" or not parts[3].isdigit():
        return False
    rows = query("""
        UPDATE contributions SET state = 'PR_MERGED', merged_at = now(), updated_at = now()
        WHERE pr_url = %s AND state <> 'PR_MERGED'
        RETURNING id, user_id, issue_id
    """, (pr_url,))
    for contribution_data in rows:
        execute("""
            UPDATE developer_profiles SET completed_count = completed_count + 1,
              progress_score = LEAST(100, progress_score + 12),
              demonstrated_skills = ARRAY(SELECT DISTINCT unnest(demonstrated_skills || i.required_skills))
            FROM issues i WHERE i.id = %s AND user_id = %s
        """, (contribution_data["issue_id"], contribution_data["user_id"]))
    return bool(rows)


@app.post("/contributions/{contribution_id}/verify")
async def verify_contribution(contribution_id: int, request: Request) -> dict[str, Any]:
    rows = query("SELECT * FROM contributions WHERE id = %s AND user_id = %s", (contribution_id, authenticated_user_id(request)))
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
    apply_merged_pr(contribution_data["pr_url"])
    return {"verified": True, "state": "PR_MERGED", "message": "Merged contribution verified by GitHub."}


@app.get("/progress")
def progress(request: Request) -> dict[str, Any]:
    user = me(request)
    history = contributions(request)
    return {"progress_score": user["progress_score"], "completed_count": user["completed_count"], "demonstrated_skills": user["demonstrated_skills"], "contributions": history}
