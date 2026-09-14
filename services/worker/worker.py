"""SkillIssues Background Priority Refresh Worker.

This worker runs as a continuous background daemon that periodically triggers catalog
synchronization and updates issue states (open/closed, comment counts, new open-source issues)
from the ingestion service without blocking user-facing API request threads.
"""

import os
import time
import httpx

# Internal service URLs within the Docker network
api_url = os.getenv("API_URL", "http://api:8000")
ingestion_url = os.getenv("INGESTION_URL", "http://ingestion:8000")

# Interval between catalog sync cycles (default: 900 seconds / 15 minutes)
interval = int(os.getenv("SYNC_INTERVAL_SECONDS", "900"))

print(f"[Worker] Started: priority issue refresh interval = {interval}s", flush=True)

while True:
    try:
        # 1. Probe the main API gateway health status
        response = httpx.get(f"{api_url}/health", timeout=5)
        print(f"[Worker] API health probe: {response.status_code}", flush=True)

        # 2. Trigger priority open issue catalog synchronization
        response = httpx.post(f"{ingestion_url}/sync/issues", timeout=120)
        print(f"[Worker] Issue refresh result: {response.status_code} {response.text}", flush=True)

    except httpx.HTTPError as error:
        # Gracefully handle transient connection errors (e.g. during service startup)
        print(f"[Worker] Connection warning (waiting for services): {error}", flush=True)

    # Sleep for the configured interval before the next refresh cycle
    time.sleep(interval)
