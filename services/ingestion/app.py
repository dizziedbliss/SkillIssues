import os
import time
import psycopg

url = os.getenv('DATABASE_URL')
print('ingestion service ready; seed with database/seed/seed.sql')
if url:
    with psycopg.connect(url) as connection:
        connection.execute('SELECT 1')
while True:
    time.sleep(900)
