#!/usr/bin/env python3
"""Isolated two-instance JSON -> Fluent Bit -> Loki retention-of-source smoke."""
import datetime, json, os, pathlib, subprocess, tempfile, time, urllib.parse, urllib.request, uuid
ROOT = pathlib.Path(__file__).resolve().parents[2]
project = 'modurelay-log-test-' + uuid.uuid4().hex[:8]
request_id = 'synthetic-' + uuid.uuid4().hex
report = {'project': project, 'checks': [], 'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat()}
with tempfile.TemporaryDirectory(prefix='modurelay-log-test-') as tmp:
    env = dict(os.environ, LOG_DIRECTORY=tmp, LOKI_PORT='0')
    cmd = ['docker', 'compose', '-p', project, '-f', str(ROOT/'deploy/logging/compose.yml')]
    def compose(*args, **kwargs):
        return subprocess.run(cmd + list(args), env=env, check=True, text=True, **kwargs)
    try:
        for name in ['pod-before-delete', 'pod-surviving']:
            row = {'service':'modurelay','cluster':'isolated-test','namespace':'verification','pod':name,'pod_uid':name+'-uid','instance':name,'request_id':request_id,'level':'info','message':'synthetic streamed response completed'}
            (pathlib.Path(tmp)/(name+'.log')).write_text(json.dumps(row)+'\n')
        compose('up','-d', stdout=subprocess.DEVNULL)
        address = compose('port','loki','3100',capture_output=True).stdout.strip()
        query = '{job="modurelay",cluster="isolated-test"} | json | request_id="'+request_id+'"'
        def read():
            url = 'http://'+address+'/loki/api/v1/query_range?'+urllib.parse.urlencode({'query':query,'limit':100,'start':str(int((time.time()-600)*1e9))})
            with urllib.request.urlopen(url,timeout=5) as r:
                body=json.load(r)
            return [json.loads(line) for stream in body['data']['result'] for _,line in stream['values']]
        deadline=time.monotonic()+120
        while time.monotonic()<deadline:
            try:
                records=read()
                if len({r['pod_uid'] for r in records})==2: break
            except Exception: pass
            time.sleep(2)
        else: raise RuntimeError('both instance logs were not queryable within 120s')
        report['checks'].append({'name':'same Request ID across two Pod UIDs','passed':True})
        for file in pathlib.Path(tmp).glob('*.log'): file.unlink()
        records=read()
        assert len({r['pod_uid'] for r in records})==2
        report['checks'].append({'name':'archived log remains after source files are deleted','passed':True})
        assert all('authorization' not in r and 'api_key' not in r for r in records)
        report['checks'].append({'name':'synthetic records retain correlation fields without credentials','passed':True})
        report['passed']=True
    except Exception as exc:
        report['passed']=False;report['error']=str(exc)
        compose('logs','--tail','30')
        raise
    finally:
        compose('down','--volumes','--remove-orphans',stdout=subprocess.DEVNULL)
        report['cleaned_up']=True
        dest=ROOT/'outputs/cluster-monitoring-review-20261023/test-evidence/central-logs.json'
        dest.parent.mkdir(parents=True,exist_ok=True);dest.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
        print(json.dumps(report,ensure_ascii=False))
