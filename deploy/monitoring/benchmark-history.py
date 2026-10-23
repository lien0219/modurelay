#!/usr/bin/env python3
"""Import 31 days of synthetic metrics and benchmark the production PromQL.

Uses its own temporary TSDB/container, records a local baseline, and always
removes its container and data. It never connects to the application's DB.
"""
import argparse
import json
import math
import os
from pathlib import Path
import shutil
import socket
import statistics
import subprocess
import tempfile
import time
from urllib.parse import urlencode
from urllib.request import urlopen

parser = argparse.ArgumentParser()
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--image', default='prom/prometheus:v3.15.1')
parser.add_argument('--pods', type=int, default=3)
parser.add_argument('--runs', type=int, default=5)
parser.add_argument('--target-p95-seconds', type=float, default=2.0)
args = parser.parse_args()
if not 1 <= args.pods <= 20 or not 3 <= args.runs <= 20:
    raise SystemExit('pods must be 1..20; runs must be 3..20')
repo = Path(__file__).resolve().parents[2]
name = 'modurelay-tsdb-benchmark-' + str(os.getpid())

def disk_guard(path):
    free = shutil.disk_usage(path).free
    if free <= 5 * 1024**3:
        raise RuntimeError('Less than 5 GiB free; stopping before creating more data')
    return free

def command(argv, log):
    result = subprocess.run(argv, stdout=log, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError('Command failed: ' + ' '.join(argv[:4]) + '; inspect benchmark log')

def query(base, expression, start, end, step):
    params = urlencode({'query': expression, 'start': start, 'end': end, 'step': step})
    begin = time.perf_counter()
    with urlopen(base + '/api/v1/query_range?' + params, timeout=60) as response:
        data = json.load(response)
    elapsed = time.perf_counter() - begin
    if data.get('status') != 'success' or not data['data']['result']:
        raise RuntimeError('A benchmark query returned no usable series')
    count = sum(len(series.get('values', [])) for series in data['data']['result'])
    if count == 0:
        raise RuntimeError('A benchmark query returned no points')
    return elapsed, len(data['data']['result']), count

args.output.parent.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory(prefix='modurelay-history-benchmark-') as temporary:
    root = Path(temporary)
    disk_before = disk_guard(root)
    data_dir = root / 'tsdb'; data_dir.mkdir()
    fixtures = root / 'fixtures'; fixtures.mkdir()
    # Daily inputs avoid promtool rescanning a month-sized OpenMetrics file
    # for every two-hour output block. Boundaries align to TSDB block edges.
    data_end = int(time.time() // 7200 * 7200)
    start = data_end - 31 * 86400
    end = data_end - 60
    sample_count = 0
    series = set()
    seeds = []
    for day in range(31):
        seed = fixtures / ('day-%02d.om' % day)
        seeds.append(seed)
        sample_times = range(start + day * 86400, start + (day + 1) * 86400, 60)
        with seed.open('w') as output:
            def emit(metric, labels, factor, gauge=False):
                global sample_count
                label_text = ','.join(k + '=' + json.dumps(str(v)) for k, v in labels.items())
                series.add((metric, label_text))
                for timestamp in sample_times:
                    value = factor if gauge else factor * ((timestamp - start) // 60 + 1)
                    output.write('%s{%s} %.6f %d\n' % (metric, label_text, value, timestamp))
                    sample_count += 1
            output.write('# TYPE modurelay_http_requests counter\n')
            for pod in range(args.pods):
                for route in ['messages', 'chat_completions']:
                    labels = dict(cluster='benchmark', namespace='benchmark', service='api', pod='api-' + str(pod), uid='uid-' + str(pod), instance='api-' + str(pod), source='app', method='POST', route=route, protocol='http')
                    emit('modurelay_http_requests_total', dict(labels, status_code='200'), 99)
                    emit('modurelay_http_requests_total', dict(labels, status_code='500'), 1)
            output.write('# TYPE modurelay_http_request_duration_seconds histogram\n')
            for pod in range(args.pods):
                for route in ['messages', 'chat_completions']:
                    labels = dict(cluster='benchmark', namespace='benchmark', service='api', pod='api-' + str(pod), uid='uid-' + str(pod), instance='api-' + str(pod), source='app', method='POST', route=route, protocol='http')
                    for bucket, factor in [('0.1', 30), ('0.5', 70), ('1', 90), ('5', 99), ('+Inf', 100)]:
                        emit('modurelay_http_request_duration_seconds_bucket', dict(labels, le=bucket), factor)
                    emit('modurelay_http_request_duration_seconds_count', labels, 100)
                    emit('modurelay_http_request_duration_seconds_sum', labels, 50)
            output.write('# TYPE up gauge\n')
            for pod in range(args.pods):
                labels = dict(cluster='benchmark', namespace='benchmark', service='api', pod='api-' + str(pod), uid='uid-' + str(pod), instance='api-' + str(pod), job='modurelay-instance')
                emit('up', labels, 1, gauge=True)
            output.write('# EOF\n')
        disk_guard(root)
    series_count = len(series)
    fixture_bytes = sum(seed.stat().st_size for seed in seeds)
    # Compile the actual fixed query builder in an isolated stdlib-only module.
    query_dir = root / 'queries'; query_dir.mkdir()
    for filename in ['ops_monitoring_models.go', 'ops_monitoring_promql.go']:
        source = (repo / 'backend/internal/service' / filename).read_text()
        (query_dir / filename).write_text(source.replace('package service', 'package main', 1))
    (query_dir / 'go.mod').write_text('module monitoring-benchmark-queries\n\ngo 1.27.3\n')
    (query_dir / 'main.go').write_text('''package main
import ("encoding/json"; "os")
func monitoringWindowSeconds(s string)(int,error){return map[string]int{"1m":60,"5m":300,"15m":900,"60m":3600}[s],nil}
func main(){out:=map[string]string{};for _,m:=range []string{"qps","http_p95_ms","http_p99_ms","http_5xx_rate","request_count"}{q:=OpsMonitoringQuery{Metric:m,Window:"5m",Cluster:"benchmark",Namespace:"benchmark",Service:"api",Source:"app"};expr,_,err:=monitoringPromQL(q);if err!=nil{panic(err)};out[m]=expr};if err:=json.NewEncoder(os.Stdout).Encode(out);err!=nil{panic(err)}}
''')
    queries = json.loads(subprocess.check_output(['go', 'run', '.'], cwd=query_dir, env=dict(os.environ, GOWORK='off')))
    config = fixtures / 'prometheus.yml'
    config.write_text('global:\n  scrape_interval: 15s\nscrape_configs: []\n')
    with args.output.with_suffix('.log').open('w') as log:
        for seed in seeds:
            command(['docker', 'run', '--rm', '--name', name + '-import', '--user', '0:0', '-v', str(fixtures) + ':/fixtures:ro', '-v', str(data_dir) + ':/prometheus', '--entrypoint', '/bin/promtool', args.image, 'tsdb', 'create-blocks-from', 'openmetrics', '/fixtures/' + seed.name, '/prometheus'], log)
            disk_guard(root)
        disk_guard(root)
        tsdb_bytes = sum(p.stat().st_size for p in data_dir.rglob('*') if p.is_file())
        with socket.socket() as probe:
            probe.bind(('127.0.0.1', 0)); port = probe.getsockname()[1]
        try:
            command(['docker', 'run', '-d', '--name', name, '--user', '0:0', '-p', '127.0.0.1:%d:9090' % port, '-v', str(fixtures) + ':/fixtures:ro', '-v', str(data_dir) + ':/prometheus', args.image, '--config.file=/fixtures/prometheus.yml', '--storage.tsdb.path=/prometheus', '--storage.tsdb.retention.time=40d', '--storage.tsdb.min-block-duration=2h', '--storage.tsdb.max-block-duration=2h'], log)
            base = 'http://127.0.0.1:' + str(port)
            deadline = time.monotonic() + 90
            while True:
                try:
                    with urlopen(base + '/-/ready', timeout=2) as response:
                        if response.status == 200: break
                except Exception:
                    if time.monotonic() >= deadline: raise RuntimeError('Benchmark Prometheus did not become ready')
                    time.sleep(1)
            results = []
            for days, step in [(7, 300), (30, 900)]:
                for metric, expression in queries.items():
                    query(base, expression, end - days * 86400, end, step)  # warmup
                    durations = []
                    for run in range(args.runs):
                        duration, returned_series, points = query(base, expression, end - days * 86400, end, step)
                        durations.append(duration)
                    ordered = sorted(durations)
                    p95 = ordered[math.ceil(.95 * len(ordered)) - 1]
                    results.append(dict(metric=metric, days=days, step_seconds=step, returned_series=returned_series, points=points, p50_seconds=statistics.median(durations), p95_seconds=p95, max_seconds=max(durations), local_target_met=p95 < args.target_p95_seconds, durations_seconds=durations))
                    print('%s %dd p95=%.3fs (%d points)' % (metric, days, p95, points), flush=True)
                    disk_guard(root)
            report = dict(scope='local synthetic baseline; not production acceptance or a production SLO', image=args.image, generated_days=31, sampling_seconds=60, pods=args.pods, input_series=series_count, input_samples=sample_count, fixture_bytes=fixture_bytes, tsdb_bytes=tsdb_bytes, free_bytes_before=disk_before, free_bytes_after=disk_guard(root), local_target_p95_seconds=args.target_p95_seconds, runs=args.runs, queries=queries, results=results)
            args.output.write_text(json.dumps(report, indent=2))
        finally:
            subprocess.run(['docker', 'logs', name], stdout=log, stderr=subprocess.STDOUT)
            subprocess.run(['docker', 'rm', '-f', name], stdout=log, stderr=subprocess.STDOUT)
print('Local baseline saved to ' + str(args.output))
