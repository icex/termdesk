"""Loopback-only response fixtures for exercising installed agents' Copy UIs.

This verifies the installed client UI/protocol, not model behavior. No real
credentials, model calls, user conversation history, or user clipboard is used.
"""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading

COPY_TEXT = 'CLAUDE_COPY_PROBE_7f39'
CODEX_COPY_TEXT = 'CODEX_COPY_PROBE_8b41'


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
                if self.path.rstrip('/').endswith('/responses'):
                    item = {'id': 'msg_clipboard_fixture', 'type': 'message', 'role': 'assistant',
                            'status': 'completed', 'content': [{'type': 'output_text',
                            'text': CODEX_COPY_TEXT, 'annotations': []}]}
                    response = {'id': 'resp_clipboard_fixture', 'object': 'response',
                                'created_at': 1, 'status': 'completed',
                                'model': body.get('model', 'codex-test'), 'output': [item],
                                'usage': {'input_tokens': 10, 'output_tokens': 8, 'total_tokens': 18}}
                    events = [
                        ('response.created', {'response': dict(response, status='in_progress', output=[])}),
                        ('response.output_item.added', {'output_index': 0,
                            'item': dict(item, status='in_progress', content=[])}),
                        ('response.content_part.added', {'output_index': 0, 'item_id': item['id'],
                            'content_index': 0, 'part': {'type': 'output_text', 'text': '', 'annotations': []}}),
                        ('response.output_text.delta', {'output_index': 0, 'item_id': item['id'],
                            'content_index': 0, 'delta': CODEX_COPY_TEXT}),
                        ('response.output_item.done', {'output_index': 0, 'item': item}),
                        ('response.completed', {'response': response}),
                    ]
                    self.send_response(200)
                    self.send_header('Content-Type', 'text/event-stream')
                    self.end_headers()
                    for sequence, (event, data) in enumerate(events):
                        data = dict(type=event, sequence_number=sequence, **data)
                        self.wfile.write(f'event: {event}\ndata: {json.dumps(data)}\n\n'.encode())
                    self.wfile.flush()
                    return
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
