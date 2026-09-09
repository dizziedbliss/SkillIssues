from fastapi import FastAPI
app = FastAPI(title='SkillIssues Contribution Service')
@app.get('/health')
def health(): return {'status': 'ok', 'service': 'contribution'}
