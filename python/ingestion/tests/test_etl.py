"""
Unit tests for Python Ingestion ETL Pipeline
"""

import unittest
import pandas as pd
from python.ingestion.etl import filter_dataframe


class TestIngestionETL(unittest.TestCase):
    def test_filter_dataframe_empty(self):
        df = pd.DataFrame()
        res = filter_dataframe(df)
        self.assertTrue(res.empty)
        self.assertEqual(
            list(res.columns),
            ["full_name", "name", "owner", "stars", "forks", "language", "description", "license"],
        )

    def test_filter_dataframe_basic(self):
        data = [
            {
                "nameWithOwner": "user/repo1",
                "stargazersCount": 100,
                "forksCount": 10,
                "primaryLanguage": "Go",
                "isArchived": 0,
                "isFork": 0,
                "forkingAllowed": 1,
                "license": "mit",
                "description": "A good repo",
            },
            {
                "nameWithOwner": "user/archived-repo",
                "stargazersCount": 500,
                "forksCount": 5,
                "primaryLanguage": "Python",
                "isArchived": 1,
                "isFork": 0,
                "forkingAllowed": 1,
                "license": "mit",
                "description": "Archived repo",
            },
            {
                "nameWithOwner": "user/forked-repo",
                "stargazersCount": 300,
                "forksCount": 2,
                "primaryLanguage": "Rust",
                "isArchived": 0,
                "isFork": 1,
                "forkingAllowed": 1,
                "license": "apache-2.0",
                "description": "Forked repo",
            },
            {
                "nameWithOwner": "user/low-stars",
                "stargazersCount": 15,
                "forksCount": 1,
                "primaryLanguage": "TypeScript",
                "isArchived": 0,
                "isFork": 0,
                "forkingAllowed": 1,
                "license": "mit",
                "description": "Low stars repo",
            },
            {
                "nameWithOwner": "user/no-license",
                "stargazersCount": 120,
                "forksCount": 10,
                "primaryLanguage": "C++",
                "isArchived": 0,
                "isFork": 0,
                "forkingAllowed": 1,
                "license": "None",
                "description": "Missing license repo",
            },
        ]
        df = pd.DataFrame(data)
        res = filter_dataframe(df, min_stars=50)

        # Only repo1 qualifies: archived, fork, low stars, and no-license should all be filtered out
        self.assertEqual(len(res), 1)
        self.assertEqual(res.iloc[0]["full_name"], "user/repo1")
        self.assertEqual(res.iloc[0]["owner"], "user")
        self.assertEqual(res.iloc[0]["name"], "repo1")
        self.assertEqual(res.iloc[0]["stars"], 100)
        self.assertEqual(res.iloc[0]["language"], "Go")

    def test_filter_dataframe_deduplication(self):
        data = [
            {
                "full_name": "popular/project",
                "stars": 200,
                "forks": 20,
                "isArchived": 0,
                "isFork": 0,
                "forkingAllowed": 1,
                "license": "mit",
            },
            {
                "full_name": "popular/project",
                "stars": 400,
                "forks": 50,
                "isArchived": 0,
                "isFork": 0,
                "forkingAllowed": 1,
                "license": "mit",
            },
        ]
        df = pd.DataFrame(data)
        res = filter_dataframe(df, min_stars=50)

        self.assertEqual(len(res), 1)
        # Highest stars should be preserved
        self.assertEqual(res.iloc[0]["stars"], 400)

    def test_filter_dataframe_max_repositories(self):
        data = [
            {"full_name": f"org/repo{i}", "stars": 100 + i, "isArchived": 0, "isFork": 0, "license": "mit"}
            for i in range(10)
        ]
        df = pd.DataFrame(data)
        res = filter_dataframe(df, min_stars=50, max_repositories=3)

        self.assertEqual(len(res), 3)
        # Sorted by stars descending
        self.assertEqual(res.iloc[0]["stars"], 109)
        self.assertEqual(res.iloc[1]["stars"], 108)
        self.assertEqual(res.iloc[2]["stars"], 107)


if __name__ == "__main__":
    unittest.main()
