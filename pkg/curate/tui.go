package curate

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

type Terminal struct {
	fd       int
	oldState *termios
	width    int
	height   int
	restore  func()
}

type termios struct {
	IFlag  uint32
	OFlag  uint32
	CFlag  uint32
	LFlag  uint32
	Line   byte
	Cc     [19]byte
	Ispeed uint32
	Ospeed uint32
}

const (
	TCGETS = 0x5401
	TCSETS = 0x5402
	ECHO   = 0x0008
	ICANON = 0x0002
	ISIG   = 0x0001
	IEXTEN = 0x8000
	VMIN   = 1
	VTIME  = 0
)

func terminalSize(fd int) (int, int, error) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return 80, 24, fmt.Errorf("terminal size: %v", errno)
	}
	return int(ws.Col), int(ws.Row), nil
}

func SetupTerminal() (*Terminal, error) {
	fd := int(os.Stdin.Fd())

	old := &termios{}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(TCGETS),
		uintptr(unsafe.Pointer(old)),
	)
	if errno != 0 {
		return nil, fmt.Errorf("TCGETS: %v", errno)
	}

	newState := *old
	newState.LFlag &^= ECHO | ICANON | ISIG | IEXTEN
	newState.Cc[VMIN] = 1
	newState.Cc[VTIME] = 0

	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(TCSETS),
		uintptr(unsafe.Pointer(&newState)),
	)
	if errno != 0 {
		return nil, fmt.Errorf("TCSETS: %v", errno)
	}

	width, height, _ := terminalSize(fd)
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH)

	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGWINCH {
				w, h, _ := terminalSize(fd)
				if w >= 40 && h >= 10 {
					width = w
					height = h
				}
				continue
			}
			RestoreTerminal(&Terminal{fd: fd, oldState: old})
			os.Exit(0)
		}
	}()

	fmt.Print("\033[?1049h")
	fmt.Print("\033[?25l")

	return &Terminal{
		fd:       fd,
		oldState: old,
		width:    width,
		height:   height,
	}, nil
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
	if t == nil || t.oldState == nil {
		return
	}
	syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.fd),
		uintptr(TCSETS),
		uintptr(unsafe.Pointer(t.oldState)),
	)
	fmt.Print("\033[?25h")
	fmt.Print("\033[?1049l")
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
