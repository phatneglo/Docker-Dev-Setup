import requests
from minio import Minio
from datetime import timedelta

# Test presigned URL generation and access
client = Minio('sgp1.digitaloceanspaces.com', 
               access_key='DO00WVEWRUJZDU2XGJRB', 
               secret_key='7kAG4m6BfRE07mnnZygCwQSTqn+hpKmK0o9zVGT0D+4', 
               secure=True)

print('Generating presigned URL...')
try:
    url = client.presigned_get_object('pg-itbs-dev', 'uploads/b7dc2813-8fb9-449e-b3b0-d4183f756733.docx', expires=timedelta(hours=1))
    print(f'Presigned URL generated: {url[:100]}...')
    
    print('\nTesting access to presigned URL...')
    response = requests.head(url, timeout=10)
    print(f'Status Code: {response.status_code}')
    print(f'Content-Length: {response.headers.get("Content-Length", "Not found")}')
    print(f'Content-Type: {response.headers.get("Content-Type", "Not found")}')
    print(f'Access-Control-Allow-Origin: {response.headers.get("Access-Control-Allow-Origin", "Not found")}')
    
    print('\nTesting GET request...')
    response = requests.get(url, timeout=10)
    print(f'GET Status Code: {response.status_code}')
    print(f'Content Length: {len(response.content)} bytes')
    
except Exception as e:
    print(f'Error: {e}') 