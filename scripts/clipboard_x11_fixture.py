"""A private authenticated X11 clipboard; never accesses the user's display."""
import os
from pathlib import Path
import secrets
import subprocess


class X11Clipboard:
    def __init__(self, root, executable, wait_for):
        self.server = self.owner = None
        self.display = ':' + str(15000 + secrets.randbelow(10000))
        authority = root / 'Xauthority'
        self.env = dict(os.environ, DISPLAY=self.display, XAUTHORITY=str(authority))
        for key in ['WAYLAND_DISPLAY', 'SSH_TTY', 'SSH_CONNECTION', 'SSH_CLIENT']:
            self.env.pop(key, None)
        subprocess.run(['xauth', '-f', str(authority), 'add', self.display, '.', secrets.token_hex(16)],
                       check=True, stderr=subprocess.DEVNULL)
        self.log = open(root / 'xvfb.log', 'wb')
        try:
            self.server = subprocess.Popen([executable, self.display, '-screen', '0', '1024x768x24',
                '-nolisten', 'tcp', '-auth', str(authority)], stdout=subprocess.DEVNULL, stderr=self.log)
            wait_for(lambda: Path('/tmp/.X11-unix/X' + self.display[1:]).exists(),
                     'private X server did not start')
            self.owner = subprocess.Popen(['xclip', '-quiet', '-selection', 'clipboard'], env=self.env,
                stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            self.owner.stdin.write(b'PRIVATE_PREVIOUS_CLIPBOARD')
            self.owner.stdin.close()
            wait_for(lambda: self.read() == b'PRIVATE_PREVIOUS_CLIPBOARD', 'private clipboard did not initialize')
        except BaseException:
            self.close()
            raise

    def read(self):
        return subprocess.run(['xclip', '-selection', 'clipboard', '-o'], env=self.env,
                              capture_output=True, timeout=2).stdout

    def close(self):
        for process in [self.owner, self.server]:
            if process is not None and process.poll() is None:
                process.terminate()
                process.wait(timeout=5)
        self.log.close()
