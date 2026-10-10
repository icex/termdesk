// Package ttyprobe bounds terminal capability queries without relying on Go
// read deadlines, which inherited terminal descriptors may not support.
package ttyprobe

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func ReadUntil(in *os.File, timeout time.Duration, complete func([]byte) bool) []byte {
	deadline := time.Now().Add(timeout)
	var response []byte
	buf := make([]byte, 1024)
	for len(response) < 4096 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		poll := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if err == unix.EINTR {
			continue
		}
		if err != nil || n == 0 || poll[0].Revents&unix.POLLIN == 0 {
			break
		}
		n, err = unix.Read(int(in.Fd()), buf)
		if n > 0 {
			response = append(response, buf[:n]...)
			if complete(response) {
				break
			}
		}
		if err != nil {
			break
		}
	}
	return response
}
