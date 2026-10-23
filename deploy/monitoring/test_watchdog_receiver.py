import json
from pathlib import Path
import tempfile
import threading
import unittest
from http.server import ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.request import Request, urlopen
from watchdog_receiver import EvidenceStore, handler_for


class ReceiverTest(unittest.TestCase):
    def test_watchdog_survives_restart_then_expires(self):
        with tempfile.TemporaryDirectory() as temp:
            now = [1000.0]
            path = Path(temp) / 'evidence.json'
            store = EvidenceStore(path, 180, lambda: now[0])
            store.receive({'alerts': [{'labels': {'alertname': 'ModuRelayMonitoringWatchdog'}, 'status': 'firing'}]})
            restarted = EvidenceStore(path, 180, lambda: now[0])
            self.assertTrue(restarted.snapshot()['healthy'])
            self.assertEqual(restarted.snapshot()['received'], 1)
            now[0] += 181
            self.assertFalse(restarted.snapshot()['healthy'])
            restarted.receive({'alerts': [{'labels': {'alertname': 'OtherFault'}, 'status': 'firing'}]})
            self.assertFalse(restarted.snapshot()['healthy'], 'other alerts cannot refresh the watchdog')
            restarted.receive({'alerts': [{'labels': {'alertname': 'ModuRelayMonitoringWatchdog'}, 'status': 'resolved'}]})
            self.assertFalse(restarted.snapshot()['healthy'], 'resolved watchdog cannot mask collection loss')

    def test_authorization_and_bad_payload_do_not_refresh_heartbeat(self):
        with tempfile.TemporaryDirectory() as temp:
            store = EvidenceStore(Path(temp) / 'state.json')
            token = 'unit-test-secret-' * 3
            server = ThreadingHTTPServer(('127.0.0.1', 0), handler_for(store, token))
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                url = 'http://127.0.0.1:%d/alerts' % server.server_address[1]
                body = json.dumps({'alerts': [{'labels': {'alertname': 'ModuRelayMonitoringWatchdog'}}]}).encode()
                with self.assertRaises(HTTPError) as error:
                    urlopen(Request(url, data=body), timeout=3)
                self.assertEqual(error.exception.code, 401)
                error.exception.close()
                self.assertFalse(store.snapshot()['watchdog_seen'])
                with urlopen(Request(url, data=body, headers={'Authorization': 'Bearer ' + token}), timeout=3) as response:
                    self.assertEqual(response.status, 200)
                self.assertTrue(store.snapshot()['watchdog_seen'])
                with self.assertRaises(ValueError):
                    store.receive({'alerts': [{'labels': {'alertname': 12}}]})
                self.assertEqual(store.snapshot()['received'], 1)
            finally:
                server.shutdown()
                server.server_close()
                thread.join()


if __name__ == '__main__':
    unittest.main()
