#!/usr/bin/env python3
"""Own only the two explicitly recorded local demo process groups."""
import json
import fcntl
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


def identity(pid):
    result = subprocess.run(
        ['ps', '-p', str(pid), '-o', 'lstart=', '-o', 'stat='],
        capture_output=True, text=True,
        env={**os.environ, 'LC_ALL': 'C', 'TZ': 'UTC'}, check=False,
    )
    fields = result.stdout.strip().split()
    if result.returncode or len(fields) < 6 or fields[-1].startswith('Z'):
        return None
    return ' '.join(fields[:-1])


def owned(record):
    return record and identity(record['pid']) == record['started']


def main():
    if len(sys.argv) >= 4 and sys.argv[1] == 'lock':
        state = Path(sys.argv[2]).expanduser().resolve()
        state.mkdir(parents=True, exist_ok=True, mode=0o700)
        with (state / 'lifecycle.lock').open('a') as lock:
            os.chmod(state / 'lifecycle.lock', 0o600)
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                print('另一个演示环境命令正在执行，等待完成…', flush=True)
                fcntl.flock(lock, fcntl.LOCK_EX)
            env = {**os.environ, 'INVOICE_DEMO_LOCK_HELD': str(state)}
            result = subprocess.run(['bash', str(Path(sys.argv[3]).resolve()), *sys.argv[4:]], env=env)
            raise SystemExit(result.returncode)
    if len(sys.argv) != 5 or sys.argv[1] not in ('start', 'stop', 'status', 'wait'):
        raise SystemExit('usage: process.py start|stop|status|wait backend|frontend STATE_DIR REPO_ROOT')
    action, role = sys.argv[1:3]
    if role not in ('backend', 'frontend'):
        raise SystemExit('invalid process role')
    state, root = Path(sys.argv[3]).expanduser().resolve(), Path(sys.argv[4]).resolve()
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(state, 0o700)
    record_path, log_path = state / f'{role}.json', state / f'{role}.log'
    record = json.loads(record_path.read_text()) if record_path.exists() else None
    if record and record.get('root') != str(root):
        raise SystemExit(f'{state} 属于其他仓库路径，请使用不同 INVOICE_DEMO_STATE_DIR')

    if action == 'status':
        if owned(record):
            print(f'{role}: running (PID {record["pid"]})')
            return
        print(f'{role}: stopped')
        raise SystemExit(1)

    if action == 'stop':
        if not owned(record):
            if record and identity(record['pid']) is not None:
                raise SystemExit(f'{role}: PID 身份已变化，拒绝停止该进程')
            record_path.unlink(missing_ok=True)
            print(f'{role}: already stopped')
            return
        pid = record['pid']
        try:
            if os.getpgid(pid) != pid:
                raise SystemExit(f'{role}: 进程组不匹配，拒绝停止')
            os.killpg(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        deadline = time.monotonic() + 35
        while owned(record) and time.monotonic() < deadline:
            time.sleep(.2)
        if owned(record):
            # This group was created by this helper and its start time still matches.
            try:
                os.killpg(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        record_path.unlink(missing_ok=True)
        print(f'{role}: stopped')
        return

    if action == 'start':
        if owned(record):
            print(f'{role}: already running (PID {record["pid"]})')
            return
        # Deliberately exclude unrelated DATABASE_*, REDIS_*, CONFIG_FILE and VITE_*.
        env = {key: os.environ[key] for key in
               ('PATH', 'HOME', 'USER', 'LOGNAME', 'TMPDIR', 'LANG', 'LC_CTYPE') if key in os.environ}
        env['TZ'] = 'Asia/Taipei'
        if role == 'backend':
            (state / 'data').mkdir(mode=0o700, exist_ok=True)
            command = [str(state / 'bin' / 'invoice-demo-server')]
            cwd = root / 'backend'
            env.update({
                'AUTO_SETUP': 'true', 'DATA_DIR': str(state / 'data'),
                'CONFIG_FILE': str(state / 'data' / 'config.yaml'),
                'RUN_MODE': 'standard', 'SERVER_HOST': '127.0.0.1', 'SERVER_PORT': '8080',
                'SERVER_MODE': 'debug', 'SERVER_FRONTEND_URL': 'http://127.0.0.1:3000',
                'SERVER_DRAIN_DELAY_SECONDS': '0', 'SERVER_SHUTDOWN_TIMEOUT_SECONDS': '15',
                'SERVER_CLEANUP_TIMEOUT_SECONDS': '10', 'SERVER_FORCE_CLOSE_TIMEOUT_SECONDS': '5',
                'DATABASE_HOST': '127.0.0.1', 'DATABASE_PORT': '55433',
                'DATABASE_USER': 'invoice_demo', 'DATABASE_PASSWORD': 'invoice_demo_local_only',
                'DATABASE_DBNAME': 'invoice_demo', 'DATABASE_SSLMODE': 'disable',
                'DATABASE_MAX_OPEN_CONNS': '20', 'DATABASE_MAX_IDLE_CONNS': '5',
                'DATABASE_READ_REPLICA_ENABLED': 'false',
                'REDIS_HOST': '127.0.0.1', 'REDIS_PORT': '56380', 'REDIS_DB': '0',
                'REDIS_MODE': 'standalone', 'REDIS_CLUSTER_ADDRS': '',
                'REDIS_POOL_SIZE': '20', 'REDIS_MIN_IDLE_CONNS': '2',
                'ADMIN_EMAIL': 'invoice-admin@local.test', 'ADMIN_PASSWORD': 'InvoiceDemo!2026',
                'SETUP_MIGRATION_TIMEOUT_SECONDS': '300',
            })
        else:
            command = ['node', str(root / 'frontend/node_modules/vite/bin/vite.js'),
                       '--config', str(root / 'frontend/vite.config.ts'),
                       '--host', '127.0.0.1', '--port', '3000', '--strictPort']
            cwd = root / 'frontend'
            env.update({'VITE_DEV_PROXY_TARGET': 'http://127.0.0.1:8080',
                        'VITE_DEV_PORT': '3000', 'VITE_API_BASE_URL': '/api/v1'})
        with log_path.open('ab', buffering=0) as output:
            os.chmod(log_path, 0o600)
            child = subprocess.Popen(command, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                     stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        temporary = None
        try:
            started = identity(child.pid)
            if not started:
                raise RuntimeError(f'{role} 启动失败，请查看 {log_path}')
            record = {'pid': child.pid, 'started': started, 'root': str(root), 'command': command}
            with tempfile.NamedTemporaryFile('w', dir=state, prefix=f'{role}-', delete=False) as output:
                temporary = Path(output.name)
                json.dump(record, output, ensure_ascii=False)
                output.flush()
                os.fsync(output.fileno())
            os.replace(temporary, record_path)
        except BaseException:
            if child.poll() is None:
                try:
                    os.killpg(child.pid, signal.SIGTERM)
                    child.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    os.killpg(child.pid, signal.SIGKILL)
                    child.wait()
                except ProcessLookupError:
                    pass
            raise
        finally:
            if temporary:
                temporary.unlink(missing_ok=True)
        print(f'{role}: started (PID {child.pid}), log: {log_path}')
        return

    url = 'http://127.0.0.1:8080/ready' if role == 'backend' else 'http://127.0.0.1:3000/login'
    deadline = time.monotonic() + (300 if role == 'backend' else 90)
    # Never route localhost readiness checks through the user's proxy settings.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    while time.monotonic() < deadline:
        if not owned(record):
            raise SystemExit(f'{role} 已退出，请查看 {log_path}')
        try:
            with opener.open(url, timeout=3) as response:
                if response.status == 200:
                    print(f'{role}: ready ({url})')
                    return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(.5)
    raise SystemExit(f'{role} 启动超时，请查看 {log_path}')


if __name__ == '__main__':
    main()
