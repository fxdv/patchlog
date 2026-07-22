//go:build linux || darwin

package curate

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type terminalState = syscall.Termios

type windowSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func makeRawTerminal(fd int) (*terminalState, error) {
	old := &terminalState{}
	if err := terminalIOCTL(fd, getTerminalStateRequest, unsafe.Pointer(old)); err != nil {
		return nil, fmt.Errorf("read terminal state: %w", err)
	}

	raw := *old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := terminalIOCTL(fd, setTerminalStateRequest, unsafe.Pointer(&raw)); err != nil {
		return nil, fmt.Errorf("set raw terminal state: %w", err)
	}
	return old, nil
}

func restoreTerminalState(fd int, state *terminalState) error {
	if err := terminalIOCTL(fd, setTerminalStateRequest, unsafe.Pointer(state)); err != nil {
		return fmt.Errorf("restore terminal state: %w", err)
	}
	return nil
}

func terminalSize(fd int) (int, int, error) {
	var size windowSize
	if err := terminalIOCTL(fd, syscall.TIOCGWINSZ, unsafe.Pointer(&size)); err != nil {
		return 80, 24, fmt.Errorf("terminal size: %w", err)
	}
	return int(size.Col), int(size.Row), nil
}

func terminalIOCTL(fd int, request uintptr, value unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(value))
	if errno != 0 {
		return errno
	}
	return nil
}

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
