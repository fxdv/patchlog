package curate

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
)

type Terminal struct {
	fd          int
	oldState    *terminalState
	width       int
	height      int
	signalCh    chan os.Signal
	signalDone  chan struct{}
	restoreOnce sync.Once
}

func SetupTerminal() (*Terminal, error) {
	fd := int(os.Stdin.Fd())

	old, err := makeRawTerminal(fd)
	if err != nil {
		return nil, err
	}

	width, height, _ := terminalSize(fd)
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}

	t := &Terminal{
		fd:         fd,
		oldState:   old,
		width:      width,
		height:     height,
		signalCh:   make(chan os.Signal, 1),
		signalDone: make(chan struct{}),
	}
	signal.Notify(t.signalCh, terminationSignals()...)

	go func() {
		select {
		case <-t.signalCh:
			RestoreTerminal(t)
			os.Exit(0)
		case <-t.signalDone:
		}
	}()

	fmt.Print("\033[?1049h")
	fmt.Print("\033[?25l")

	return t, nil
}

func (t *Terminal) Size() (int, int) {
	w, h, _ := terminalSize(t.fd)
	if w >= 40 && h >= 10 {
		t.width = w
		t.height = h
	}
	return t.width, t.height
}

func RestoreTerminal(t *Terminal) {
	if t == nil {
		return
	}
	t.restoreOnce.Do(func() {
		if t.signalCh != nil {
			signal.Stop(t.signalCh)
		}
		if t.signalDone != nil {
			close(t.signalDone)
		}
		if t.oldState != nil {
			_ = restoreTerminalState(t.fd, t.oldState)
		}
		fmt.Print("\033[?25h")
		fmt.Print("\033[?1049l")
	})
}

func ClearScreen() {
	fmt.Print("\033[2J\033[H")
}

func (t *Terminal) ReadKey() (KeyEvent, error) {
	buf := make([]byte, 32)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return KeyEvent{}, fmt.Errorf("read: %w", err)
	}

	ev, consumed := ParseEscapeSequence(buf[:n])
	if consumed == 0 {
		return KeyEvent{}, fmt.Errorf("incomplete sequence")
	}
	return ev, nil
}
