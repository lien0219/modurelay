#!/usr/bin/env python3
"""Exercise the demo supervisor using temporary processes, never real services.

Run with: python3 -B -m unittest discover -s deploy/invoice-demo -p 'test_*.py' -v
"""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock


PROCESS_SCRIPT = Path(__file__).with_name('process.py').resolve()
SPEC = importlib.util.spec_from_file_location('invoice_demo_process', PROCESS_SCRIPT)
supervisor = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(supervisor)


class ProcessLifecycleTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='invoice-demo-process-test-')
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name).resolve()
        self.state, self.root = self.base / 'state', self.base / 'repo'
        (self.state / 'bin').mkdir(parents=True)
        (self.root / 'backend').mkdir(parents=True)
        self.backend = self.state / 'bin' / 'invoice-demo-server'
        self.backend.write_text('#!/bin/sh\nexec /bin/sleep 120\n')
        self.backend.chmod(0o700)
        self.children = []
        self.real_popen = subprocess.Popen
        self.addCleanup(self.cleanup_children)
        self.enterContext(mock.patch.object(supervisor.subprocess, 'Popen', side_effect=self.spawn))

    def spawn(self, command, *args, **kwargs):
        child = self.real_popen(command, *args, **kwargs)
        if isinstance(command, (list, tuple)) and command[0] == str(self.backend):
            self.children.append(child)
        return child

    def cleanup_children(self):
        for child in self.children:
            if child.poll() is None:
                try:
                    if os.getpgid(child.pid) == child.pid:
                        os.killpg(child.pid, signal.SIGKILL)
                    else:
                        child.kill()
                except ProcessLookupError:
                    pass
            child.wait(timeout=5)

    def invoke(self, action):
        output = io.StringIO()
        with mock.patch.object(sys, 'argv', [str(PROCESS_SCRIPT), action, 'backend',
                                             str(self.state), str(self.root)]):
            with contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
                try:
                    supervisor.main()
                except SystemExit as error:
                    if isinstance(error.code, int):
                        return error.code, output.getvalue()
                    return 1, output.getvalue() + str(error.code)
        return 0, output.getvalue()

    def test_start_status_and_repeated_start_stop_manage_one_process(self):
        self.assertEqual(self.invoke('status')[0], 1)
        code, output = self.invoke('start')
        self.assertEqual(code, 0, output)
        record_path = self.state / 'backend.json'
        first_record = record_path.read_bytes()
        record = json.loads(first_record)
        self.assertEqual(len(self.children), 1)
        self.assertEqual(record['pid'], self.children[0].pid)
        self.assertIsNone(self.children[0].poll())
        self.assertEqual(self.invoke('status')[0], 0)

        self.assertEqual(self.invoke('start')[0], 0)
        self.assertEqual(len(self.children), 1, 'Repeated start must not spawn another backend')
        self.assertEqual(record_path.read_bytes(), first_record)

        code, output = self.invoke('stop')
        self.assertEqual(code, 0, output)
        self.children[0].wait(timeout=5)
        self.assertFalse(record_path.exists())
        self.assertEqual(self.invoke('status')[0], 1)
        self.assertEqual(self.invoke('stop')[0], 0)

    def test_stale_pid_identity_does_not_signal_an_unrelated_process(self):
        unrelated = self.real_popen(['/bin/sleep', '120'], start_new_session=True)
        self.children.append(unrelated)
        record_path = self.state / 'backend.json'
        record = {'pid': unrelated.pid, 'started': 'not this process start time',
                  'root': str(self.root)}
        record_path.write_text(json.dumps(record))

        self.assertEqual(self.invoke('status')[0], 1)
        code, output = self.invoke('stop')
        self.assertNotEqual(code, 0, output)
        self.assertIsNone(unrelated.poll(), 'A stale record must not terminate a reused PID')
        self.assertEqual(json.loads(record_path.read_text()), record)

    def test_failed_record_write_reaps_the_new_backend_and_removes_tempfile(self):
        with mock.patch.object(supervisor.os, 'replace', side_effect=OSError('simulated disk error')):
            with self.assertRaisesRegex(OSError, 'simulated disk error'):
                self.invoke('start')

        self.assertEqual(len(self.children), 1)
        self.assertIsNotNone(self.children[0].poll(), 'An unrecorded backend must be terminated')
        self.assertFalse((self.state / 'backend.json').exists())
        self.assertEqual(list(self.state.glob('backend-*')), [])
        self.assertEqual(self.invoke('status')[0], 1)

    def test_parallel_lifecycle_commands_execute_serially(self):
        script, events, gate = self.base / 'locked-command.sh', self.base / 'events', self.base / 'gate'
        script.write_text(
            '#!/bin/bash\nset -eu\n'
            'printf "%s-enter\\n" "$1" >> "$2"\n'
            'if [[ "$1" == first ]]; then\n'
            '  while [[ ! -e "$3" ]]; do sleep 0.02; done\n'
            'fi\n'
            'printf "%s-leave\\n" "$1" >> "$2"\n'
        )
        commands = []
        try:
            for label in ('first', 'second'):
                child = self.real_popen(
                    [sys.executable, str(PROCESS_SCRIPT), 'lock', str(self.state),
                     str(script), label, str(events), str(gate)],
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                )
                commands.append(child)
                if label == 'first':
                    deadline = time.monotonic() + 5
                    while (not events.exists() or 'first-enter' not in events.read_text()):
                        if child.poll() is not None or time.monotonic() >= deadline:
                            self.fail('The first lock holder did not enter its command')
                        time.sleep(0.02)

            second = commands[1]
            readable, _, _ = select.select([second.stdout], [], [], 5)
            self.assertTrue(readable, 'The second command did not reach the lock')
            self.assertIn('等待完成', second.stdout.readline())
            self.assertIsNone(second.poll())
            self.assertEqual(events.read_text().splitlines(), ['first-enter'])

            gate.touch()
            for child in commands:
                output, errors = child.communicate(timeout=5)
                self.assertEqual(child.returncode, 0, output + errors)
            self.assertEqual(events.read_text().splitlines(),
                             ['first-enter', 'first-leave', 'second-enter', 'second-leave'])
        finally:
            # Release the temporary shell even when an assertion fails; never
            # signal the real demo or any process read from its state files.
            gate.touch()
            for child in commands:
                try:
                    child.communicate(timeout=5)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.communicate(timeout=5)


if __name__ == '__main__':
    unittest.main()
