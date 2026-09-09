import os
import time
import httpx

api_url = os.getenv('API_URL', 'http://api:8000')
ingestion_url = os.getenv('INGESTION_URL', 'http://ingestion:8000')
interval = int(os.getenv('SYNC_INTERVAL_SECONDS', '900'))
print(f'worker ready; priority issue refresh interval={interval}s', flush=True)
while True:
    try:
        response = httpx.get(f'{api_url}/health', timeout=5)
        print(f'worker health check: {response.status_code}', flush=True)
        response = httpx.post(f'{ingestion_url}/sync/issues', timeout=120)
        print(f'worker issue refresh: {response.status_code} {response.text}', flush=True)
    except httpx.HTTPError as error:
        print(f'worker waiting for API: {error}', flush=True)
    time.sleep(interval)
