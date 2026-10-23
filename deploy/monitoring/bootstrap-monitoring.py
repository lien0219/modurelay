#!/usr/bin/env python3
"""Create local-only secrets without printing credentials or overwriting files."""
import argparse
import json
import os
from pathlib import Path
import secrets

parser = argparse.ArgumentParser()
parser.add_argument('--directory', type=Path, default=Path(__file__).resolve().parent / 'secrets')
args = parser.parse_args()
root = args.directory.resolve()
root.mkdir(mode=0o700, parents=True, exist_ok=True)
os.chmod(root, 0o700)

# Compose mounts local secrets as bind files and cannot remap their UID. The
# 0700 parent prevents other host users from traversing into this directory;
# 0644 files allow each non-root container to read only its mounted secrets.
def ensure(name, value):
    path = root / name
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
    except FileExistsError:
        return path.read_text().strip()
    with os.fdopen(fd, 'w') as output:
        output.write(value + '\n')
    return value

user = ensure('minio-root-user', 'monitor-' + secrets.token_hex(8))
password = ensure('minio-root-password', secrets.token_hex(32))
ensure('receiver-token', secrets.token_hex(32))
ensure('app-metrics-token', secrets.token_hex(32))
ensure('application-targets.json', '[]')
ensure('cluster-targets.json', '[]')
ensure('object-store.yml', '\n'.join([
    'type: S3', 'config:', '  bucket: modurelay-metrics',
    '  endpoint: minio:9000', '  insecure: true',
    '  access_key: ' + json.dumps(user), '  secret_key: ' + json.dumps(password),
]))
print('Local monitoring secrets ready in ' + str(root))
print('Application targets remain empty until explicitly configured. No external notifications are configured.')
