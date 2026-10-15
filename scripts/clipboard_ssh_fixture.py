"""Private loopback SSH transport for clipboard E2E tests; never uses user sshd."""
import json
import os
from pathlib import Path
import select
import shlex
import shutil
import socket
import subprocess
import time


class SSHSession:
    def __init__(self, binary, root, session_type, wait_for):
        self.root, self.binary = root, binary
        self.command = lambda command: session_type.command(self, command)
        self.open_shell = lambda: session_type.open_shell(self)
        self.ssh = self.sshd = None
        self.logs = []
        try:
            for key_name in ['host-key', 'client-key']:
                subprocess.run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '',
                                '-f', str(root / key_name)], check=True, stdout=subprocess.DEVNULL)
            (root / 'authorized_keys').write_bytes((root / 'client-key.pub').read_bytes())
            with socket.socket() as port_probe:
                port_probe.bind(('127.0.0.1', 0))
                port = port_probe.getsockname()[1]
            config = root / 'sshd_config'
            config.write_text(f'Port {port}\nListenAddress 127.0.0.1\n'
                f'HostKey {root / "host-key"}\nPidFile {root / "sshd.pid"}\n'
                f'AuthorizedKeysFile {root / "authorized_keys"}\n'
                'PasswordAuthentication no\nKbdInteractiveAuthentication no\n'
                'UsePAM no\nStrictModes no\nLogLevel ERROR\n')
            self.sshd_log = open(root / 'sshd.log', 'wb'); self.logs.append(self.sshd_log)
            self.sshd = subprocess.Popen([shutil.which('sshd'), '-D', '-e', '-f', str(config)],
                stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=self.sshd_log)

            def listening():
                if self.sshd.poll() is not None:
                    raise AssertionError((root / 'sshd.log').read_text())
                try:
                    with socket.create_connection(('127.0.0.1', port), timeout=.1):
                        return True
                except OSError:
                    return False
            wait_for(listening, 'private sshd did not listen')
            app_root = root / 'app'; app_root.mkdir()
            (root / 'config.toml').write_text('tour_completed = true\nanimations = false\nenabled_widgets = ["clock"]\n')
            # The bootstrap runs inside an actual SSH PTY. Its finally block
            # reaps exactly the Termdesk server this test starts.
            bootstrap = root / 'bootstrap.py'
            bootstrap.write_text(
                'import fcntl,json,os,pathlib,struct,subprocess,termios,time\n'
                f'root=pathlib.Path({str(root)!r})\n'
                'fcntl.ioctl(0,termios.TIOCSWINSZ,struct.pack("HHHH",30,100,1000,600))\n'
                'env=os.environ.copy()\n'
                'env.update(HOME=str(root),XDG_CONFIG_HOME=str(root/"config"),XDG_CACHE_HOME=str(root/"cache"),'
                'XDG_RUNTIME_DIR=str(root),TERMDESK_HOME=str(root),TERMDESK_CONFIG_PATH=str(root/"config.toml"),'
                'TERMDESK_LOG_LEVEL="off",SHELL="/bin/bash",TERM="xterm-256color")\n'
                'env.pop("TERMDESK",None);env.pop("TERMDESK_PROJECT_DIR",None)\n'
                f'server=subprocess.Popen([{binary!r},"--server","ssh-clipboard"],env=env,cwd=root/"app",'
                'stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE)\n'
                'try:\n'
                ' deadline=time.monotonic()+5\n'
                ' while not (root/"termdesk/ssh-clipboard.sock").exists():\n'
                '  if server.poll() is not None: raise RuntimeError(server.stderr.read().decode())\n'
                '  if time.monotonic()>deadline: raise RuntimeError("server startup timed out")\n'
                '  time.sleep(.02)\n'
                f' client=subprocess.run([{binary!r},"attach","ssh-clipboard"],env=env,cwd=root/"app")\n'
                'finally:\n'
                ' server.terminate();server.wait(timeout=5);server.stderr.close()\n'
                ' (root/"cleanup.json").write_text(json.dumps({"server_reaped":True,"returncode":server.returncode}))\n')
            self.ssh_log = open(root / 'ssh.log', 'wb'); self.logs.append(self.ssh_log)
            self.ssh = subprocess.Popen(['ssh', '-F', '/dev/null', '-tt', '-p', str(port),
                '-i', str(root / 'client-key'), '-o', 'IdentitiesOnly=yes',
                '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=/dev/null',
                '-o', 'LogLevel=ERROR', '127.0.0.1', shlex.join(['python3', str(bootstrap)])],
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=self.ssh_log)
            output = self.drain(1)
            if self.ssh.poll() is not None:
                raise AssertionError('SSH startup failed: ' + (root / 'ssh.log').read_text() + repr(output))
        except BaseException:
            self.close()
            raise

    def keys(self, payload):
        self.ssh.stdin.write(payload)
        self.ssh.stdin.flush()

    def drain(self, duration=.15):
        end = time.monotonic() + duration
        output = b''
        while time.monotonic() < end:
            if select.select([self.ssh.stdout], [], [], min(.03, max(0, end-time.monotonic())))[0]:
                data = os.read(self.ssh.stdout.fileno(), 1 << 20)
                if not data:
                    break
                output += data
        return output

    def close(self):
        if self.ssh is not None:
            if self.ssh.poll() is None:
                try:
                    self.keys(b'\x01d')
                    self.drain(.3)
                    self.ssh.wait(timeout=5)
                except (BrokenPipeError, subprocess.TimeoutExpired):
                    self.ssh.terminate()
                    self.ssh.wait(timeout=5)
            self.ssh.stdin.close(); self.ssh.stdout.close()
        if self.sshd is not None and self.sshd.poll() is None:
            self.sshd.terminate(); self.sshd.wait(timeout=5)
        for log in self.logs:
            log.close()
