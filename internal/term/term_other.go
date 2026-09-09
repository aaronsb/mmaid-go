//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package term

import "time"

type termios struct{}

func isTerminal(int) bool { return false }

func makeRaw(int, time.Duration) (*State, error) { return nil, ErrUnsupported }

func restore(int, *State) error { return ErrUnsupported }
