"""
SkillIssues Kaggle ETL Pipeline
Handles dataset download, vectorized pandas filtering, normalization, and bulk upsert to PostgreSQL.
"""

from __future__ import annotations

import logging
import os
from typing import Any, Dict, List, Optional
import pandas as pd

logger = logging.getLogger("skillissues.ingestion")
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")


def filter_dataframe(
    df: pd.DataFrame,
    min_stars: int = 50,
    max_repositories: int = 0,
) -> pd.DataFrame:
    """
    Vectorized filter on raw GitHub repository metadata DataFrame.
    Filters out archived repos, forks, repos without permissive licenses,
    deduplicates on repository full_name, and sorts by stars descending.
    """
    if df.empty:
        return pd.DataFrame(
            columns=["full_name", "name", "owner", "stars", "forks", "language", "description", "license"]
        )

    # Normalize column names: rename common Kaggle column variants
    col_map = {
        "nameWithOwner": "full_name",
        "fullName": "full_name",
        "stargazersCount": "stars",
        "forksCount": "forks",
        "primaryLanguage": "language",
    }
    df = df.rename(columns={k: v for k, v in col_map.items() if k in df.columns})

    # Ensure required columns exist
    if "full_name" not in df.columns and "name" in df.columns and "owner" in df.columns:
        df["full_name"] = df["owner"].astype(str) + "/" + df["name"].astype(str)

    if "full_name" not in df.columns:
        raise ValueError("DataFrame must contain 'full_name' or ('owner', 'name') columns")

    # Coerce stars and forks
    if "stars" in df.columns:
        df["stars"] = pd.to_numeric(df["stars"], errors="coerce").fillna(0).astype(int)
    else:
        df["stars"] = 0

    if "forks" in df.columns:
        df["forks"] = pd.to_numeric(df["forks"], errors="coerce").fillna(0).astype(int)
    else:
        df["forks"] = 0

    # Filtering mask
    mask = pd.Series(True, index=df.index)

    # Filter archived
    if "isArchived" in df.columns:
        mask &= (df["isArchived"] == 0) | (df["isArchived"] == False)

    # Filter isFork
    if "isFork" in df.columns:
        mask &= (df["isFork"] == 0) | (df["isFork"] == False)

    # Filter forkingAllowed
    if "forkingAllowed" in df.columns:
        mask &= (df["forkingAllowed"] == 1) | (df["forkingAllowed"] == True)

    # Filter stars
    if min_stars > 0:
        mask &= df["stars"] >= min_stars

    # Filter license (must have a recognized license, not null / none / other / na)
    if "license" in df.columns:
        license_str = df["license"].astype(str).str.strip().str.lower()
        valid_license = (
            df["license"].notna()
            & (license_str != "")
            & (license_str != "none")
            & (license_str != "null")
            & (license_str != "nan")
            & (license_str != "other")
            & (license_str != "noassertion")
        )
        mask &= valid_license

    filtered = df[mask].copy()

    # Deduplicate on full_name, keeping highest stars
    filtered = filtered.sort_values(by="stars", ascending=False)
    filtered = filtered.drop_duplicates(subset=["full_name"], keep="first")

    # Extract owner and name if missing
    if "owner" not in filtered.columns or "name" not in filtered.columns:
        splits = filtered["full_name"].str.split("/", n=1, expand=True)
        filtered["owner"] = splits[0]
        filtered["name"] = splits[1] if splits.shape[1] > 1 else ""

    # Ensure optional text columns exist
    if "language" not in filtered.columns:
        filtered["language"] = None
    if "description" not in filtered.columns:
        filtered["description"] = ""
    if "license" not in filtered.columns:
        filtered["license"] = None

    # Apply max_repositories truncation if requested
    if max_repositories > 0:
        filtered = filtered.head(max_repositories)

    result_cols = ["full_name", "name", "owner", "stars", "forks", "language", "description", "license"]
    return filtered[result_cols].reset_index(drop=True)


def download_kaggle_dataset(
    dataset_handle: str = "pelmers/github-repository-metadata-with-5-stars",
) -> str:
    """
    Downloads the Kaggle dataset via kagglehub and returns the local directory path.
    Requires KAGGLE_USERNAME and KAGGLE_KEY or ~/.kaggle/kaggle.json.
    """
    try:
        import kagglehub
        logger.info("Downloading dataset %s via kagglehub...", dataset_handle)
        path = kagglehub.dataset_download(dataset_handle)
        logger.info("Dataset downloaded to: %s", path)
        return path
    except Exception as exc:
        logger.error("Failed to download dataset from Kaggle: %s", exc)
        raise


def load_dataset_file(path: str) -> pd.DataFrame:
    """
    Loads dataset from a JSON, CSV, or Parquet file or directory.
    """
    if os.path.isdir(path):
        # Find first matching data file in directory
        for root, _, files in os.walk(path):
            for file in files:
                if file.endswith((".json", ".jsonl", ".parquet", ".csv")):
                    path = os.path.join(root, file)
                    break
            if not os.path.isdir(path):
                break

    logger.info("Loading dataset file: %s", path)
    if path.endswith((".json", ".jsonl")):
        try:
            return pd.read_json(path, lines=True)
        except Exception:
            return pd.read_json(path)
    elif path.endswith(".parquet"):
        return pd.read_parquet(path)
    elif path.endswith(".csv"):
        return pd.read_csv(path)
    else:
        raise ValueError(f"Unsupported file format: {path}")


def upsert_repositories_to_db(
    database_url: str,
    df: pd.DataFrame,
    batch_size: int = 2000,
) -> int:
    """
    Bulk upserts repository records into PostgreSQL and initializes sync_state.
    """
    if df.empty:
        logger.warning("No repositories to upsert")
        return 0

    try:
        import psycopg
        use_psycopg3 = True
    except ImportError:
        import psycopg2 as psycopg  # fallback
        use_psycopg3 = False

    logger.info("Connecting to database: %s", database_url.split("@")[-1])
    conn = psycopg.connect(database_url)
    total_upserted = 0

    try:
        with conn.cursor() as cur:
            # Batch upsert repositories
            upsert_sql = """
            INSERT INTO repositories (full_name, name, owner, stars, forks, language, description, license, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (full_name) DO UPDATE SET
                stars = EXCLUDED.stars,
                forks = EXCLUDED.forks,
                language = COALESCE(EXCLUDED.language, repositories.language),
                description = COALESCE(EXCLUDED.description, repositories.description),
                license = COALESCE(EXCLUDED.license, repositories.license),
                updated_at = NOW();
            """

            records = []
            for row in df.itertuples(index=False):
                records.append((
                    row.full_name,
                    row.name,
                    row.owner,
                    int(row.stars),
                    int(row.forks),
                    str(row.language) if pd.notna(row.language) and row.language else None,
                    str(row.description) if pd.notna(row.description) and row.description else "",
                    str(row.license) if pd.notna(row.license) and row.license else None,
                ))

                if len(records) >= batch_size:
                    cur.executemany(upsert_sql, records)
                    conn.commit()
                    total_upserted += len(records)
                    logger.info("Upserted %d / %d repositories...", total_upserted, len(df))
                    records = []

            if records:
                cur.executemany(upsert_sql, records)
                conn.commit()
                total_upserted += len(records)

            # Ensure sync_state rows exist for all repositories
            logger.info("Populating sync_state entries for any new repositories...")
            cur.execute("""
            INSERT INTO sync_state (repository_id, priority, created_at, updated_at)
            SELECT id, 0, NOW(), NOW()
            FROM repositories
            ON CONFLICT (repository_id) DO NOTHING;
            """)
            conn.commit()

        logger.info("Successfully upserted %d repositories into PostgreSQL", total_upserted)
        return total_upserted
    finally:
        conn.close()
