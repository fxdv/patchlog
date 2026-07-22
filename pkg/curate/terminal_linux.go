//go:build linux

package curate

import "syscall"

const (
	getTerminalStateRequest = syscall.TCGETS
	setTerminalStateRequest = syscall.TCSETS
)
