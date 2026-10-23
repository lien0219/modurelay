#!/usr/bin/env python3
"""Verify actual Thanos 5m/1h downsampling with one small, real TSDB series."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
if shutil.disk_usage(Path.cwd()).free <= 5 * 1024**3:
    raise SystemExit('Less than 5 GiB free; refusing to generate a fixture')
args.output.parent.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory(prefix='modurelay-downsample-test-') as temporary, args.output.with_suffix('.log').open('w') as log:
    root = Path(temporary)
    (root / 'bucket').mkdir()
    (root / 'object-store.yml').write_text('type: FILESYSTEM\nconfig:\n  directory: /fixture/bucket\n')
    # The official BlockWriter writes real chunk/index/meta files. A 12-day
    # block is old and long enough for both downsampling levels, while one
    # five-minute Counter series needs only 3,457 samples.
    (root / 'go.mod').write_text('module monitoring-downsample-fixture\n\ngo 1.27.3\n\nrequire github.com/prometheus/prometheus v0.315.1\n')
    (root / 'main.go').write_text('''package main
import("context";"fmt";"log/slog";"os";"time";"github.com/prometheus/prometheus/model/labels";"github.com/prometheus/prometheus/storage";"github.com/prometheus/prometheus/tsdb")
func main(){ctx:=context.Background();w,err:=tsdb.NewBlockWriter(slog.Default(),os.Args[1],int64(20*24*time.Hour/time.Millisecond));if err!=nil{panic(err)};defer w.Close();a:=w.Appender(ctx);var ref storage.SeriesRef;start:=time.Now().Add(-60*24*time.Hour).Truncate(time.Hour).UnixMilli();for i:=0;i<=12*24*12;i++{ref,err=a.Append(ref,labels.FromStrings("__name__","modurelay_downsample_fixture_total","job","fixture"),start+int64(i)*300000,float64(i*10));if err!=nil{panic(err)}};if err=a.Commit();err!=nil{panic(err)};id,err:=w.Flush(ctx);if err!=nil{panic(err)};fmt.Println(id)}
''')
    subprocess.run(['go', 'run', '-mod=mod', '.', str(root / 'blocks')], cwd=root, env=dict(os.environ, GOWORK='off'), check=True, stdout=log, stderr=subprocess.STDOUT)
    def thanos(*arguments):
        subprocess.run(['docker', 'run', '--rm', '--network', 'none', '--user', '0:0', '-v', str(root) + ':/fixture', 'quay.io/thanos/thanos:v0.42.4'] + list(arguments), check=True, stdout=log, stderr=subprocess.STDOUT)
    thanos('tools', 'bucket', 'upload-blocks', '--objstore.config-file=/fixture/object-store.yml', '--path=/fixture/blocks', '--label=cluster="isolated-downsample-fixture"')
    for iteration in range(3):
        thanos('compact', '--objstore.config-file=/fixture/object-store.yml', '--data-dir=/fixture/compact', '--consistency-delay=0s')
    blocks = []
    for path in sorted((root / 'bucket').glob('*/meta.json')):
        meta = json.loads(path.read_text())
        blocks.append(dict(ulid=meta['ulid'], resolution_ms=meta['thanos']['downsample']['resolution'], min_time_ms=meta['minTime'], max_time_ms=meta['maxTime'], stats=meta['stats']))
    resolutions = sorted(set(block['resolution_ms'] for block in blocks))
    if resolutions != [0, 300000, 3600000]:
        raise RuntimeError('Expected real raw/5m/1h blocks, found resolutions: ' + repr(resolutions))
    args.output.write_text(json.dumps(dict(scope='isolated real Thanos compactor downsampling; one synthetic Counter, no production load claim', prometheus_module='v0.315.1', thanos_image='quay.io/thanos/thanos:v0.42.4', input_series=1, input_samples=3457, input_span_days=12, compactor_iterations=3, resolutions_ms=resolutions, blocks=blocks, generated_5m=True, generated_1h=True), indent=2))
    print('PASS: real compactor generated raw, 5m and 1h TSDB blocks; resolutions 0/300000/3600000 ms')
