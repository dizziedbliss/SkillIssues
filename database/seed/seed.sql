INSERT INTO users (github_id, username, avatar_url) VALUES ('demo-1', 'alex-dev', 'https://github.com/identicons/alex-dev.png') ON CONFLICT (github_id) DO NOTHING;
INSERT INTO developer_profiles (user_id, bio, experience_level, target_level, progress_score, demonstrated_skills)
SELECT id, 'Backend learner building production instincts.', 'BEGINNER', 'INTERMEDIATE', 28, ARRAY['Python'] FROM users WHERE github_id = 'demo-1'
ON CONFLICT (user_id) DO NOTHING;
INSERT INTO developer_preferences (user_id, interests, preferred_languages)
SELECT id, ARRAY['APIs', 'Developer Experience'], ARRAY['Python', 'TypeScript'] FROM users WHERE github_id = 'demo-1'
ON CONFLICT (user_id) DO NOTHING;
INSERT INTO repositories (github_id, full_name, url, description, language, stars, license) VALUES
(1001, 'fastapi/fastapi', 'https://github.com/fastapi/fastapi', 'Modern, fast web framework for building APIs with Python.', 'Python', 80000, 'MIT'),
(1002, 'encode/httpx', 'https://github.com/encode/httpx', 'A next generation HTTP client for Python.', 'Python', 13000, 'BSD-3-Clause'),
(1003, 'vitejs/vite', 'https://github.com/vitejs/vite', 'Next generation frontend tooling.', 'TypeScript', 70000, 'MIT') ON CONFLICT (github_id) DO NOTHING;
INSERT INTO issues (github_id, repository_id, number, title, body, url, difficulty_score, difficulty, required_skills, technologies, learning_tags, comments)
SELECT 2001, id, 11900, 'Improve authentication error handling', 'Make authentication failures return consistent, actionable errors for API consumers.', 'https://github.com/fastapi/fastapi/issues/11900', 4, 'INTERMEDIATE', ARRAY['Python', 'REST APIs'], ARRAY['FastAPI', 'Pydantic'], ARRAY['error handling', 'API design'], 8 FROM repositories WHERE github_id = 1001
ON CONFLICT (github_id) DO NOTHING;
INSERT INTO issues (github_id, repository_id, number, title, body, url, difficulty_score, difficulty, required_skills, technologies, learning_tags, comments)
SELECT 2002, id, 3360, 'Add clearer timeout guidance to client docs', 'Document timeout behavior with a small runnable example and troubleshooting notes.', 'https://github.com/encode/httpx/issues/3360', 3, 'BEGINNER', ARRAY['Python', 'Technical Writing'], ARRAY['HTTPX'], ARRAY['documentation', 'HTTP'], 3 FROM repositories WHERE github_id = 1002
ON CONFLICT (github_id) DO NOTHING;
INSERT INTO issues (github_id, repository_id, number, title, body, url, difficulty_score, difficulty, required_skills, technologies, learning_tags, comments)
SELECT 2003, id, 18000, 'Improve plugin error overlay feedback', 'Make the development overlay more useful when a plugin fails during startup.', 'https://github.com/vitejs/vite/issues/18000', 6, 'ADVANCED', ARRAY['TypeScript', 'Frontend'], ARRAY['Vite', 'React'], ARRAY['tooling', 'debugging'], 14 FROM repositories WHERE github_id = 1003
ON CONFLICT (github_id) DO NOTHING;
