#!/usr/bin/env python3
import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def text_from_message(message):
    content = message.get("content", "")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for part in content:
            if isinstance(part, dict) and part.get("type") == "text":
                parts.append(part.get("text", ""))
        return "\n".join(parts)
    return ""


def json_response(model, message, finish_reason="stop"):
    return {
        "id": "chatcmpl_e2e",
        "object": "chat.completion",
        "created": 0,
        "model": model,
        "choices": [
            {
                "index": 0,
                "message": message,
                "finish_reason": finish_reason,
            }
        ],
    }


def tool_call_response(model, tool_call_id, name, arguments):
    return json_response(
        model,
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [
                {
                    "id": tool_call_id,
                    "type": "function",
                    "function": {
                        "name": name,
                        "arguments": json.dumps(arguments, ensure_ascii=True),
                    },
                }
            ],
        },
        finish_reason="tool_calls",
    )


class Handler(BaseHTTPRequestHandler):
    server_version = "FakeOpenAIServer/1.0"

    def do_GET(self):
        if self.path == "/healthz":
            self.send_json(200, {"ok": True})
            return
        if self.path == "/v1/models":
            self.send_json(200, {"object": "list", "data": [{"id": "gpt-5.1", "object": "model"}]})
            return
        self.send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self.send_json(404, {"error": "not found"})
            return

        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        body = json.loads(raw or b"{}")

        model = body.get("model", "gpt-5.1")
        messages = body.get("messages", [])
        last_message = messages[-1] if messages else {}
        last_role = last_message.get("role", "")
        last_text = text_from_message(last_message)
        user_texts = [text_from_message(msg) for msg in messages if msg.get("role") == "user"]

        if last_role == "tool":
            tool_result = last_text
            tool_call_id = last_message.get("tool_call_id", "")
            if tool_call_id == "call_write_note":
                self.send_chat_response(body, model, {"role": "assistant", "content": "DONE_WRITE"})
                return
            if tool_call_id == "call_read_note":
                content = "READ_OK" if "hello-from-e2e" in tool_result else "READ_BAD"
                self.send_chat_response(body, model, {"role": "assistant", "content": content})
                return

        if "Reply with exactly E2E_PONG." in last_text:
            self.send_chat_response(body, model, {"role": "assistant", "content": "E2E_PONG"})
            return

        if "Use write_file to create note.txt with content hello-from-e2e." in last_text:
            self.send_chat_response(body, model, {
                "role": "assistant",
                "content": "",
                "tool_calls": [
                    {
                        "id": "call_write_note",
                        "type": "function",
                        "function": {
                            "name": "write_file",
                            "arguments": json.dumps({
                                "path": "note.txt",
                                "content": "hello-from-e2e",
                            }, ensure_ascii=True),
                        },
                    }
                ],
            }, finish_reason="tool_calls")
            return

        if "Use read_file to read note.txt." in last_text:
            self.send_chat_response(body, model, {
                "role": "assistant",
                "content": "",
                "tool_calls": [
                    {
                        "id": "call_read_note",
                        "type": "function",
                        "function": {
                            "name": "read_file",
                            "arguments": json.dumps({
                                "path": "note.txt",
                            }, ensure_ascii=True),
                        },
                    }
                ],
            }, finish_reason="tool_calls")
            return

        if "Remember that my codename is ORANGE_E2E." in last_text:
            self.send_chat_response(body, model, {"role": "assistant", "content": "ACK_ORANGE"})
            return

        if "What is my codename? Reply with exactly ORANGE_E2E." in last_text:
            remembered = any("Remember that my codename is ORANGE_E2E." in text for text in user_texts[:-1])
            content = "ORANGE_E2E" if remembered else "UNKNOWN"
            self.send_chat_response(body, model, {"role": "assistant", "content": content})
            return

        self.send_chat_response(body, model, {"role": "assistant", "content": "UNHANDLED_PROMPT"})

    def send_chat_response(self, body, model, message, finish_reason="stop"):
        if body.get("stream"):
            self.send_sse_response(model, message, finish_reason)
            return
        self.send_json(200, json_response(model, message, finish_reason))

    def send_sse_response(self, model, message, finish_reason):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()

        if message.get("tool_calls"):
            tool_call = message["tool_calls"][0]
            first_chunk = {
                "id": "chatcmpl_e2e_chunk",
                "object": "chat.completion.chunk",
                "created": 0,
                "model": model,
                "choices": [
                    {
                        "index": 0,
                        "delta": {
                            "role": "assistant",
                            "tool_calls": [
                                {
                                    "index": 0,
                                    "id": tool_call["id"],
                                    "type": tool_call["type"],
                                    "function": tool_call["function"],
                                }
                            ],
                        },
                        "finish_reason": None,
                    }
                ],
            }
            final_chunk = {
                "id": "chatcmpl_e2e_chunk",
                "object": "chat.completion.chunk",
                "created": 0,
                "model": model,
                "choices": [
                    {
                        "index": 0,
                        "delta": {},
                        "finish_reason": finish_reason,
                    }
                ],
            }
            self.write_sse(first_chunk)
            self.write_sse(final_chunk)
            self.wfile.write(b"data: [DONE]\n\n")
            self.wfile.flush()
            return

        content = message.get("content", "")
        first_chunk = {
            "id": "chatcmpl_e2e_chunk",
            "object": "chat.completion.chunk",
            "created": 0,
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "delta": {
                        "role": "assistant",
                        "content": content,
                    },
                    "finish_reason": None,
                }
            ],
        }
        final_chunk = {
            "id": "chatcmpl_e2e_chunk",
            "object": "chat.completion.chunk",
            "created": 0,
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "delta": {},
                    "finish_reason": finish_reason,
                }
            ],
        }
        self.write_sse(first_chunk)
        self.write_sse(final_chunk)
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()

    def write_sse(self, payload):
        self.wfile.write(f"data: {json.dumps(payload)}\n\n".encode("utf-8"))
        self.wfile.flush()

    def log_message(self, format, *args):
        return

    def send_json(self, status, payload):
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    args = parser.parse_args()

    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
