#!/usr/bin/env python3
"""HTTP stub for declare-topology-test.sh: records method, path, Authorization,
body of every request as JSON lines (arg: output file). Serves on 127.0.0.1:<port>."""
import http.server, json, sys

port = int(sys.argv[1])
out = sys.argv[2]
open(out, "w").close()

class H(http.server.BaseHTTPRequestHandler):
    def _record(self):
        length = int(self.headers.get("Content-Length", 0) or 0)
        body = self.rfile.read(length).decode("utf-8", "replace") if length else ""
        entry = {"method": self.command, "path": self.path,
                 "auth": self.headers.get("Authorization"), "body": body}
        with open(out, "a") as f:
            f.write(json.dumps(entry) + "\n")
        self.send_response(404 if self.command == "GET" else 201)
        self.end_headers()
    do_GET = do_POST = _record
    def log_message(self, *args, **kwargs): pass

http.server.HTTPServer(("127.0.0.1", port), H).serve_forever()
