//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package term

import (
	"syscall"
	"unsafe"
)

const (
	getTermios = syscall.TIOCGETA
	setTermios = syscall.TIOCSETA
)

// flushInput discards input the terminal has queued but nothing has read.
func flushInput(fd int) error {
	which := int32(1) // FREAD
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TIOCFLUSH, uintptr(unsafe.Pointer(&which)))
	if errno != 0 {
		return errno
	}
	return nil
}
