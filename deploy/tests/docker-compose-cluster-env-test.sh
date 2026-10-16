#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

python3 - <<'PY'
import json
import os
from pathlib import Path
import subprocess
import tempfile

expected = {
    "REDIS_MODE": "cluster",
    "REDIS_CLUSTER_ADDRS": "redis-0:6379,redis-1:6379,redis-2:6379",
    "DATABASE_READ_REPLICA_ENABLED": "true",
    "DATABASE_READ_REPLICA_HOST": "postgres-read",
    "DATABASE_READ_REPLICA_PORT": "5433",
    "DATABASE_READ_REPLICA_USER": "reporter",
    "DATABASE_READ_REPLICA_PASSWORD": "test-read-password",
    "DATABASE_READ_REPLICA_DBNAME": "sub2api",
    "DATABASE_READ_REPLICA_SSLMODE": "require",
    "DATABASE_READ_REPLICA_MAX_OPEN_CONNS": "20",
    "DATABASE_READ_REPLICA_MAX_IDLE_CONNS": "5",
    "DATABASE_READ_REPLICA_CONNECT_TIMEOUT_SECONDS": "4",
    "OPS_METRICS_ENABLED": "true",
    "OPS_METRICS_TOKEN": "test-metrics-token",
}
for path in sorted(Path("deploy").glob("docker-compose*.yml")):
    env = dict(os.environ, POSTGRES_PASSWORD="test-password", DATABASE_HOST="postgres-write",
               DATABASE_PASSWORD="test-password", JWT_SECRET="test-jwt-secret",
               TOTP_ENCRYPTION_KEY="test-totp-key", SERVER_FRONTEND_URL="https://example.com",
               SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS="api.openai.com", **expected)
    # Cluster bootstrap addresses must suffice without a standalone REDIS_HOST.
    env.pop("REDIS_HOST", None)
    # Production has a service env_file. Use an empty isolated file instead of
    # reading an operator's .env or requiring one in a clean checkout.
    with tempfile.TemporaryDirectory(prefix="modurelay-compose-test-") as directory:
        config = Path(directory) / path.name
        config.write_text(path.read_text())
        (Path(directory) / ".env").write_text("")
        rendered = subprocess.run(
            ["docker", "compose", "--env-file", "/dev/null", "-f", str(config),
             "config", "--format", "json"],
            env=env, capture_output=True, text=True,
        )
    assert rendered.returncode == 0, f"{path}: {rendered.stderr}"
    services = json.loads(rendered.stdout)["services"]
    actual = services.get("sub2api", services.get("modurelay"))["environment"]
    for key, value in expected.items():
        assert actual.get(key) == value, f"{path}: {key} not passed to application"
print("docker compose cluster environment test passed")
PY
