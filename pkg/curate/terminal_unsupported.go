//go:build !linux && !darwin

package curate

import (
	"fmt"
	"os"
	"runtime"
)

type terminalState struct{}

func makeRawTerminal(int) (*terminalState, error) {
	return nil, fmt.Errorf("interactive curate terminal is not supported on %s", runtime.GOOS)
}

func restoreTerminalState(int, *terminalState) error {
	return nil
}

func terminalSize(int) (int, int, error) {
	return 80, 24, nil
}

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
