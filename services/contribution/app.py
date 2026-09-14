"""SkillIssues Contribution Service.

This service validates GitHub Pull Requests against open-source repositories to verify
developer contributions, author attribution, and merge statuses for XP awards.
"""

import os
from typing import Any
from urllib.parse import urlparse

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", "")

app = FastAPI(
    title="SkillIssues Contribution Service",
    description="Validates GitHub Pull Requests and verifies author attribution and merge status.",
    version="1.0.0",
)


class PullRequestCheck(BaseModel):
    """Payload schema for pull request validation requests."""
    repository: str = Field(..., description="GitHub repository full name, e.g. 'tiangolo/fastapi'")
    pr_number: int = Field(..., description="Pull request number on the upstream repository")
    username: str = Field(..., description="Expected GitHub username of the pull request author")


@app.get("/health")
def health() -> dict[str, str]:
    """Health check endpoint for container orchestrators."""
    return {"status": "ok", "service": "contribution"}


@app.post("/validate-pr")
async def validate_pr(payload: PullRequestCheck) -> dict[str, Any]:
    """Validate a GitHub pull request's existence, author attribution, and merge state.

    Queries the GitHub REST API (`GET /repos/{owner}/{repo}/pulls/{number}`) using
    the configured server GITHUB_TOKEN.

    Checks performed:
        - Verify the PR exists on GitHub.
        - Verify that the PR author login matches payload.username (case-insensitive).
        - Extract the current PR state ('open', 'closed') and boolean 'merged' flag.

    Raises:
        HTTPException 503: If GITHUB_TOKEN is not configured in environment.
        HTTPException 422: If the PR could not be found on GitHub.
    """
    if not GITHUB_TOKEN:
        raise HTTPException(503, "GitHub verification is not configured on the server.")

    async with httpx.AsyncClient(timeout=10) as client:
        response = await client.get(
            f"https://api.github.com/repos/{payload.repository}/pulls/{payload.pr_number}",
            headers={
                "Authorization": f"Bearer {GITHUB_TOKEN}",
                "Accept": "application/vnd.github+json",
            },
        )

    if response.status_code != 200:
        raise HTTPException(422, "GitHub pull request could not be found.")

    pull_request = response.json()
    author = pull_request.get("user", {}).get("login", "")

    # Compare usernames using casefold for internationalized, case-insensitive accuracy
    is_valid_author = author.casefold() == payload.username.casefold()

    return {
        "valid": is_valid_author,
        "author": author,
        "state": pull_request.get("state"),
        "merged": bool(pull_request.get("merged")),
    }
