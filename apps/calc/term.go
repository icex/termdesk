package main

import (
	"golang.org/x/sys/unix"
)

func makeRaw(fd uintptr) (*unix.Termios, error) {
	old, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Cflag |= unix.CS8
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(fd), unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return old, nil
}

func restore(fd uintptr, state *unix.Termios) {
	unix.IoctlSetTermios(int(fd), unix.TCSETS, state)
}
