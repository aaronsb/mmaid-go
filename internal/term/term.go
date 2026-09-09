// Package term puts a terminal into raw mode for the tester's probes and
// restores it. Linux and macOS go through ioctl in the syscall package; every
// other GOOS reports raw mode as unsupported and the tester asks instead.
package term

import (
	"errors"
	"time"
)

// ErrUnsupported is returned where raw mode is not implemented.
var ErrUnsupported = errors.New("raw mode is not supported on this platform")

// State is the terminal's settings before MakeRaw, for Restore.
type State struct {
	termios termios
}

// MakeRaw switches fd to raw mode: no echo, no line buffering, no signal
// keys, and reads that return after timeout with nothing rather than block.
func MakeRaw(fd int, timeout time.Duration) (*State, error) {
	return makeRaw(fd, timeout)
}

// Restore puts fd back as MakeRaw found it.
func Restore(fd int, s *State) error {
	if s == nil {
		return nil
	}
	return restore(fd, s)
}

// IsTerminal reports whether fd is a terminal.
func IsTerminal(fd int) bool {
	return isTerminal(fd)
}
