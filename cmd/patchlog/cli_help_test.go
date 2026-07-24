package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestPrimaryHelpShowsCoreContractWithoutAdvancedFlags(t *testing.T) {
	var output bytes.Buffer
	_, _, err := parseCLI([]string{"--help"}, &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseCLI --help error = %v, want flag.ErrHelp", err)
	}
	help := output.String()
	for _, expected := range []string{
		"patchlog release --dry-run",
		"release prepare",
		"release finalize",
		"--plan-json",
		"Advanced command reference:",
	} {
		if !strings.Contains(help, expected) {
			t.Errorf("primary help does not contain %q", expected)
		}
	}
	for _, advanced := range []string{"--gamify", "--ai-enhance", "--confluence", "--metrics"} {
		if strings.Contains(help, advanced) {
			t.Errorf("primary help unexpectedly contains advanced flag %q", advanced)
		}
	}
}
