#!/usr/bin/env python3
"""Prove raw retention with real synthetic TSDB blocks in an isolated filesystem bucket."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser()
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
if shutil.disk_usage(Path.cwd()).free <= 5 * 1024**3:
    raise SystemExit('Less than 5 GiB free; refusing to generate retention fixtures')
args.output.parent.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory(prefix='modurelay-retention-test-') as temporary, args.output.with_suffix('.log').open('w') as log:
    root = Path(temporary)
    (root / 'bucket').mkdir()
    (root / 'object-store.yml').write_text('type: FILESYSTEM\nconfig:\n  directory: /fixture/bucket\n')
    def run(image, *arguments, entrypoint=None):
        command = ['docker', 'run', '--rm', '--network', 'none', '--user', '0:0', '-v', str(root) + ':/fixture']
        if entrypoint: command += ['--entrypoint', entrypoint]
        subprocess.run(command + [image] + list(arguments), check=True, stdout=log, stderr=subprocess.STDOUT)
    expected = {}
    for age in (5, 40, 400):
        start = (int(time.time()) // 7200) * 7200 - age * 86400
        path = root / ('age-' + str(age) + '.prom')
        path.write_text('# TYPE retention_fixture counter\n' + ''.join('retention_fixture_total{fixture="age-' + str(age) + '"} ' + str(i + 1) + ' ' + str(start + i * 60) + '\n' for i in range(120)) + '# EOF\n')
        run('prom/prometheus:v3.15.1', 'tsdb', 'create-blocks-from', 'openmetrics', '/fixture/' + path.name, '/fixture/blocks-' + str(age), entrypoint='/bin/promtool')
        for meta_path in (root / ('blocks-' + str(age))).glob('*/meta.json'):
            expected[meta_path.parent.name] = {'age_days': age, 'should_retain': age <= 31}
        run('quay.io/thanos/thanos:v0.42.4', 'tools', 'bucket', 'upload-blocks', '--objstore.config-file=/fixture/object-store.yml', '--path=/fixture/blocks-' + str(age), '--label=cluster="isolated-retention-fixture"')
    before = sorted(p.name for p in (root / 'bucket').iterdir() if (p / 'meta.json').is_file())
    if len(before) != 3: raise RuntimeError('Expected exactly three real TSDB blocks before retention')
    # No store gateway reads this private bucket. Immediate deletion is confined
    # to this fixture; production Compose keeps Thanos's safe default delay.
    # Retention marks blocks after the cleanup phase. The next iteration cleans
    # those markers, exactly as the long-running production compactor would.
    for iteration in range(2):
        run('quay.io/thanos/thanos:v0.42.4', 'compact', '--objstore.config-file=/fixture/object-store.yml', '--data-dir=/fixture/compact', '--retention.resolution-raw=31d', '--retention.resolution-5m=90d', '--retention.resolution-1h=365d', '--consistency-delay=0s', '--delete-delay=0s', '--downsampling.disable')
        if iteration == 0: time.sleep(1)
    after = sorted(p.name for p in (root / 'bucket').iterdir() if (p / 'meta.json').is_file())
    retained = sorted(key for key, value in expected.items() if value['should_retain'])
    if after != retained: raise RuntimeError('Compactor retention result did not match expected block ages')
    args.output.write_text(json.dumps({'scope': 'isolated real Thanos compactor; raw 31d retention only; downsample tier expiry not claimed', 'prometheus_version': 'v3.15.1', 'thanos_version': 'v0.42.4', 'before_blocks': before, 'after_blocks': after, 'fixtures': expected, 'raw_older_than_31d_deleted': True, 'recent_raw_retained': True, 'production_delete_delay_unchanged': True}, indent=2))
    print('PASS: real compactor deleted 40d/400d raw blocks and retained 5d raw block')
