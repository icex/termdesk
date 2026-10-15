"""Loopback-only Anthropic response fixture for exercising Claude's Copy UI.

This verifies the installed client UI/protocol, not model behavior. No real
credentials, model calls, user conversation history, or user clipboard is used.
"""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading

COPY_TEXT = 'CLAUDE_COPY_PROBE_7f39'


class AgentAPI:
    def __enter__(self):
        self.requests = []
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                fixture.requests.append(self.path)
                if 'count_tokens' in self.path:
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(b'{"input_tokens": 10}')
                    return
                message = {'id': 'msg_clipboard_fixture', 'type': 'message', 'role': 'assistant',
                           'model': body.get('model', 'claude-test'), 'content': [],
                           'stop_reason': None, 'stop_sequence': None,
                           'usage': {'input_tokens': 10, 'output_tokens': 0}}
                self.send_response(200)
                if body.get('stream'):
                    self.send_header('Content-Type', 'text/event-stream')
                    self.end_headers()
                    events = [
                        ('message_start', {'message': message}),
                        ('content_block_start', {'index': 0, 'content_block': {'type': 'text', 'text': ''}}),
                        ('content_block_delta', {'index': 0, 'delta': {'type': 'text_delta', 'text': COPY_TEXT}}),
                        ('content_block_stop', {'index': 0}),
                        ('message_delta', {'delta': {'stop_reason': 'end_turn', 'stop_sequence': None},
                                           'usage': {'output_tokens': 8}}),
                        ('message_stop', {}),
                    ]
                    for event, data in events:
                        data = dict(type=event, **data)
                        self.wfile.write(f'event: {event}\ndata: {json.dumps(data)}\n\n'.encode())
                    self.wfile.flush()
                else:
                    message.update(content=[{'type': 'text', 'text': COPY_TEXT}], stop_reason='end_turn')
                    self.send_header('Content-Type', 'application/json')
                    self.end_headers()
                    self.wfile.write(json.dumps(message).encode())

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.url = f'http://127.0.0.1:{self.server.server_port}'
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        return self

    def __exit__(self, *args):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)
