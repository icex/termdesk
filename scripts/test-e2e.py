#!/usr/bin/env python3
"""Exercise a freshly built Termdesk through isolated Unix sockets and PTYs.

Never reads, lists, attaches to, or signals the user's real Termdesk sessions.
All configuration, trust records, logs, buffers and sockets live in a temporary
root. Only processes launched by this script are terminated during cleanup.
"""
import base64
import json
import shutil
import os
from pathlib import Path
import pty
import select
import signal
import socket
import struct
import subprocess
import tempfile
import time

REPO = Path(__file__).resolve().parents[1]


def wait_for(check, description, timeout=4):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        if check():
            return
        time.sleep(0.025)
    raise AssertionError(description)


class Session:
    def __init__(self, binary, root, name, cwd, project_dir=None, **config):
        self.root, self.binary, self.name = root, binary, name
        self.env = os.environ.copy()
        self.env.update(HOME=str(root), XDG_CONFIG_HOME=str(root / 'config'),
                        XDG_CACHE_HOME=str(root / 'cache'),
                        XDG_RUNTIME_DIR=str(root), TERMDESK_HOME=str(root),
                        TERMDESK_CONFIG_PATH=str(root / 'config.toml'),
                        TERMDESK_LOG_LEVEL='off', TERMDESK_GRAPHICS='kitty',
                        SHELL='/bin/bash', TERM='xterm-256color')
        self.env.pop('TERMDESK', None)
        self.env.pop('TERMDESK_DEBUG', None)
        self.env.pop('TERMDESK_PROJECT_DIR', None)
        if project_dir is not None:
            self.env['TERMDESK_PROJECT_DIR'] = str(project_dir)
        options = {'tour_completed': True, 'animations': False,
                   'enabled_widgets': 'clock'}
        options.update(config)
        (root / 'config.toml').write_text(''.join(
            f'{key} = {json.dumps(value)}\n' for key, value in options.items()))
        self.server = subprocess.Popen([binary, '--server', name], cwd=cwd,
            env=self.env, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE, start_new_session=True)
        self.path = root / 'termdesk' / f'{name}.sock'
        wait_for(lambda: self.path.exists(), 'isolated server did not start')
        self.conn = socket.socket(socket.AF_UNIX)
        self.conn.connect(str(self.path))
        self.closed = False
        self.pending = b''
        self.send('R', struct.pack('>HH', 30, 100))
        self.drain(0.5)

    def send(self, typ, payload):
        self.conn.sendall(typ.encode() + struct.pack('>I', len(payload)) + payload)

    def keys(self, payload):
        self.send('I', payload)

    def drain(self, duration=0.15):
        end = time.monotonic() + duration
        output = b''
        while time.monotonic() < end:
            if select.select([self.conn], [], [], min(0.03, max(0, end-time.monotonic())))[0]:
                data = self.conn.recv(1 << 20)
                if not data:
                    self.closed = True
                    break
                self.pending += data
                while len(self.pending) >= 5:
                    length = struct.unpack('>I', self.pending[1:5])[0]
                    if len(self.pending) < 5 + length:
                        break
                    typ = self.pending[:1]
                    if typ in (b'O', b'S'):
                        output += self.pending[5:5+length]
                    self.pending = self.pending[5+length:]
        return output

    def command(self, command):
        self.keys(b'\x15' + command.encode() + b'\r')
        return self.drain(0.25)

    def open_shell(self):
        self.keys(b'\x0e')
        self.drain(0.3)
        marker = self.root / 'shell-ready'
        self.command(f"printf ready > '{marker}'")
        wait_for(marker.exists, 'shell did not accept input')

    def cli(self, *args):
        return subprocess.run([self.binary, *args], env=self.env, cwd=self.root,
                              capture_output=True, timeout=4, check=True)

    def detach(self):
        self.keys(b'\x01d')
        wait_for(lambda: bool(self.drain(0.05)) or self.closed,
                 'detach did not produce output')
        wait_for(lambda: (self.drain(0.05), self.closed)[1], 'client did not detach')
        assert self.server.poll() is None, 'detach ended the server'

    def close(self):
        self.conn.close()
        if self.server.poll() is None:
            self.server.terminate()
        try:
            self.server.wait(timeout=4)
        except subprocess.TimeoutExpired:
            self.server.kill()
            self.server.wait()
            raise AssertionError('isolated server did not stop cleanly')
        self.server.stderr.close()


def check_attach_without_terminal_reply(session):
    # pty.fork creates a new controlling terminal; /dev/tty is never the caller's.
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(session.root)
        os.execve(session.binary, [session.binary, 'attach', session.name], session.env)
    try:
        response = b''
        end = time.monotonic() + 2
        while time.monotonic() < end:
            if select.select([fd], [], [], 0.05)[0]:
                response += os.read(fd, 65536)
                if len(response) > 100:
                    break
        assert b'2026$p' in response and len(response) > 100, 'attach blocked on unanswered probe'
        os.write(fd, b'\x01d')
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            if select.select([fd], [], [], 0.05)[0]:
                try:
                    os.read(fd, 65536)
                except OSError:
                    break
            waited, _ = os.waitpid(pid, os.WNOHANG)
            if waited:
                pid = None
                break
        while pid and time.monotonic() < deadline:
            waited, _ = os.waitpid(pid, os.WNOHANG)
            if waited:
                pid = None
                break
            time.sleep(0.01)
        assert pid is None, 'attached client did not detach cleanly'
    finally:
        if pid:
            os.kill(pid, signal.SIGTERM)
            os.waitpid(pid, 0)
        os.close(fd)


def main():
    results = []
    with tempfile.TemporaryDirectory(prefix='td-e2e-', dir='/tmp') as temp:
        root = Path(temp)
        binary = str(root / 'td-bin')
        subprocess.run(['go', 'build', '-o', binary, './cmd/termdesk'], cwd=REPO, check=True)
        live = root / 'live'; live.mkdir()
        s = Session(binary, live, 'review', live)
        try:
            s.open_shell()
            for _ in range(3):
                assert 'review' in s.cli('ls').stdout.decode()
                s.drain()
                assert not s.closed, 'listing disconnected active client'
            results.append('listing preserves attached client')

            marker = live / 'paste-executed'
            s.keys(b'\x1b[200~' + f"printf executed > '{marker}'\n".encode() + b'\x1b[201~')
            s.drain(0.3)
            assert not marker.exists(), 'paste executed before Enter'
            s.keys(b'\r'); s.drain(0.2)
            wait_for(marker.exists, 'protected paste did not execute after Enter')
            results.append('multiline paste waits for Enter')

            # Terminal output must neither read nor delete this ordinary file.
            victim = live / 'ordinary-file'; victim.write_text('keep this marker')
            encoded = base64.b64encode(str(victim).encode()).decode()
            s.command(f"printf '\\033_Ga=T,t=t,f=100,i=456;{encoded}\\033\\134'")
            assert victim.read_text() == 'keep this marker'
            results.append('graphics output cannot delete local files')

            # Direct transmission still travels through the real app/server path.
            direct = s.command("printf '\\033_Ga=t,i=42,f=100;AAAA\\033\\134\\033_Ga=p,i=42,c=1,r=1\\033\\134'")
            assert b'\x1b_Ga=t,i=' in direct and b'\x1b_Ga=p,i=' in direct, 'direct graphics lost'
            results.append('direct graphics transmit/place remains functional')

            # Negotiated filesystem rejection falls back to direct; the DA1
            # sentinel must arrive after both graphics replies.
            query_script = live / 'graphics-query.py'
            reply_file = live / 'graphics-reply'
            query_script.write_text("import os,tty,termios\n"
                "old=termios.tcgetattr(0);tty.setraw(0)\n"
                "os.write(1,b'\\x1b_Ga=q,t=f,i=7;\\x1b\\\\\\x1b_Ga=q,t=d,i=8,s=1,v=1;AAAA\\x1b\\\\\\x1b[c')\n"
                "data=b''\n"
                "while not data.endswith(b'c'): data+=os.read(0,4096)\n"
                "termios.tcsetattr(0,termios.TCSANOW,old)\n"
                "open(" + repr(str(reply_file)) + ", 'wb').write(data)\n")
            s.command(f"python3 '{query_script}'")
            wait_for(reply_file.exists, 'graphics query negotiation did not complete')
            reply = reply_file.read_bytes()
            assert b'i=7;ENOTSUPPORTED' in reply and b'i=8;OK' in reply
            assert reply.index(b'i=7;') < reply.index(b'i=8;') < reply.index(b'\x1b[')
            results.append('graphics negotiation replies precede DA1 sentinel')

            # Split, type into the new pane, then return to a single pane.
            s.keys(b'\x01%'); s.drain(0.25)
            split_marker = live / 'split-input'
            s.command(f"printf split > '{split_marker}'")
            wait_for(split_marker.exists, 'new split did not receive input')
            s.keys(b'\x01x'); s.drain(0.2)
            results.append('split input and close work through PTY')

            # Save this project workspace despite there being no project config.
            # Open a session in such a project using an explicitly created workspace.
            s.keys(b'\x01\x13');s.drain(0.4)
            global_ws = live / '.config' / 'termdesk' / 'workspace.toml'
            wait_for(global_ws.exists, 'workspace save did not complete')
            assert global_ws.stat().st_mode & 0o077 == 0, 'workspace exposes clipboard/history'
            buffers = list((live / '.config' / 'termdesk' / 'buffers').glob('*.buf.gz'))
            assert buffers and all(p.stat().st_mode & 0o077 == 0 for p in buffers)
            results.append('workspace and terminal buffers persist privately')

            s.detach()
            check_attach_without_terminal_reply(s)
            results.append('detach/reattach works without terminal probe replies')
        finally:
            s.close()

        # An inspected project config may autostart; an unapproved one may not.
        project = root / 'project'; project.mkdir()
        config = project / '.termdesk.toml'
        marker = project / 'autostart-marker'
        config.write_text('[[autostart]]\ncommand = "/bin/sh"\n'
                          f'args = ["-c", "printf trusted > {marker}"]\n')
        trust_root = root / 'trust-session'; trust_root.mkdir()
        s = Session(binary, trust_root, 'untrusted', project)
        try:
            s.drain(0.4)
            assert not marker.exists(), 'untrusted config executed'
            s.cli('trust', str(config))
        finally:
            s.close()
        s = Session(binary, trust_root, 'trusted', project)
        try:
            wait_for(marker.exists, 'approved config did not autostart')
        finally:
            s.close()
        marker.unlink()
        config.write_text(config.read_text() + '# changed contents\n')
        s = Session(binary, trust_root, 'changed', project)
        try:
            s.drain(0.4)
            assert not marker.exists(), 'changed config inherited trust'
        finally:
            s.close()
        results.append('project autostart requires unchanged approved contents')

        # Explicit project sessions persist locally even without .termdesk.toml.
        bare_project = root / 'bare-project'; bare_project.mkdir()
        bare_root = root / 'bare-session'; bare_root.mkdir()
        s = Session(binary, bare_root, 'bare', bare_project, project_dir=bare_project)
        try:
            s.open_shell(); s.keys(b'\x01\x13'); s.drain(0.3)
            local = bare_project / '.termdesk-workspace.toml'
            wait_for(local.exists, 'project without config saved to wrong location')
            assert not (bare_root / '.config' / 'termdesk' / 'workspace.toml').exists()
        finally:
            s.close()
        results.append('project without config saves to its own workspace')

        # Exercise a real editor when available; capture must leave unsaved text alone.
        editor = shutil.which('nvim')
        if editor:
            editor_project = root / 'editor-project'; editor_project.mkdir()
            document = editor_project / 'document.txt'; document.write_text('original\n')
            editor_config = editor_project / '.termdesk.toml'
            editor_config.write_text('[[autostart]]\ncommand = ' + json.dumps(editor)
                                     + '\nargs = ["--clean", ' + json.dumps(str(document)) + ']\n')
            editor_root = root / 'editor-session'; editor_root.mkdir()
            s = Session(binary, editor_root, 'approve-editor', editor_project)
            try:
                s.cli('trust', str(editor_config))
            finally:
                s.close()
            s = Session(binary, editor_root, 'editor', editor_project)
            try:
                s.drain(0.4); s.keys(b'iUNSAVED '); s.drain(0.2)
                s.keys(b'\x01\x13'); s.drain(0.3)
                assert document.read_text() == 'original\n', 'capture wrote editor document'
                # The save action returns to window-management mode.
                s.keys(b'i'); s.drain(0.1)
                s.keys(b'\x1b'); s.drain(0.15)
                s.keys(b':wq\r'); s.drain(0.3)
                wait_for(lambda: document.read_text().startswith('UNSAVED '),
                         'explicit editor save did not work')
            finally:
                s.close()
            results.append('real Neovim capture preserves unsaved edits')
        else:
            print('SKIP: real Neovim check (nvim unavailable)', flush=True)

        # A copied workspace is untrusted, then becomes restorable through CLI trust.
        ws_root = root / 'restore-session'; ws_root.mkdir()
        source = live / '.config' / 'termdesk' / 'workspace.toml'
        wsdir = ws_root / '.config' / 'termdesk'; wsdir.mkdir(parents=True)
        target = wsdir / 'workspace.toml';target.write_bytes(source.read_bytes())
        s = Session(binary, ws_root, 'untrusted-restore', ws_root)
        try:
            original = target.read_bytes()
            assert b'Terminal 1' not in s.drain(0.1), 'untrusted workspace rendered saved window'
            s.cli('trust', str(target))
        finally:
            s.close()
        s = Session(binary, ws_root, 'trusted-restore', ws_root)
        try:
            restored = ws_root / 'restored-input'
            s.command(f"printf restored > '{restored}'")
            wait_for(restored.exists, 'restored terminal did not receive input')
            # Closing while output is active must complete through the app loop.
            s.command("seq 1 5000")
            s.keys(b'\x01\x17'); s.drain(0.1); s.keys(b'\r'); s.drain(0.3)
            s.keys(b'\x01\x11'); s.drain(0.1); s.keys(b'\r')
            wait_for(lambda: s.server.poll() is not None, 'quit hung after terminal output', timeout=5)
            assert s.server.returncode == 0
        finally:
            s.close()
        results.append('trusted workspace restore, active-output close and quit')

    for result in results:
        print('PASS:', result, flush=True)
    print(f'{len(results)} isolated end-to-end checks passed', flush=True)


if __name__ == '__main__':
    main()
