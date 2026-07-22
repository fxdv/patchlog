//go:build darwin

package curate

import "syscall"

const (
	getTerminalStateRequest = syscall.TIOCGETA
	setTerminalStateRequest = syscall.TIOCSETA
)
