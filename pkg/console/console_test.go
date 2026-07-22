package console

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCheckNoColor(t *testing.T) {
	original := os.Getenv("NO_COLOR")
	originalTTY := isTTY
	defer func() {
		os.Setenv("NO_COLOR", original)
		isTTY = originalTTY
	}()

	isTTY = true
	os.Unsetenv("NO_COLOR")
	os.Setenv("TERM", "xterm")
	noColor = checkNoColor()
	if noColor {
		t.Error("expected noColor=false without NO_COLOR")
	}

	os.Setenv("NO_COLOR", "1")
	noColor = checkNoColor()
	if !noColor {
		t.Error("expected noColor=true with NO_COLOR set")
	}
}

func TestColorFunctionsNoColor(t *testing.T) {
	original := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", original)
	os.Setenv("NO_COLOR", "1")
	noColor = true

	if BoldText("x") != "x" {
		t.Error("BoldText should return plain with noColor")
	}
	if RedText("x") != "x" {
		t.Error("RedText should return plain with noColor")
	}
	if CyanText("x") != "x" {
		t.Error("CyanText should return plain with noColor")
	}
}

func TestColorFunctionsWithColor(t *testing.T) {
	original := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", original)
	os.Unsetenv("NO_COLOR")
	noColor = false

	got := RedText("hello")
	if !strings.Contains(got, "\033[31m") {
		t.Error("RedText should contain ANSI red code")
	}
	if !strings.Contains(got, "\033[0m") {
		t.Error("RedText should contain reset code")
	}
}

func TestSpinnerWithWriter(t *testing.T) {
	originalTTY := isTTY
	defer func() { isTTY = originalTTY }()
	isTTY = true

	var buf bytes.Buffer
	s := &Spinner{
		message:  "testing",
		done:     make(chan struct{}),
		finished: make(chan struct{}),
		writer:   &buf,
	}
	s.Start()
	time.Sleep(150 * time.Millisecond)
	s.Stop(true)
	output := buf.String()
	if !strings.Contains(output, "testing") {
		t.Error("spinner output should contain message")
	}
	if !strings.Contains(output, "✓") {
		t.Error("spinner output should contain checkmark on success")
	}
}

func TestSpinnerStopFailure(t *testing.T) {
	originalTTY := isTTY
	defer func() { isTTY = originalTTY }()
	isTTY = true

	var buf bytes.Buffer
	s := &Spinner{
		message:  "failing",
		done:     make(chan struct{}),
		finished: make(chan struct{}),
		writer:   &buf,
	}
	s.Start()
	time.Sleep(150 * time.Millisecond)
	s.Stop(false)
	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Error("spinner output should contain X on failure")
	}
}

func TestSpinnerUpdate(t *testing.T) {
	s := NewSpinner("initial")
	s.Update("updated")
	s.mu.Lock()
	msg := s.message
	s.mu.Unlock()
	if msg != "updated" {
		t.Errorf("expected message 'updated', got %q", msg)
	}
}

func TestStep(t *testing.T) {
	original := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = original }()

	Step("test step")
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "test step") {
		t.Error("Step should write the label to stderr")
	}
}

func TestPrintSummary(t *testing.T) {
	original := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = original }()

	PrintSummary(Summary{
		Version:   "1.2.0",
		Commits:   10,
		BumpLevel: "minor",
		Duration:  5 * time.Second,
	})
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	s := buf.String()
	if !strings.Contains(s, "1.2.0") {
		t.Error("summary should contain version")
	}
	if !strings.Contains(s, "Release Summary") {
		t.Error("summary should contain heading")
	}
	if !strings.Contains(s, "minor") {
		t.Error("summary should contain bump level")
	}
}

func TestPrintSummaryWithTag(t *testing.T) {
	original := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = original }()

	PrintSummary(Summary{
		Version: "1.2.0",
		Commits: 5,
		Tag:     "1.2.0",
		Pushed:  true,
	})
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	s := buf.String()
	if !strings.Contains(s, "Tagged:") {
		t.Error("summary should contain Tagged label")
	}
	if !strings.Contains(s, "Pushed:") {
		t.Error("summary should contain Pushed label")
	}
}
