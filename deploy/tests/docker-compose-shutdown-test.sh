#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

python3 - <<'PY'
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

defaults = {
    "SERVER_SHUTDOWN_TIMEOUT_SECONDS": "900",
    "SERVER_DRAIN_DELAY_SECONDS": "5",
    "SERVER_CLEANUP_TIMEOUT_SECONDS": "30",
    "SERVER_FORCE_CLOSE_TIMEOUT_SECONDS": "10",
}
overrides = {
    "SERVER_SHUTDOWN_TIMEOUT_SECONDS": "1200",
    "SERVER_DRAIN_DELAY_SECONDS": "0",
    "SERVER_CLEANUP_TIMEOUT_SECONDS": "45",
    "SERVER_FORCE_CLOSE_TIMEOUT_SECONDS": "20",
    "APP_STOP_GRACE_PERIOD": "22m",
}
for path in sorted(Path("deploy").glob("docker-compose*.yml")):
    for values, stop_seconds in [(defaults, 960), (overrides, 1320)]:
        env = dict(os.environ, POSTGRES_PASSWORD="test-password", DATABASE_HOST="postgres",
                   DATABASE_PASSWORD="test-password", JWT_SECRET="test-jwt-secret",
                   TOTP_ENCRYPTION_KEY="test-totp-key", SERVER_FRONTEND_URL="https://example.com",
                   SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS="api.openai.com")
        for key in overrides:
            env.pop(key, None)
        # Default case deliberately omits SERVER_* so Compose's own defaults are tested.
        if values is overrides:
            env.update(overrides)
        with tempfile.TemporaryDirectory(prefix="modurelay-shutdown-compose-") as directory:
            config = Path(directory) / path.name
            config.write_text(path.read_text())
            (Path(directory) / ".env").write_text("")
            rendered = subprocess.run(
                ["docker", "compose", "--env-file", "/dev/null", "-f", str(config),
                 "config", "--format", "json"], env=env, capture_output=True, text=True,
            )
        assert rendered.returncode == 0, f"{path}: {rendered.stderr}"
        services = json.loads(rendered.stdout)["services"]
        app = services.get("sub2api", services.get("modurelay"))
        for key in defaults:
            assert app["environment"].get(key) == values[key], f"{path}: {key} not passed"
        # Compose renders Go duration strings, for example 16m0s.
        match = re.fullmatch(r"(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?", str(app["stop_grace_period"]))
        assert match, f"{path}: unsupported stop_grace_period {app['stop_grace_period']}"
        actual_stop = sum(float(value or 0) * scale for value, scale in zip(match.groups(), (3600, 60, 1)))
        assert actual_stop == stop_seconds, f"{path}: stop grace does not match override/default"
        budget = sum(int(values[key]) for key in defaults)
        assert actual_stop > budget, f"{path}: runtime can kill the app before cleanup completes"
        assert any("/health" in str(part) for part in app["healthcheck"]["test"]), path
for path in [Path("deploy/sub2api.service"), Path("deploy/install.sh")]:
    timeout = re.search(r"^TimeoutStopSec=(\d+)$", path.read_text(), re.MULTILINE)
    assert timeout, f"{path}: systemd stop budget is not configured"
    assert int(timeout.group(1)) > sum(map(int, defaults.values())), f"{path}: systemd timeout too short"
print("docker compose shutdown budgets and overrides passed")
PY
