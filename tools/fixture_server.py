#!/usr/bin/env python3
import json
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler

class FixtureHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # Quiet logging

    def do_POST(self):
        if self.path != "/v2/query":
            self.send_response(404)
            self.end_headers()
            return

        auth = self.headers.get("Authorization", "")
        if not auth:
            self.send_response(401)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"message": "Unauthorized"}).encode("utf-8"))
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body_bytes = self.rfile.read(content_length)
        body = json.loads(body_bytes.decode("utf-8"))

        sql = body.get("sql", "")
        if "SELECT ERROR" in sql:
            self.send_response(400)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"message": "Syntax error in SQL"}).encode("utf-8"))
            return

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        res = {
            "schema": {
                "fields": [{"name": "message", "datatype": "Utf8", "nullable": True}]
            },
            "data": [{"message": "hello from fixture"}]
        }
        self.wfile.write(json.dumps(res).encode("utf-8"))

def run_server(port=8901):
    server = HTTPServer(("127.0.0.1", port), FixtureHandler)
    print(f"Fixture server running on http://127.0.0.1:{port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()

if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8901
    run_server(port)
