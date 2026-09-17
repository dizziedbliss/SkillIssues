"""
SkillIssues Ingestion CLI
Command-line interface to execute the Kaggle ETL pipeline.
"""

from __future__ import annotations

import argparse
import os
import sys
from python.ingestion.etl import (
    download_kaggle_dataset,
    filter_dataframe,
    load_dataset_file,
    upsert_repositories_to_db,
    logger,
)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="SkillIssues Kaggle Repository Metadata ETL",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "--dataset-path",
        type=str,
        default=os.getenv("REPOSITORY_METADATA_PATH", ""),
        help="Local path to repository metadata file (.json, .parquet, .csv) or directory",
    )
    parser.add_argument(
        "--kaggle",
        action="store_true",
        help="Download dataset automatically via kagglehub",
    )
    parser.add_argument(
        "--dataset-handle",
        type=str,
        default="pelmers/github-repository-metadata-with-5-stars",
        help="Kaggle dataset handle",
    )
    parser.add_argument(
        "--database-url",
        type=str,
        default=os.getenv("DATABASE_URL", "postgresql://skillissues:skillissues@localhost:5432/skillissues"),
        help="PostgreSQL database URL",
    )
    parser.add_argument(
        "--min-stars",
        type=int,
        default=int(os.getenv("MIN_STARS", "50")),
        help="Minimum star count filter threshold",
    )
    parser.add_argument(
        "--max-repos",
        type=int,
        default=int(os.getenv("MAX_REPOSITORIES", "0")),
        help="Maximum qualified repositories to ingest (0 for unlimited)",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=2000,
        help="PostgreSQL batch upsert chunk size",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Execute dataset load and filter without inserting into PostgreSQL",
    )
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    data_path = args.dataset_path

    if args.kaggle or not data_path:
        if args.kaggle:
            data_path = download_kaggle_dataset(args.dataset_handle)
        elif not data_path:
            # Check default locations
            candidates = [
                "/repo/repo_metadata.json",
                "./repo/repo_metadata.json",
                "./database/seed/repo_metadata.json",
            ]
            for cand in candidates:
                if os.path.exists(cand):
                    data_path = cand
                    break
            if not data_path:
                logger.error("No dataset path provided and default candidate files not found.")
                logger.error("Use --kaggle to download or --dataset-path <path>.")
                return 1

    logger.info("Starting ETL with dataset at: %s", data_path)
    df_raw = load_dataset_file(data_path)
    logger.info("Raw dataset loaded: %d rows, %d columns", len(df_raw), len(df_raw.columns))

    logger.info("Applying vectorized filters (min_stars=%d, max_repos=%d)...", args.min_stars, args.max_repos)
    df_clean = filter_dataframe(df_raw, min_stars=args.min_stars, max_repositories=args.max_repos)
    logger.info("Qualified repositories after filtering: %d", len(df_clean))

    if args.dry_run:
        logger.info("Dry run complete. Top 5 repositories:")
        for _, row in df_clean.head(5).iterrows():
            logger.info("  %s (%d stars, %s, %s)", row["full_name"], row["stars"], row["language"], row["license"])
        return 0

    count = upsert_repositories_to_db(args.database_url, df_clean, batch_size=args.batch_size)
    logger.info("ETL process completed successfully! Total repos: %d", count)
    return 0


if __name__ == "__main__":
    sys.exit(main())
