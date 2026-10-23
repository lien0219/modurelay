#!/usr/bin/env python3
"""Exercise only a new isolated Compose project and remove its own volumes."""
from datetime import datetime, timedelta, timezone
import argparse
from http.client import HTTPException
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

parser = argparse.ArgumentParser()
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
directory = Path(__file__).resolve().parent
project = 'modurelay-monitoring-test-' + str(os.getpid())
if shutil.disk_usage(directory).free <= 5 * 1024**3:
    raise SystemExit('Less than 5 GiB free; refusing to create a test stack')
args.output.parent.mkdir(parents=True, exist_ok=True)

def free_port():
    with socket.socket() as probe:
        probe.bind(('127.0.0.1', 0))
        return probe.getsockname()[1]

def read(url, data=None, token=None):
    headers = {'Content-Type': 'application/json'}
    if token: headers['Authorization'] = 'Bearer ' + token
    body = None if data is None else json.dumps(data).encode()
    with urlopen(Request(url, data=body, headers=headers), timeout=5) as response:
        raw = response.read()
        return json.loads(raw) if raw.startswith(b'{') else raw.decode()

def wait_for(check, seconds=180):
    deadline = time.monotonic() + seconds
    while True:
        try:
            result = check()
            if result: return result
        except (HTTPException, OSError, KeyError, ValueError):
            pass
        if time.monotonic() >= deadline:
            raise RuntimeError('Timed out waiting for isolated monitoring stack')
        time.sleep(1)

with tempfile.TemporaryDirectory(prefix=project + '-') as temporary:
    root = Path(temporary)
    secret_dir = root / 'secrets'
    subprocess.run(['python3', str(directory / 'bootstrap-monitoring.py'), '--directory', str(secret_dir)], check=True, stdout=subprocess.DEVNULL)
    ports = dict(prometheus=free_port(), query=free_port(), alertmanager=free_port(), receiver=free_port())
    env = dict(os.environ, MONITORING_SECRETS_DIR=str(secret_dir), WATCHDOG_TIMEOUT_SECONDS='12', MONITORING_PROMETHEUS_PORT=str(ports['prometheus']), MONITORING_QUERY_PORT=str(ports['query']), MONITORING_ALERTMANAGER_PORT=str(ports['alertmanager']), MONITORING_RECEIVER_PORT=str(ports['receiver']))
    compose = ['docker', 'compose', '-p', project, '-f', str(directory / 'thanos-retention.compose.yml'), '--profile', 'local-object-store']
    token = (secret_dir / 'receiver-token').read_text().strip()
    with args.output.with_suffix('.log').open('w') as log:
        def run(*arguments):
            subprocess.run(compose + list(arguments), env=env, check=True, stdout=log, stderr=subprocess.STDOUT)
        try:
            run('config', '--quiet')
            run('up', '-d', '--build')
            base = {key: 'http://127.0.0.1:' + str(port) for key, port in ports.items()}
            wait_for(lambda: read(base['prometheus'] + '/-/ready'))
            wait_for(lambda: read(base['alertmanager'] + '/-/ready'))
            wait_for(lambda: read(base['query'] + '/-/ready'))
            live = wait_for(lambda: read(base['query'] + '/api/v1/query?query=up%7Bjob%3D%22prometheus%22%7D')['data']['result'])
            healthy_thanos = wait_for(lambda: (lambda rows: rows if len(rows) == 4 and all(r['value'][1] == '1' for r in rows) else None)(read(base['prometheus'] + '/api/v1/query?query=up%7Bjob%3D%22thanos%22%7D')['data']['result']))
            # Put a real closed TSDB block into S3, then query it through Store.
            # It never enters Prometheus's TSDB, so Sidecar cannot supply it.
            fixture = root / 'store-fixture'; fixture.mkdir()
            fixture_time = int(time.time() // 7200 * 7200) - 14400
            (fixture / 'data.om').write_text('# TYPE modurelay_store_fixture counter\n' + ''.join('modurelay_store_fixture_total ' + str(i + 1) + ' ' + str(fixture_time + i * 60) + '\n' for i in range(120)) + '# EOF\n')
            subprocess.run(['docker', 'run', '--rm', '--user', '0:0', '-v', str(fixture) + ':/fixture', '--entrypoint', '/bin/promtool', 'prom/prometheus:v3.15.1', 'tsdb', 'create-blocks-from', 'openmetrics', '/fixture/data.om', '/fixture/blocks'], check=True, stdout=log, stderr=subprocess.STDOUT)
            run('run', '--rm', '--no-deps', '--user', '0:0', '-v', str(fixture) + ':/fixture', 'sidecar', 'tools', 'bucket', 'upload-blocks', '--path=/fixture/blocks', '--objstore.config-file=/run/secrets/object_store', '--label=cluster="local-store-fixture"')
            run('restart', 'store')
            stored = wait_for(lambda: read(base['query'] + '/api/v1/query?query=modurelay_store_fixture_total&time=' + str(fixture_time + 7140))['data']['result'])
            if stored[0]['value'][1] != '120': raise RuntimeError('Stored TSDB fixture query returned the wrong value')
            before = wait_for(lambda: (lambda s: s if s['watchdog_seen'] else None)(read(base['receiver'] + '/events', token=token)))
            now = datetime.now(timezone.utc)
            read(base['alertmanager'] + '/api/v2/alerts', data=[{'labels': {'alertname': 'ModuRelayIsolatedSmoke', 'cluster': 'local-evidence', 'severity': 'warning'}, 'annotations': {'summary': 'Local isolated test; no user notification'}, 'startsAt': now.isoformat(), 'endsAt': (now + timedelta(minutes=5)).isoformat()}])
            received = wait_for(lambda: (lambda s: s if any(e['labels'].get('alertname') == 'ModuRelayIsolatedSmoke' for e in s['events']) else None)(read(base['receiver'] + '/events', token=token)))
            run('restart', 'watchdog-receiver')
            restored = wait_for(lambda: read(base['receiver'] + '/events', token=token))
            if restored['received'] < received['received']:
                raise RuntimeError('Receiver lost persisted notification evidence on restart')
            # Stop the complete collection/delivery pipeline, keeping the
            # independent receiver alive to prove dead-man timeout behavior.
            run('stop', 'prometheus', 'alertmanager', 'sidecar', 'query', 'store', 'compact')
            def expired():
                try:
                    read(base['receiver'] + '/healthz')
                except HTTPError as error:
                    code = error.code; error.close()
                    return code == 503
                return False
            wait_for(expired, 30)
            images = json.loads(subprocess.check_output(compose + ['images', '--format', 'json'], env=env))
            args.output.write_text(json.dumps(dict(scope='isolated local Docker evidence; no external notification sent', project=project, ports=ports, live_thanos_series=len(live), healthy_thanos_components=len(healthy_thanos), object_store_historical_query=True, watchdog_received=True, alertmanager_test_received=True, receiver_persistence=True, missing_pipeline_detected=True, received_events=received['received'], image_versions=images), indent=2))
            print('PASS: Thanos live and object-store query, watchdog, authenticated Alertmanager delivery, persisted restart, and pipeline-loss detection', flush=True)
        finally:
            subprocess.run(compose + ['logs', '--no-color', '--tail', '150'], env=env, stdout=log, stderr=subprocess.STDOUT)
            subprocess.run(compose + ['down', '--volumes', '--remove-orphans'], env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
