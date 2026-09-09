from typing import Any
from fastapi import FastAPI
app = FastAPI(title='SkillIssues Recommendation Service')
@app.get('/health')
def health(): return {'status': 'ok', 'service': 'recommendation'}

@app.post('/score')
def score(payload: dict[str, Any]) -> list[dict[str, Any]]:
	user = payload['user']
	candidates = payload['issues']
	user_skills = {skill.lower() for skill in (user.get('demonstrated_skills') or [])}
	interests = {item.lower() for item in (user.get('interests') or [])}
	languages = {item.lower() for item in (user.get('preferred_languages') or [])}
	target = {'BEGINNER': 3, 'INTERMEDIATE': 5, 'ADVANCED': 7}.get(user.get('target_level'), 5)
	for item in candidates:
		skills = {skill.lower() for skill in item['required_skills']}
		tags = {tag.lower() for tag in item['learning_tags']}
		skill_match = len(user_skills & skills) * 2
		interest_match = len(interests & tags)
		language_match = bool(item.get('language') and item['language'].lower() in languages)
		item['recommendation_score'] = skill_match + interest_match + max(0, 5 - abs(item['difficulty_score'] - target)) + min(3, item['stars'] // 25000 + 1)
		item['why_recommended'] = []
		if skill_match: item['why_recommended'].append('Builds on your current skills')
		if interest_match: item['why_recommended'].append('Matches your interests')
		if language_match: item['why_recommended'].append(f"Uses your preferred language: {item['language']}")
		if item['difficulty_score'] >= target: item['why_recommended'].append('Slightly stretches your current level')
		if not item['why_recommended']: item['why_recommended'].append('A strong open-source starting point')
	return sorted(candidates, key=lambda item: item['recommendation_score'], reverse=True)
