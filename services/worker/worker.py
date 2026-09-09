import os
import time
import httpx

api_url = os.getenv('API_URL', 'http://api:8000')
interval = int(os.getenv('SYNC_INTERVAL_SECONDS', '900'))
print(f'worker ready; priority refresh interval={interval}s')
while True:
    try:
        response = httpx.get(f'{api_url}/health', timeout=5)
        print(f'worker health check: {response.status_code}', flush=True)
    except httpx.HTTPError as error:
        print(f'worker waiting for API: {error}', flush=True)
    time.sleep(interval)
