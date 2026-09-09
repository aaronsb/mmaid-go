//go:build linux

package term

import "syscall"

const (
	getTermios = syscall.TCGETS
	setTermios = syscall.TCSETS
)

// flushInput discards input the terminal has queued but nothing has read.
// The syscall package lacks TCFLSH on some architectures, so its value is
// spelled out per family in term_linux_*.go.
func flushInput(fd int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tcflsh, syscall.TCIFLUSH)
	if errno != 0 {
		return errno
	}
	return nil
}
