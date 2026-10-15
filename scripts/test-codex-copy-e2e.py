#!/usr/bin/env python3
"""Regress Codex drag-copy after attaching over SSH to a local GUI session.

Uses a private X server, dummy credentials and a loopback response fixture.
No user clipboard, conversations, or existing Termdesk processes are accessed.
"""
import argparse
import base64
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile
from unittest.mock import patch

from clipboard_agent_fixture import AgentAPI, CODEX_COPY_TEXT
from clipboard_ssh_fixture import SSHSession
from clipboard_x11_fixture import X11Clipboard

spec = importlib.util.spec_from_file_location('e2e', Path(__file__).with_name('test-e2e.py'))
e2e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e2e)

SCREEN_HELPER = '''package main
import (
 "encoding/json"
 "io"
 "os"
 "strings"
 "github.com/charmbracelet/x/vt"
)
func main() {
 e := vt.NewSafeEmulator(100, 30)
 defer e.Close()
 go io.Copy(io.Discard, e)
 b, _ := io.ReadAll(os.Stdin)
 e.Write(b)
 rows := []string{}
 for y := 0; y < 30; y++ {
  var s strings.Builder
  for x := 0; x < 100; x++ {
   c := e.CellAt(x, y)
   if c == nil || c.Content == "" { s.WriteByte(' ') } else if c.Width > 0 { s.WriteString(c.Content) }
  }
  rows = append(rows, s.String())
 }
 json.NewEncoder(os.Stdout).Encode(rows)
}
'''


def copied_text(output):
    payloads = re.findall(rb'\x1b\]52;[^;]*;([^\x07]*?)\x07', output)
    return [base64.b64decode(payload) for payload in payloads]


def mouse(session, button, x, y, release=False):
    session.keys(f'\x1b[<{button};{x};{y}{"m" if release else "M"}'.encode())
    return session.drain(.1)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', help='test an existing build')
    parser.add_argument('--xvfb', default=shutil.which('Xvfb'), help='path to an Xvfb executable')
    args = parser.parse_args()
    if not args.xvfb:
        parser.error('Xvfb is required; use --xvfb /path/to/Xvfb')
    results = []
    with tempfile.TemporaryDirectory(prefix='td-codex-copy-', dir='/tmp') as temp, AgentAPI() as api:
        root = Path(temp)
        binary = str(Path(args.binary).resolve()) if args.binary else str(root / 'termdesk-bin')
        if not args.binary:
            subprocess.run(['go', 'build', '-o', binary, './cmd/termdesk'], cwd=e2e.REPO, check=True)
        helper_source = root / 'screen.go'; helper_source.write_text(SCREEN_HELPER)
        helper = root / 'screen'
        subprocess.run(['go', 'build', '-o', str(helper), str(helper_source)], cwd=e2e.REPO, check=True)
        desktop = X11Clipboard(root, args.xvfb, e2e.wait_for)
        local = remote = None
        try:
            work = root / 'local'; work.mkdir()
            with patch.dict(os.environ, desktop.env, clear=True):
                local = e2e.Session(binary, work, 'local-origin', work)
            local.open_shell()
            codex_home = root / 'codex'; codex_home.mkdir()
            command = ['env', 'HOME=' + str(codex_home), 'CODEX_HOME=' + str(codex_home),
                'OPENAI_API_KEY=clipboard-test-placeholder', shutil.which('codex'),
                '-c', 'check_for_update_on_startup=false',
                '-c', 'model_provider="clipboard-test"',
                '-c', 'model_providers.clipboard-test.name="Clipboard test"',
                '-c', 'model_providers.clipboard-test.base_url="' + api.url + '/v1"',
                '-c', 'model_providers.clipboard-test.wire_api="responses"',
                '-c', 'model_providers.clipboard-test.env_key="OPENAI_API_KEY"',
                '-c', f'projects."{work}".trust_level="trusted"']
            output = local.command(shlex.join(command)) + local.drain(2)
            local.keys(b'\x01k'); output += local.drain(.4)
            local.keys(b'\x1b[200~clipboard fixture response please\x1b[201~')
            output += local.drain(.2)
            local.keys(b'\r')
            response = local.drain(3); output += response
            startup_text = re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', output)
            assert CODEX_COPY_TEXT.encode() in response, f'Codex did not render the loopback response: {startup_text[-1800:]!r}; paths={api.requests}'
            results.append('fullscreen Codex started locally with a usable private desktop clipboard')

            ssh_root = root / 'ssh'; ssh_root.mkdir()
            remote = SSHSession(binary, ssh_root, e2e.Session, e2e.wait_for, existing=local)
            output += remote.initial_output + remote.drain(.3)
            assert local.server.poll() is None, 'reattach replaced the local session process'
            results.append('real SSH reattach preserved the running local-origin session and Codex')

            for backwards in [False, True]:
                rows = json.loads(subprocess.check_output([str(helper)], input=output, timeout=5))
                found = [(row.index(CODEX_COPY_TEXT) + 1, y + 1) for y, row in enumerate(rows)
                         if CODEX_COPY_TEXT in row and '• ' in row]
                assert found, 'Codex response is not visible after reattach'
                x, y = found[0]
                start, end = (x + len(CODEX_COPY_TEXT), x) if backwards else (x, x + len(CODEX_COPY_TEXT))
                selection = mouse(remote, 0, start, y)
                selection += mouse(remote, 32, end, y)
                selection += mouse(remote, 0, end, y, release=True)
                selection += remote.drain(.6)
                output += selection
                assert CODEX_COPY_TEXT.encode() in copied_text(selection), (
                    'Codex copied to the server desktop but the SSH client received no matching OSC52 copy')
                assert desktop.read() == CODEX_COPY_TEXT.encode(), 'Codex native selection/copy behavior changed'
                results.append(('backward' if backwards else 'forward') +
                               ' native Codex drag updates both its desktop clipboard and the attached terminal')

            # A plain click must not overwrite the user's previous copy.
            click = mouse(remote, 0, x, y) + mouse(remote, 0, x, y, release=True)
            assert not copied_text(click), 'plain click unexpectedly replaced the clipboard'
            results.append('plain clicks preserve clipboard contents')

            remote.keys(b'\x01]')
            pasted = remote.drain(.4)
            assert CODEX_COPY_TEXT.encode() in pasted, 'native selection did not enter Termdesk clipboard history'
            results.append('selected text stays available for Termdesk prefix paste')
            remote.keys(b'\x03'); remote.drain(.15)
            remote.keys(b'\x03'); remote.drain(.3)
        finally:
            if remote is not None:
                remote.close()
                receipt = json.loads((root / 'ssh/cleanup.json').read_text())
                assert receipt['existing_server_preserved']
            if local is not None:
                local.close()
            desktop.close()
        results.append('private SSH, desktop clipboard and local-origin server cleaned up')
    print(json.dumps({'passed': results}, indent=2))


if __name__ == '__main__':
    main()
