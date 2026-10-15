#!/usr/bin/env python3
"""Clipboard protocol tests through a real isolated Termdesk server and PTYs.

The socket client stands in for an outer terminal with configurable OSC 52
support. No real user clipboard or running session is touched.
"""
import argparse
from contextlib import ExitStack
import base64
import importlib.util
import json
import re
import shlex
import shutil
from pathlib import Path
import subprocess
import tempfile

spec = importlib.util.spec_from_file_location('e2e', Path(__file__).with_name('test-e2e.py'))
e2e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e2e)

START, END = b'\x1b[200~', b'\x1b[201~'
QUERY = b'\x1b]52;c;?'


def osc(text):
    return b'\x1b]52;c;' + base64.b64encode(text.encode()) + b'\x07'


def capture(session, name, action, copies=(), protected=True):
    ready = session.root / (name + '.ready')
    result = session.root / (name + '.bytes')
    fixture = session.root / (name + '.py')
    mode = b'\x1b[?2004h' if protected else b'\x1b[?2004l'
    fixture.write_text(
        'import os,select,termios,time,tty\n'
        'old=termios.tcgetattr(0);tty.setraw(0)\n'
        'try:\n'
        f' os.write(1, {mode!r})\n'
        f' for fragment in {repr(list(copies))}:\n'
        '  os.write(1, fragment);time.sleep(.04)\n'
        f' open({str(ready)!r}, "w").close()\n'
        ' data=b"";deadline=time.monotonic()+12\n'
        ' while time.monotonic()<deadline:\n'
        '  if select.select([0],[],[],.1)[0]:\n'
        '   data+=os.read(0,65536)\n'
        '   if b"\x04" in data: break\n'
        f' open({str(result)!r}, "wb").write(data.split(b"\x04")[0])\n'
        'finally:\n'
        ' os.write(1,b"\x1b[?2004l");termios.tcsetattr(0,termios.TCSANOW,old)\n')
    output = session.command(f"python3 '{fixture}'")
    e2e.wait_for(ready.exists, f'{name}: capture did not start')
    output += session.drain(.2)
    output += action(session) or b''
    session.keys(b'\x04')
    session.drain(.2)
    e2e.wait_for(result.exists, f'{name}: capture did not stop')
    return result.read_bytes(), output


def test_agents(s, results, api, copy_command=False):
    from clipboard_agent_fixture import COPY_TEXT
    work = s.root
    for agent in ['codex', 'claude']:
        executable = shutil.which(agent)
        if executable is None:
            raise AssertionError(f'{agent} is required for real-agent checks')
        agent_home = work / agent; agent_home.mkdir()
        if agent == 'codex':
            argv = [executable, '--no-alt-screen',
                    '-c', 'check_for_update_on_startup=false',
                    '-c', 'model_provider="clipboard-test"',
                    '-c', 'model_providers.clipboard-test.name="Clipboard test"',
                    '-c', 'model_providers.clipboard-test.base_url="http://127.0.0.1:9/v1"',
                    '-c', 'model_providers.clipboard-test.wire_api="responses"',
                    '-c', 'model_providers.clipboard-test.env_key="OPENAI_API_KEY"',
                    '-c', f'projects."{work}".trust_level="trusted"']
            env = ['CODEX_HOME=' + str(agent_home), 'OPENAI_API_KEY=clipboard-test-placeholder']
        else:
            (agent_home / '.claude.json').write_text(json.dumps({
                'hasCompletedOnboarding': True, 'theme': 'dark', 'numStartups': 1}))
            argv = [executable, '--bare']
            env = ['CLAUDE_CONFIG_DIR=' + str(agent_home),
                   'ANTHROPIC_API_KEY=clipboard-test-placeholder',
                   'ANTHROPIC_BASE_URL=' + api.url,
                   'CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1']
        output = s.command(shlex.join(['env', '-u', 'DISPLAY', '-u', 'WAYLAND_DISPLAY', '-u', 'DBUS_SESSION_BUS_ADDRESS', 'HOME=' + str(agent_home)] + env + argv))
        output += s.drain(2)
        # Preserve the bounded startup capture for diagnosis, without
        # copying any user configuration or credentials.
        (work / (agent + '-startup.ansi')).write_bytes(output)
        if agent == 'claude' and b'trust this folder' in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', output):
            s.keys(b'\x1b[B\r')
            output += s.drain(2)
        if agent == 'claude' and b'Detected a custom API key' in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', output):
            s.keys(b'\x1b[A\r')
            output += s.drain(2)
        s.keys(START + (agent + '-native-paste-probe').encode() + END)
        pasted = s.drain(.5)
        marker = (agent + '-native-paste-probe').encode()
        startup = re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', output)
        assert marker in pasted, f'{agent} native paste missing: {pasted[-1200:]!r}; startup={startup[-2200:]!r}'
        assert b'Failed to paste image' not in pasted
        s.keys(b'\x1b[2;2~')
        query_output = s.drain(.2)
        assert QUERY in query_output, f'{agent}: Shift+Insert did not query clipboard'
        second_marker = (agent + '-shortcut-paste-probe').encode()
        s.keys(osc(second_marker.decode()))
        shortcut_output = s.drain(.4)
        assert second_marker in shortcut_output, f'{agent}: shortcut text missing: {shortcut_output[-1200:]!r}'
        assert b'Failed to paste image' not in shortcut_output
        results.append(f'real {agent} TUI accepts native and Shift+Insert text paste without image errors')
        if agent == 'claude' and copy_command:
            # Clear the unsent paste probes, ask the loopback fixture for one
            # response, then exercise Claude's own /copy action.
            s.keys(b'\x15' + b'clipboard fixture response please\r')
            response = s.drain(3)
            assert COPY_TEXT.encode() in response, f'Claude fixture response missing: {response[-2000:]!r}; requests={api.requests}'
            s.keys(b'/copy\r')
            copied_output = s.drain(1)
            copies = re.findall(rb'\x1b\]52;[^;]*;([^\x07]*?)\x07', copied_output)
            decoded = [base64.b64decode(copy) for copy in copies]
            assert any(COPY_TEXT.encode() in copy for copy in decoded), f'Claude /copy did not reach host clipboard: {copied_output[-2500:]!r}'
            s.keys(b'\x01]')
            saved = s.drain(.4)
            assert COPY_TEXT.encode() in saved, f'Claude /copy did not remain in Termdesk clipboard: {saved[-1500:]!r}'
            results.append('real Claude /copy over SSH reaches host and stays available to prefix paste (loopback response fixture)')
        s.keys(b'\x03');s.drain(.15);s.keys(b'\x03');s.drain(.3)

def test_nvim(s, results):
    result = s.root / 'nvim-clipboard.txt'
    ready = s.root / 'nvim-ready'
    init = s.root / 'nvim-init.lua'
    init.write_text("local osc=require('vim.ui.clipboard.osc52')\n"
        "vim.g.clipboard={name='OSC52',copy={['+']=osc.copy('+'),['*']=osc.copy('*')},"
        "paste={['+']=osc.paste('+'),['*']=osc.paste('*')}}\n")
    s.command(shlex.join(['nvim', '-u', str(init), '-i', 'NONE', '-n',
                         '-c', f"call writefile(['ready'], '{ready}')", str(result)]))
    e2e.wait_for(ready.exists, 'Neovim did not start')
    s.drain(.2)
    s.keys(b'"+p')
    output = s.drain(.2)
    assert QUERY in output, f'Neovim OSC52 read did not reach outer terminal: {output[-1000:]!r}'
    text = 'Neovim native clipboard read and copy'
    s.keys(osc(text));s.drain(.3)
    s.keys(b'ggVG"+y')
    output = s.drain(.4)
    copies = re.findall(rb'\x1b\]52;[^;]*;([^\x07]*?)\x07', output)
    assert any(text.encode() in base64.b64decode(copy) for copy in copies), 'Neovim native yank did not reach host clipboard'
    s.keys(b':wq\r');s.drain(.3)
    e2e.wait_for(result.exists, 'Neovim did not save clipboard text')
    assert result.read_text() == text + '\n', repr(result.read_text())
    results.append('real Neovim native OSC52 clipboard read and yank round trip')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', help='test an existing build instead of building')
    args = parser.parse_args()
    results = []
    with tempfile.TemporaryDirectory(prefix='td-clipboard-', dir='/tmp') as temp, ExitStack() as cleanup:
        root = Path(temp)
        binary = str(Path(args.binary).resolve()) if args.binary else str(root / 'termdesk-bin')
        if not args.binary:
            subprocess.run(['go', 'build', '-o', binary, './cmd/termdesk'], cwd=e2e.REPO, check=True)
        work = root / 'session'; work.mkdir()
        s = e2e.Session(binary, work, 'clipboard', work)
        try:
            s.open_shell()
            text = 'Unicode → 日本語\nsecond line\tend'

            def native(s):
                # Fragment both markers and multibyte text across socket writes.
                for fragment in [START[:2], START[2:], text.encode()[:13], text.encode()[13:], END[:3], END[3:]]:
                    s.keys(fragment)
                    s.drain(.02)
            got, _ = capture(s, 'native', native)
            assert got == START + text.encode() + END, repr(got)
            results.append('fragmented native Unicode/multiline paste arrives exactly once with brackets')

            def host_reply(s):
                s.keys(b'\x1b[2;2~')  # Shift+Insert
                output = s.drain(.2)
                assert QUERY in output, 'Paste did not request the outer clipboard'
                s.keys(osc('fresh host text'))
                return output + s.drain(1.15)
            got, _ = capture(s, 'host', host_reply)
            assert got == START + b'fresh host text' + END, repr(got)
            results.append('Shift+Insert reads host text rather than triggering child image paste; no timeout duplicate')

            def fallback(s):
                s.keys(b'\x1b[2;2~')
                return s.drain(1.25)
            got, _ = capture(s, 'unsupported-read', fallback)
            assert got == START + b'fresh host text' + END, repr(got)
            results.append('unsupported host clipboard reads fall back to saved text')

            copied = osc('copied inside Claude/Vim')
            def prefix(s):
                s.keys(b'\x01]')
                return s.drain(.2)
            got, output = capture(s, 'child-copy', prefix, [copied[:1], copied[1:3], copied[3:4], copied[4:]])
            assert got == START + b'copied inside Claude/Vim' + END, repr(got)
            assert copied in output, 'child OSC 52 was not forwarded to outer terminal'
            results.append('child OSC 52 split inside introducer reaches host and internal paste')

            def child_read(s):
                s.keys(osc('clipboard read by child'))
                return s.drain(.2)
            got, output = capture(s, 'child-read', child_read, [b'\x1b]52;c;?\x07'])
            assert QUERY in output, 'child clipboard query did not reach outer terminal'
            assert got == osc('clipboard read by child'), repr(got)
            results.append('child OSC 52 clipboard reads receive the host reply as a protocol response')

            burst = b''.join(osc(f'rapid copy {i:03}') for i in range(50))
            got, output = capture(s, 'rapid-copies', prefix, [burst])
            assert got == START + b'rapid copy 049' + END, repr(got)
            forwarded = output.rsplit(b'\x1b]52;c;', 1)[-1].split(b'\x07', 1)[0]
            assert base64.b64decode(forwarded) == b'rapid copy 049', repr(forwarded)
            results.append('rapid child copies preserve the newest internal and host clipboard values')

            got, _ = capture(s, 'plain-application', prefix, protected=False)
            assert got == b'rapid copy 049', repr(got)
            results.append('applications without bracketed paste receive plain text')

            def ctrl_shift_v(s):
                s.keys(b'\x1b[118;6u')  # CSI-u Ctrl+Shift+V, where distinguishable
                output = s.drain(.2)
                assert QUERY in output, 'modified Ctrl+Shift+V was not intercepted'
                s.keys(osc('modified key paste'))
                return output + s.drain(.2)
            got, _ = capture(s, 'modified-key', ctrl_shift_v)
            assert got == START + b'modified key paste' + END, repr(got)
            results.append('distinguishable Ctrl+Shift+V is a text paste, not a child control key')

            def control_v(s):
                s.keys(b'\x16')
                return s.drain(.2)
            got, _ = capture(s, 'literal-control-v', control_v)
            assert got == b'\x16', repr(got)
            results.append('literal Ctrl+V reaches applications unchanged')

            def native_during_query(s):
                s.keys(b'\x1b[2;2~'); s.drain(.1)
                s.keys(START + b'native wins' + END)
                s.drain(.15)
                s.keys(osc('late reply'))
                return s.drain(1.1)
            got, _ = capture(s, 'cancel-query', native_during_query)
            assert got == START + b'native wins' + END, repr(got)
            results.append('native paste cancels pending read; late response and timeout cannot duplicate it')

            def menu_paste(s):
                s.keys(b'\x01e'); s.drain(.15)
                s.keys(b'\r')
                output = s.drain(.2)
                assert QUERY in output, 'Edit > Paste did not query host clipboard'
                s.keys(osc('menu clipboard'))
                return output + s.drain(.2)
            got, _ = capture(s, 'edit-menu', menu_paste)
            assert got == START + b'menu clipboard' + END, repr(got)
            results.append('Edit > Paste invokes the same host clipboard path')

            from clipboard_agent_fixture import AgentAPI
            api = cleanup.enter_context(AgentAPI())
            test_agents(s, results, api)

            # Real Vim: native paste in insert mode and Ctrl+V in Normal mode.
            vim_result = work / 'vim-paste.txt'
            vim_ready = work / 'vim-ready'
            s.command(f"vim -Nu NONE -n -i NONE -c 'call writefile([\"ready\"], \"{vim_ready}\")' '{vim_result}'")
            e2e.wait_for(vim_ready.exists, 'Vim did not start')
            s.drain(.2)
            vim_text = 'Vim clipboard → 日本語\nsecond line'
            s.keys(b'i' + START + vim_text.encode() + END)
            s.drain(.2)
            s.keys(b'\x1b'); s.drain(.1)
            # Assert Ctrl+V still enters visual block mode in the real app.
            block = work / 'vim-block'
            s.keys(b'\x16'); s.drain(.1)
            s.keys(b':'+f"call writefile([visualmode()], '{block}')".encode()+b'\r')
            s.drain(.15)
            s.keys(b':wq\r'); s.drain(.3)
            e2e.wait_for(vim_result.exists, 'Vim did not save')
            assert vim_result.read_text() == vim_text + '\n', repr(vim_result.read_text())
            assert block.read_bytes() == b'\x16\n', 'Vim Ctrl+V visual block mode was broken'
            results.append('real Vim pastes Unicode/multiline text and retains Ctrl+V visual block mode')
            test_nvim(s, results)
        finally:
            s.close()
        from clipboard_ssh_fixture import SSHSession
        ssh_root = root / 'ssh'; ssh_root.mkdir()
        ssh_session = SSHSession(binary, ssh_root, e2e.Session, e2e.wait_for)
        try:
            ssh_session.open_shell()
            got, _ = capture(ssh_session, 'ssh-native', native)
            assert got == START + text.encode() + END, repr(got)
            got, _ = capture(ssh_session, 'ssh-shortcut', host_reply)
            assert got == START + b'fresh host text' + END, repr(got)
            got, output = capture(ssh_session, 'ssh-copy', prefix,
                                  [copied[:1], copied[1:3], copied[3:4], copied[4:]])
            assert got == START + b'copied inside Claude/Vim' + END, repr(got)
            assert copied in output, 'SSH lost OSC 52 clipboard write'
            got, output = capture(ssh_session, 'ssh-child-read', child_read, [b'\x1b]52;c;?\x07'])
            assert QUERY in output and got == osc('clipboard read by child'), repr(got)
            env_file = ssh_root / 'child-ssh-env'
            ssh_session.command(f'printf "%s" "$SSH_TTY" > "{env_file}"')
            e2e.wait_for(env_file.exists, 'SSH child environment receipt missing')
            assert env_file.read_text().startswith('/dev/pts/'), 'real SSH_TTY did not reach child'
            results.append('real private SSH PTY: native paste, Shift+Insert, fragmented child copy, remote detection')
            test_nvim(ssh_session, results)
            test_agents(ssh_session, results, api, copy_command=True)
        finally:
            ssh_session.close()
        receipt = ssh_root / 'cleanup.json'
        assert receipt.exists() and json.loads(receipt.read_text())['server_reaped'], 'SSH server cleanup not confirmed'
        results.append('private SSH client, server and sshd shut down; Termdesk server reaped')
    print(json.dumps({'passed': results}, indent=2))


if __name__ == '__main__':
    main()
