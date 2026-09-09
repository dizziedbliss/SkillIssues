from fastapi import FastAPI
app = FastAPI(title='SkillIssues Recommendation Service')
@app.get('/health')
def health(): return {'status': 'ok', 'service': 'recommendation'}
