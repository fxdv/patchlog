// Package console provides terminal output utilities: spinners, colored text, and release summary formatting.
package console

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var spinnerFramesASCII = []string{"|", "/", "-", "\\"}

var isTTY = checkTTY()
var noColor = checkNoColor()

func checkNoColor() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" || !isTTY
}

func checkTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func c(code, s string) string {
	if noColor {
		return s
	}
	return code + s + Reset
}

func BoldText(s string) string   { return c(Bold, s) }
func DimText(s string) string    { return c(Dim, s) }
func RedText(s string) string    { return c(Red, s) }
func GreenText(s string) string  { return c(Green, s) }
func YellowText(s string) string { return c(Yellow, s) }
func BlueText(s string) string   { return c(Blue, s) }
func CyanText(s string) string   { return c(Cyan, s) }

type Spinner struct {
	mu       sync.Mutex
	message  string
	active   bool
	done     chan struct{}
	finished chan struct{}
	writer   io.Writer
}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		message:  message,
		done:     make(chan struct{}),
		finished: make(chan struct{}),
		writer:   os.Stderr,
	}
}

func (s *Spinner) Start() {
	if !isTTY {
		fmt.Fprintf(s.writer, "  %s\n", s.message)
		return
	}
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.mu.Unlock()

	frames := spinnerFrames
	if !isTTY {
		frames = spinnerFramesASCII
	}

	go func() {
		defer close(s.finished)
		i := 0
		for {
			s.mu.Lock()
			if !s.active {
				s.mu.Unlock()
				return
			}
			msg := s.message
			s.mu.Unlock()

			frame := frames[i%len(frames)]
			fmt.Fprintf(s.writer, "\r  %s %s", CyanText(frame), msg)
			i++

			select {
			case <-s.done:
				return
			case <-time.After(80 * time.Millisecond):
			}
		}
	}()
}

func (s *Spinner) Stop(success bool) {
	if !isTTY {
		return
	}
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	s.mu.Unlock()

	close(s.done)
	<-s.finished

	icon := GreenText("✓")
	if !success {
		icon = RedText("✗")
	}
	fmt.Fprintf(s.writer, "\r  %s %s\n", icon, s.message)
}

func (s *Spinner) Update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

type Summary struct {
	Version       string
	Commits       int
	JiraTickets   int
	BumpLevel     string
	Tag           string
	Pushed        bool
	Published     bool
	PublishURL    string
	Confluence    bool
	ConfluenceURL string
	Changelog     bool
	ChangelogURL  string
	Trends        bool
	TrendsURL     string
	DepsCount     int
	Duration      time.Duration
}

func PrintSummary(s Summary) {
	w := os.Stderr
	fmt.Fprint(w, "\n")
	fmt.Fprintf(w, "  %s\n", BoldText("─── Release Summary ───"))
	fmt.Fprintf(w, "  %-20s %s\n", "Version:", BoldText(GreenText(s.Version)))
	fmt.Fprintf(w, "  %-20s %d\n", "Commits analyzed:", s.Commits)
	if s.JiraTickets > 0 {
		fmt.Fprintf(w, "  %-20s %s\n", "Jira tickets:", CyanText(fmt.Sprintf("%d enriched", s.JiraTickets)))
	}
	if s.BumpLevel != "" {
		bumpColor := YellowText
		switch s.BumpLevel {
		case "major":
			bumpColor = RedText
		case "minor":
			bumpColor = YellowText
		case "patch":
			bumpColor = GreenText
		}
		fmt.Fprintf(w, "  %-20s %s\n", "Bump level:", bumpColor(s.BumpLevel))
	}
	if s.Tag != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Tagged:", GreenText(s.Tag))
	}
	if s.Pushed {
		fmt.Fprintf(w, "  %-20s %s\n", "Pushed:", GreenText("yes"))
	}
	if s.Published {
		fmt.Fprintf(w, "  %-20s %s\n", "Release draft:", GreenText(s.PublishURL))
	}
	if s.Confluence {
		fmt.Fprintf(w, "  %-20s %s\n", "Confluence page:", BlueText(s.ConfluenceURL))
	}
	if s.Changelog {
		label := "Changelog:"
		if s.ChangelogURL != "" {
			fmt.Fprintf(w, "  %-20s %s\n", label, BlueText(s.ChangelogURL))
		} else {
			fmt.Fprintf(w, "  %-20s %s\n", label, GreenText("updated"))
		}
	}
	if s.Trends {
		label := "Trends dashboard:"
		if s.TrendsURL != "" {
			fmt.Fprintf(w, "  %-20s %s\n", label, BlueText(s.TrendsURL))
		} else {
			fmt.Fprintf(w, "  %-20s %s\n", label, GreenText("updated"))
		}
	}
	if s.DepsCount > 0 {
		fmt.Fprintf(w, "  %-20s %s\n", "Dependencies:", CyanText(fmt.Sprintf("%d changed", s.DepsCount)))
	}
	fmt.Fprintf(w, "  %-20s %s\n", "Duration:", DimText(s.Duration.Round(time.Millisecond).String()))
	fmt.Fprint(w, "\n")
}

func Step(label string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", DimText("→"), label)
}
