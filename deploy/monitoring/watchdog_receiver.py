"""Authenticated local evidence receiver; never sends mail or remote requests.

Run on a separate host/failure domain for production dead-man monitoring. An
external availability monitor should poll /healthz; this process cannot notify
when its own host is unavailable.
"""
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import hmac
import json
import os
from pathlib import Path
import threading
import time


class EvidenceStore:
    def __init__(self, path, timeout=180, clock=time.time):
        self.path = Path(path)
        self.timeout = timeout
        self.clock = clock
        self.started_at = clock()
        self.lock = threading.Lock()
        try:
            self.state = json.loads(self.path.read_text())
        except FileNotFoundError:
            self.state = {"received": 0, "last_watchdog_at": None, "events": []}

    def receive(self, payload):
        alerts = payload.get("alerts") if isinstance(payload, dict) else None
        if not isinstance(alerts, list) or len(alerts) > 1000:
            raise ValueError("alerts must be an array with at most 1000 items")
        events = []
        for alert in alerts:
            if not isinstance(alert, dict) or not isinstance(alert.get("labels"), dict):
                raise ValueError("each alert must contain labels")
            labels = alert["labels"]
            if not all(isinstance(k, str) and isinstance(v, str) for k, v in labels.items()):
                raise ValueError("alert labels must contain strings")
            status = alert.get("status", payload.get("status", "firing"))
            if status not in ("firing", "resolved"):
                raise ValueError("invalid alert status")
            events.append({"labels": labels, "status": status, "received_at": self.clock()})
        with self.lock:
            for event in events:
                if event["labels"].get("alertname") == "ModuRelayMonitoringWatchdog" and event["status"] == "firing":
                    self.state["last_watchdog_at"] = event["received_at"]
            self.state["received"] += len(events)
            self.state["events"] = (self.state["events"] + events)[-100:]
            self.path.parent.mkdir(parents=True, exist_ok=True)
            tmp = self.path.with_suffix(".tmp")
            tmp.write_text(json.dumps(self.state, separators=(",", ":")))
            os.replace(tmp, self.path)
        return len(events)

    def snapshot(self):
        with self.lock:
            state = dict(self.state)
            last = state["last_watchdog_at"]
            elapsed = self.clock() - (last if last is not None else self.started_at)
            state["healthy"] = elapsed <= self.timeout
            state["watchdog_age_seconds"] = max(0, elapsed)
            state["watchdog_seen"] = last is not None
            return state


def handler_for(store, token):
    class Receiver(BaseHTTPRequestHandler):
        def log_message(self, fmt, *args):
            # Never log headers, payloads, or credentials.
            return

        def reply(self, code, body):
            raw = json.dumps(body).encode()
            self.send_response(code)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def authorized(self):
            actual = self.headers.get("Authorization", "")
            return hmac.compare_digest(actual, "Bearer " + token)

        def do_GET(self):
            state = store.snapshot()
            if self.path == "/healthz":
                self.reply(200 if state["healthy"] else 503, {k: state[k] for k in ("healthy", "watchdog_seen", "watchdog_age_seconds")})
            elif self.path == "/events" and self.authorized():
                self.reply(200, state)
            else:
                self.reply(404, {"error": "not found"})

        def do_POST(self):
            if self.path != "/alerts":
                self.reply(404, {"error": "not found"})
                return
            if not self.authorized():
                self.reply(401, {"error": "unauthorized"})
                return
            try:
                length = int(self.headers.get("Content-Length", "0"))
                if not 0 < length <= 1024 * 1024:
                    self.reply(413, {"error": "invalid payload size"})
                    return
                received = store.receive(json.loads(self.rfile.read(length)))
                self.reply(200, {"received": received})
            except (ValueError, UnicodeError):
                self.reply(400, {"error": "invalid alert payload"})

    return Receiver


def main():
    token = Path(os.environ.get("RECEIVER_TOKEN_FILE", "/run/secrets/receiver_token")).read_text().strip()
    if len(token) < 32:
        raise ValueError("receiver token must contain at least 32 characters")
    timeout = int(os.environ.get("WATCHDOG_TIMEOUT_SECONDS", "180"))
    if timeout < 1:
        raise ValueError("watchdog timeout must be positive")
    store = EvidenceStore("/state/evidence.json", timeout)
    server = ThreadingHTTPServer(("0.0.0.0", 8080), handler_for(store, token))
    print("Local monitoring evidence receiver ready", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
