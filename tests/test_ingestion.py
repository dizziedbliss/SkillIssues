import importlib.util
import json
import os
import sys
import tempfile
import types
from pathlib import Path
import unittest

os.environ.setdefault("DATABASE_URL", "postgresql://unused")
fake_psycopg = types.ModuleType("psycopg")
fake_psycopg.connect = lambda *args, **kwargs: None
sys.modules.setdefault("psycopg", fake_psycopg)
spec = importlib.util.spec_from_file_location("ingestion", Path(__file__).parents[1] / "services" / "ingestion" / "app.py")
ingestion = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ingestion)


class IngestionTests(unittest.TestCase):
    def test_qualification_requires_safe_repository_flags(self):
        record = {
            "owner": "demo", "name": "project", "stars": 100,
            "isFork": False, "isArchived": False, "forkingAllowed": True,
            "license": "MIT", "primaryLanguage": "Python", "pushedAt": "2026-01-01T00:00:00Z",
        }
        cleaned = ingestion.clean_repository(record)
        self.assertEqual(cleaned["full_name"], "demo/project")
        self.assertEqual(cleaned["language"], "Python")
        self.assertIsNotNone(cleaned["pushed_at"])

    def test_stream_records_does_not_require_full_file_load(self):
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8") as handle:
            json.dump([{"owner": "a", "name": "b"}, {"owner": "c", "name": "d"}], handle)
            path = Path(handle.name)
        try:
            self.assertEqual(list(ingestion.stream_records(path)), [{"owner": "a", "name": "b"}, {"owner": "c", "name": "d"}])
        finally:
            path.unlink()


if __name__ == "__main__":
    unittest.main()
