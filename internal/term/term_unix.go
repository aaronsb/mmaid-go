//go:build linux || darwin

package term

import (
	"syscall"
	"time"
	"unsafe"
)

type termios = syscall.Termios

func ioctl(fd int, req uintptr, t *termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func isTerminal(fd int) bool {
	var t termios
	return ioctl(fd, getTermios, &t) == nil
}

func makeRaw(fd int, timeout time.Duration) (*State, error) {
	var old termios
	if err := ioctl(fd, getTermios, &old); err != nil {
		return nil, err
	}
	raw := old
	raw.Iflag &^= syscall.IXON | syscall.ICRNL | syscall.BRKINT | syscall.INPCK | syscall.ISTRIP
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	// A read returns after VTIME tenths of a second with whatever arrived,
	// so a terminal that never answers a query does not hang the probe.
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = uint8(max(1, timeout/(100*time.Millisecond)))
	if err := ioctl(fd, setTermios, &raw); err != nil {
		return nil, err
	}
	return &State{termios: old}, nil
}

func restore(fd int, s *State) error {
	return ioctl(fd, setTermios, &s.termios)
}
