#!/usr/bin/env python3
"""Dev-only reverse proxy that fakes the Authentik forward-auth headers.

    scripts/dev-proxy.py [listen_port=18081] [upstream=http://127.0.0.1:18080] [uid=42] [username=dev]

Never expose this; it hands out an identity to anyone who connects.
"""
import http.server, sys, urllib.request

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 18081
UP = sys.argv[2] if len(sys.argv) > 2 else "http://127.0.0.1:18080"
UID = sys.argv[3] if len(sys.argv) > 3 else "42"
USER = sys.argv[4] if len(sys.argv) > 4 else "dev"
HOP = {"connection", "keep-alive", "transfer-encoding", "content-length", "host"}

class H(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def _do(self):
        n = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(n) if n else None
        hdrs = {k: v for k, v in self.headers.items() if k.lower() not in HOP}
        hdrs.update({"X-Authentik-Uid": UID, "X-Authentik-Username": USER,
                     "X-Authentik-Groups": "users", "Host": "shit.example"})
        req = urllib.request.Request(UP + self.path, data=body, headers=hdrs, method=self.command)
        try:
            with urllib.request.urlopen(req) as r:
                data = r.read(); self.send_response(r.status)
                for k, v in r.headers.items():
                    if k.lower() not in HOP: self.send_header(k, v)
        except urllib.error.HTTPError as e:
            data = e.read(); self.send_response(e.code)
            for k, v in e.headers.items():
                if k.lower() not in HOP: self.send_header(k, v)
        self.send_header("Content-Length", str(len(data))); self.end_headers(); self.wfile.write(data)
    do_GET = do_POST = do_DELETE = do_PATCH = do_PUT = _do
    def log_message(self, *a): pass

http.server.ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
