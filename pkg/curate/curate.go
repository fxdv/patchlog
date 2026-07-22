// Package curate provides an interactive terminal UI for curating release notes before publishing.
package curate

import (
	"fmt"

	"github.com/fxdv/patchlog/pkg/render"
)

func Run(report render.Report) (render.Report, bool, error) {
	term, err := SetupTerminal()
	if err != nil {
		return report, false, fmt.Errorf("terminal setup: %w", err)
	}
	defer RestoreTerminal(term)

	w, h := term.Size()
	state := NewState(report, w, h)

	ClearScreen()
	fmt.Print(state.Render())

	for {
		ev, err := term.ReadKey()
		if err != nil {
			continue
		}

		action := state.HandleKey(ev)
		w, h = term.Size()
		state.Width = w
		state.Height = h

		switch action {
		case "quit":
			ClearScreen()
			return report, false, nil
		case "publish":
			ClearScreen()
			return state.FilteredReport(), true, nil
		case "save":
			ClearScreen()
			return state.FilteredReport(), false, nil
		}

		if state.Mode == ModeHelp {
			ClearScreen()
			fmt.Print(state.Render())
			continue
		}

		ClearScreen()
		fmt.Print(state.Render())
	}
}
