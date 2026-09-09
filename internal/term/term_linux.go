//go:build linux

package term

import "syscall"

const (
	getTermios = syscall.TCGETS
	setTermios = syscall.TCSETS
)
