"""SkillIssues Recommendation Service.

This service calculates personalized recommendation scores for open-source GitHub issues
based on developer profiles (demonstrated skills, interests, preferred languages, and target level).
"""

from typing import Any
from fastapi import FastAPI

app = FastAPI(
    title="SkillIssues Recommendation Service",
    description="Deterministic multi-factor challenge scoring engine matching developer profiles with open-source issues.",
    version="1.0.0",
)


@app.get("/health")
def health() -> dict[str, str]:
    """Health check endpoint for container orchestrators and worker probes."""
    return {"status": "ok", "service": "recommendation"}


@app.post("/score")
def score(payload: dict[str, Any]) -> list[dict[str, Any]]:
    """Score and rank candidate issues against a developer profile.

    Scoring formula:
        Score = (Skill Match * 2) + Interest Match + Difficulty Proximity + Popularity Bonus

    Factors:
        - Skill Match: Overlap between user's demonstrated skills and issue required skills (*2 weighting).
        - Interest Match: Overlap between user's declared interests and issue learning tags.
        - Difficulty Proximity: How closely the issue's difficulty matches the developer's target level:
            BEGINNER: 3, INTERMEDIATE: 5, ADVANCED: 7.
            Proximity = max(0, 5 - abs(difficulty_score - target)).
        - Popularity Bonus: Tiered boost for popular repositories (max +3 points).

    Returns:
        The candidate issues list sorted descending by recommendation_score,
        with human-readable 'why_recommended' explanation tags added to each issue.
    """
    user = payload["user"]
    candidates = payload["issues"]

    # Normalize user attributes for case-insensitive matching
    user_skills = {skill.lower() for skill in (user.get("demonstrated_skills") or [])}
    interests = {item.lower() for item in (user.get("interests") or [])}
    languages = {item.lower() for item in (user.get("preferred_languages") or [])}

    # Map target level strings to baseline numerical difficulty targets
    target = {"BEGINNER": 3, "INTERMEDIATE": 5, "ADVANCED": 7}.get(
        user.get("target_level"), 5
    )

    for item in candidates:
        skills = {skill.lower() for skill in item.get("required_skills", [])}
        tags = {tag.lower() for tag in item.get("learning_tags", [])}

        # 1. Skill overlap (weighted x2)
        skill_match = len(user_skills & skills) * 2

        # 2. Interest overlap
        interest_match = len(interests & tags)

        # 3. Preferred language match
        language_match = bool(
            item.get("language") and item["language"].lower() in languages
        )

        # 4. Difficulty proximity (closer to target = higher score, max 5)
        difficulty_proximity = max(0, 5 - abs(item.get("difficulty_score", 5) - target))

        # 5. Repository popularity boost based on stars
        stars = item.get("stars") or 0
        popularity_bonus = min(3, stars // 25000 + 1)

        # Final composite score
        item["recommendation_score"] = (
            skill_match + interest_match + difficulty_proximity + popularity_bonus
        )

        # Construct user-facing explanation badges
        item["why_recommended"] = []
        if skill_match:
            item["why_recommended"].append("Builds on your current skills")
        if interest_match:
            item["why_recommended"].append("Matches your interests")
        if language_match:
            item["why_recommended"].append(
                f"Uses your preferred language: {item['language']}"
            )
        if item.get("difficulty_score", 5) >= target:
            item["why_recommended"].append("Slightly stretches your current level")
        if not item["why_recommended"]:
            item["why_recommended"].append("A strong open-source starting point")

    # Return candidates ranked highest recommendation score first
    return sorted(
        candidates, key=lambda item: item["recommendation_score"], reverse=True
    )
