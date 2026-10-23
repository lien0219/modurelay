"""Create the local fixture bucket using S3 Signature V4 and no dependencies.

This utility only contacts the fixed MinIO service on the internal Compose
network. Cloud bucket provisioning is intentionally left to the operator's IaC.
"""
from datetime import datetime, timezone
import hashlib
import hmac
from pathlib import Path
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


def sign(key, text):
    return hmac.new(key, text.encode(), hashlib.sha256).digest()


def create_bucket():
    access = Path('/run/secrets/minio_root_user').read_text().strip()
    secret = Path('/run/secrets/minio_root_password').read_text().strip()
    now = datetime.now(timezone.utc)
    date = now.strftime('%Y%m%d')
    timestamp = now.strftime('%Y%m%dT%H%M%SZ')
    payload_hash = hashlib.sha256(b'').hexdigest()
    headers = 'host:minio:9000\nx-amz-content-sha256:' + payload_hash + '\nx-amz-date:' + timestamp + '\n'
    signed_headers = 'host;x-amz-content-sha256;x-amz-date'
    canonical = '\n'.join(['PUT', '/modurelay-metrics', '', headers, signed_headers, payload_hash])
    scope = date + '/us-east-1/s3/aws4_request'
    string_to_sign = '\n'.join(['AWS4-HMAC-SHA256', timestamp, scope, hashlib.sha256(canonical.encode()).hexdigest()])
    key = sign(sign(sign(sign(('AWS4' + secret).encode(), date), 'us-east-1'), 's3'), 'aws4_request')
    signature = hmac.new(key, string_to_sign.encode(), hashlib.sha256).hexdigest()
    authorization = 'AWS4-HMAC-SHA256 Credential=' + access + '/' + scope + ', SignedHeaders=' + signed_headers + ', Signature=' + signature
    request = Request('http://minio:9000/modurelay-metrics', data=b'', method='PUT', headers={
        'X-Amz-Date': timestamp, 'X-Amz-Content-Sha256': payload_hash, 'Authorization': authorization,
    })
    try:
        with urlopen(request, timeout=3) as response:
            return response.status == 200
    except HTTPError as error:
        try:
            body = error.read(4096)
            if error.code == 409 and (b'BucketAlreadyOwnedByYou' in body or b'BucketAlreadyExists' in body):
                return True
            if error.code in (400, 401, 403):
                raise RuntimeError('Local bucket setup rejected authentication or request format') from None
        finally:
            error.close()
    except (URLError, TimeoutError):
        pass
    return False


if __name__ == '__main__':
    for attempt in range(60):
        if create_bucket():
            print('Local monitoring bucket ready')
            break
        time.sleep(2)
    else:
        raise SystemExit('Local object storage did not become ready')
