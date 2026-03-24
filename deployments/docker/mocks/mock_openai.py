import json
import time
from http.server import BaseHTTPRequestHandler, HTTPServer


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
            self._send(200, {"status": "ok", "service": "mock-openai"})
            return
        if self.path == "/v1/models":
            self._send(
                200,
                {
                    "object": "list",
                    "data": [
                        {"id": "gpt-4", "object": "model", "owned_by": "openai"},
                        {"id": "gpt-3.5-turbo", "object": "model", "owned_by": "openai"},
                        {"id": "claude-3-haiku", "object": "model", "owned_by": "anthropic"},
                    ],
                },
            )
            return
        self._send(404, {"error": {"message": "not found"}})

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self._send(404, {"error": {"message": "not found"}})
            return

        content_length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(content_length) if content_length > 0 else b"{}"
        req = json.loads(body.decode("utf-8"))
        model = req.get("model") or "gpt-4"
        messages = req.get("messages") or []

        user_text = ""
        for msg in messages:
            if msg.get("role") == "user":
                user_text = msg.get("content", "")
                break

        text = f"mock-response: {user_text}" if user_text else "mock-response"

        # Simulate tiny upstream latency.
        time.sleep(0.03)
        self._send(
            200,
            {
                "id": f"chatcmpl-mock-{int(time.time()*1000)}",
                "object": "chat.completion",
                "created": int(time.time()),
                "model": model,
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": text},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {
                    "prompt_tokens": max(1, len(json.dumps(messages)) // 8),
                    "completion_tokens": max(1, len(text) // 8),
                    "total_tokens": max(2, len(json.dumps(messages)) // 8 + len(text) // 8),
                },
            },
        )


def main():
    server = HTTPServer(("0.0.0.0", 8090), Handler)
    print("mock-openai listening on :8090", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
