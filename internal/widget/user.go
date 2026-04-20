package widget

import (
	"os"
	"runtime"
	"strings"
)

// UserWidget displays an OS icon and the logged-in username.
type UserWidget struct {
	Username string
}

func (w *UserWidget) Name() string { return "user" }

func (w *UserWidget) Render() string {
	if w.Username == "" {
		return ""
	}
	return osIcon() + " " + w.Username
}

func (w *UserWidget) ColorLevel() string { return "" }

// osIcon returns a Nerd Font icon for the detected operating system.
func osIcon() string {
	return osIconForGOOS(runtime.GOOS)
}

// osIconForGOOS returns the Nerd Font icon for the given GOOS value.
func osIconForGOOS(goos string) string {
	switch goos {
	case "darwin":
		return "\uf179" //  Apple
	case "android":
		return "\uf17b" //  Android
	case "windows":
		return "\uf17a" //  Windows
	case "freebsd":
		return "\uf30c" //  FreeBSD
	default:
		// Linux — try to detect specific distro
		return linuxDistroIcon()
	}
}

// linuxDistroIcon reads /etc/os-release to detect the Linux distribution.
func linuxDistroIcon() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "\uf17c" //  generic Linux (Tux)
	}
	return matchDistroIcon(string(data))
}

// matchDistroIcon returns the Nerd Font icon for a Linux distro based on os-release content.
func matchDistroIcon(data string) string {
	lower := strings.ToLower(data)
	switch {
	case strings.Contains(lower, "ubuntu"):
		return "\uf31b" //  Ubuntu
	case strings.Contains(lower, "debian"):
		return "\uf306" //  Debian
	case strings.Contains(lower, "fedora"):
		return "\uf30a" //  Fedora
	case strings.Contains(lower, "arch"):
		return "\uf303" //  Arch
	case strings.Contains(lower, "centos"):
		return "\uf304" //  CentOS
	case strings.Contains(lower, "manjaro"):
		return "\uf312" //  Manjaro
	case strings.Contains(lower, "suse"), strings.Contains(lower, "opensuse"):
		return "\uf314" //  openSUSE
	case strings.Contains(lower, "mint"):
		return "\uf30e" //  Linux Mint
	case strings.Contains(lower, "gentoo"):
		return "\uf30d" //  Gentoo
	case strings.Contains(lower, "alpine"):
		return "\uf300" //  Alpine
	case strings.Contains(lower, "nixos"):
		return "\uf313" //  NixOS
	case strings.Contains(lower, "void"):
		return "\uf32e" //  Void
	case strings.Contains(lower, "pop"):
		return "\uf32a" //  Pop!_OS (uses custom)
	case strings.Contains(lower, "raspberry"):
		return "\uf315" //  Raspberry Pi
	default:
		return "\uf17c" //  generic Linux (Tux)
	}
}
