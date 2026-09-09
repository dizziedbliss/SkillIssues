import os
from urllib.parse import urlparse

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

GITHUB_TOKEN = os.getenv('GITHUB_TOKEN', '')
app = FastAPI(title='SkillIssues Contribution Service')


class PullRequestCheck(BaseModel):
	repository: str
	pr_number: int
	username: str


@app.get('/health')
def health():
	return {'status': 'ok', 'service': 'contribution'}


@app.post('/validate-pr')
async def validate_pr(payload: PullRequestCheck):
	if not GITHUB_TOKEN:
		raise HTTPException(503, 'GitHub verification is not configured')
	async with httpx.AsyncClient(timeout=10) as client:
		response = await client.get(
			f'https://api.github.com/repos/{payload.repository}/pulls/{payload.pr_number}',
			headers={'Authorization': f'Bearer {GITHUB_TOKEN}', 'Accept': 'application/vnd.github+json'},
		)
	if response.status_code != 200:
		raise HTTPException(422, 'GitHub pull request could not be found')
	pull_request = response.json()
	author = pull_request.get('user', {}).get('login', '')
	return {'valid': author.casefold() == payload.username.casefold(), 'author': author, 'state': pull_request.get('state'), 'merged': bool(pull_request.get('merged'))}
