package widget

import "os"

// HostnameWidget displays the machine hostname with an SSH indicator.
type HostnameWidget struct {
	Hostname string
	IsSSH    bool
}

func (w *HostnameWidget) Name() string { return "hostname" }

func (w *HostnameWidget) Render() string {
	if w.Hostname == "" {
		return ""
	}
	if w.IsSSH {
		return "\uf023 " + w.Hostname // nf-fa-lock
	}
	return "\uf108 " + w.Hostname // 󰈈 monitor icon
}

func (w *HostnameWidget) ColorLevel() string { return "" }

// isSSHSession returns true if the current session is over SSH.
func isSSHSession() bool {
	for _, v := range []string{"SSH_TTY", "SSH_CLIENT", "SSH_CONNECTION"} {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}
