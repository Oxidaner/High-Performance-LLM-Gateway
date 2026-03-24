import json
from http.server import BaseHTTPRequestHandler, HTTPServer


def make_embedding(text):
    base = float((sum(ord(ch) for ch in text) % 997) + 1)
    return [round(((base + i * 13) % 1000) / 1000.0, 6) for i in range(16)]


class Handler(BaseHTTPRequestHandler):
    def _send(self, status, payload):
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path in ("/health", "/healthz"):
            self._send(200, {"status": "ok", "service": "mock-embedding-worker"})
            return
        self._send(404, {"error": {"message": "not found"}})

    def do_POST(self):
        if self.path != "/embeddings":
            self._send(404, {"error": {"message": "not found"}})
            return

        content_length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(content_length) if content_length > 0 else b"{}"
        req = json.loads(body.decode("utf-8"))

        model = req.get("model") or "text-embedding-ada-002"
        input_data = req.get("input", "")
        if isinstance(input_data, list):
            texts = [str(x) for x in input_data]
        else:
            texts = [str(input_data)]

        data = []
        total_tokens = 0
        for idx, text in enumerate(texts):
            total_tokens += max(1, len(text) // 4)
            data.append(
                {
                    "object": "embedding",
                    "embedding": make_embedding(text),
                    "index": idx,
                }
            )

        self._send(
            200,
            {
                "object": "list",
                "data": data,
                "model": model,
                "usage": {
                    "prompt_tokens": total_tokens,
                    "completion_tokens": 0,
                    "total_tokens": total_tokens,
                },
            },
        )


def main():
    server = HTTPServer(("0.0.0.0", 8081), Handler)
    print("mock-embedding-worker listening on :8081", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
