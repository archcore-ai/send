#!/usr/bin/env python3
"""In-memory mock of the Send v1 HTTP API (backend-http-api.spec) for skill e2e tests.

NOT the production server — just enough of the contract to drive send.sh / send.ps1
round-trips: create -> upload -> finalize -> redeem -> download, with atomic
one-time redemption, sha256 verification, and a request log for security assertions.

Usage:
    python3 mock_sendd.py [--port 0] [--log /path/to/requests.log]
Prints the base URL (http://127.0.0.1:PORT) on stdout once listening, then serves
until terminated.
"""
import argparse
import hashlib
import json
import os
import secrets
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

LOCK = threading.Lock()
SENDS = {}  # id -> dict(version, one_time, expires_at, status, parts{tid: {size,sha,bytes}}, consumed)
GRANTS = {}  # token -> dict(send_id, expires)
LOG_PATH = None


def log_request(method, path, query, auth):
    if not LOG_PATH:
        return
    with open(LOG_PATH, "a", encoding="utf-8") as fh:
        fh.write(f"{method} {path} q={query} auth={auth}\n")


def now():
    return int(time.time())


def iso(ts):
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(ts))


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    # Quiet the default stderr logging; we keep our own structured log.
    def log_message(self, *args):
        pass

    def _send(self, code, obj=None, raw=None, ctype="application/json"):
        if raw is not None:
            body = raw
        else:
            body = json.dumps(obj or {}).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _err(self, code, error_code, msg=""):
        self._send(code, {"error_code": error_code, "message": msg})

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        return self.rfile.read(length) if length else b""

    def _parts(self):
        # path: /v1/sends/{id}/parts/{part_id}
        segs = self.path.split("?")[0].strip("/").split("/")
        return segs

    def do_GET(self):
        path = self.path.split("?")[0]
        query = self.path.split("?")[1] if "?" in self.path else ""
        log_request("GET", path, query, self.headers.get("Authorization", ""))

        if path == "/healthz":
            return self._send(200, {"ok": True})

        segs = path.strip("/").split("/")
        # GET /v1/sends/{id}
        if len(segs) == 3 and segs[0] == "v1" and segs[1] == "sends":
            sid = segs[2]
            with LOCK:
                s = SENDS.get(sid)
                if not s:
                    return self._err(404, "SEND_NOT_FOUND")
                parts = [{"part_id": t, "encrypted_size": p["size"], "sha256": p["sha"]}
                         for t, p in s["parts"].items()]
                total = sum(p["size"] for p in s["parts"].values())
                return self._send(200, {
                    "id": sid, "status": s["status"], "one_time": s["one_time"],
                    "expires_at": iso(s["expires_at"]), "part_count": len(parts),
                    "total_encrypted_size": total, "parts": parts,
                })

        # GET /v1/sends/{id}/parts/{part_id}  (requires grant)
        if len(segs) == 5 and segs[3] == "parts":
            sid, tid = segs[2], segs[4]
            token = self._bearer() or self._query_token(query)
            with LOCK:
                g = GRANTS.get(token)
                if not g or g["send_id"] != sid or g["expires"] < now():
                    return self._err(403, "INVALID_REDEEM")
                s = SENDS.get(sid)
                if not s or tid not in s["parts"]:
                    return self._err(404, "SEND_NOT_FOUND")
                return self._send(200, raw=s["parts"][tid]["bytes"],
                                  ctype="application/octet-stream")

        return self._err(404, "SEND_NOT_FOUND")

    def do_POST(self):
        path = self.path.split("?")[0]
        log_request("POST", path, "", self.headers.get("Authorization", ""))
        body = self._read_body()
        segs = path.strip("/").split("/")

        # POST /v1/sends
        if path == "/v1/sends":
            try:
                req = json.loads(body or b"{}")
            except json.JSONDecodeError:
                return self._err(400, "BAD_REQUEST", "invalid json")
            ttl = int(req.get("ttl_seconds", 86400))
            if ttl > 7 * 24 * 3600:
                return self._err(400, "BAD_REQUEST", "ttl too large")
            sid = "snd_" + secrets.token_hex(12)
            parts = {}
            for p in req.get("parts", []):
                parts[p["part_id"]] = {"size": int(p["encrypted_size"]),
                                       "sha": p["sha256"], "bytes": None}
            with LOCK:
                SENDS[sid] = {
                    "version": req.get("version"), "one_time": bool(req.get("one_time", True)),
                    "expires_at": now() + ttl, "status": "creating",
                    "parts": parts, "consumed": False,
                }
            host = self.headers.get("Host", "127.0.0.1")
            upload_urls = {t: f"/v1/sends/{sid}/parts/{t}" for t in parts}
            return self._send(201, {
                "id": sid, "upload_urls": upload_urls,
                "public_url": f"http://{host}/s/{sid}",
                "expires_at": iso(SENDS[sid]["expires_at"]),
                "one_time": SENDS[sid]["one_time"],
            })

        # POST /v1/sends/{id}/finalize
        if len(segs) == 4 and segs[3] == "finalize":
            sid = segs[2]
            with LOCK:
                s = SENDS.get(sid)
                if not s:
                    return self._err(404, "SEND_NOT_FOUND")
                missing = [t for t, p in s["parts"].items() if p["bytes"] is None]
                if missing:
                    return self._err(400, "INCOMPLETE", f"missing {missing}")
                s["status"] = "finalized"
                return self._send(200, {"ok": True, "public_url": f"/s/{sid}",
                                        "expires_at": iso(s["expires_at"])})

        # POST /v1/sends/{id}/redeem  — atomic one-time consume
        if len(segs) == 4 and segs[3] == "redeem":
            sid = segs[2]
            with LOCK:
                s = SENDS.get(sid)
                if not s:
                    return self._err(404, "SEND_NOT_FOUND")
                if s["status"] != "finalized" and not s["consumed"]:
                    return self._err(409, "SEND_FINALIZED", "not finalized")
                if s["expires_at"] < now():
                    return self._err(410, "SEND_EXPIRED")
                if s["one_time"] and s["consumed"]:
                    return self._err(410, "SEND_ALREADY_REDEEMED")
                if s["one_time"]:
                    s["consumed"] = True
                token = "red_" + secrets.token_hex(16)
                GRANTS[token] = {"send_id": sid, "expires": now() + 600}
                parts = [{"part_id": t, "encrypted_size": p["size"]}
                         for t, p in s["parts"].items()]
                return self._send(200, {"redeem_token": token,
                                        "expires_at": iso(now() + 600), "parts": parts})

        return self._err(404, "SEND_NOT_FOUND")

    def do_PUT(self):
        path = self.path.split("?")[0]
        log_request("PUT", path, "", self.headers.get("Authorization", ""))
        segs = path.strip("/").split("/")
        if len(segs) == 5 and segs[3] == "parts":
            sid, tid = segs[2], segs[4]
            body = self._read_body()
            declared = self.headers.get("X-Send-Ciphertext-Sha256", "")
            actual = hashlib.sha256(body).hexdigest()
            with LOCK:
                s = SENDS.get(sid)
                if not s:
                    return self._err(404, "SEND_NOT_FOUND")
                if s["status"] != "creating":
                    return self._err(409, "SEND_FINALIZED")
                if tid not in s["parts"]:
                    return self._err(400, "BAD_REQUEST", "unknown part")
                if declared and declared != actual:
                    return self._err(422, "INTEGRITY_FAILED")
                if s["parts"][tid]["size"] != len(body):
                    return self._err(422, "INTEGRITY_FAILED", "size mismatch")
                s["parts"][tid]["bytes"] = body
                return self._send(200, {"ok": True, "part_id": tid,
                                        "encrypted_size": len(body)})
        return self._err(404, "SEND_NOT_FOUND")

    def _bearer(self):
        h = self.headers.get("Authorization", "")
        return h[7:] if h.startswith("Bearer ") else ""

    def _query_token(self, query):
        for kv in query.split("&"):
            if kv.startswith("redeem_token="):
                return kv[len("redeem_token="):]
        return ""


def main():
    global LOG_PATH
    ap = argparse.ArgumentParser()
    ap.add_argument("--port", type=int, default=0)
    ap.add_argument("--log", default=os.environ.get("SEND_MOCK_LOG"))
    args = ap.parse_args()
    LOG_PATH = args.log

    httpd = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    port = httpd.server_address[1]
    sys.stdout.write(f"http://127.0.0.1:{port}\n")
    sys.stdout.flush()
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
