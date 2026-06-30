








package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/tui/app"
	"nunezlagos/domain/internal/tui/menu"
)

// runTUI lanza el TUI bubbletea. Retorna exit code.
func runTUI(args []string) int {


	if len(args) > 0 {
		idx := menu.IndexOf(args[0])
		if idx >= 0 {
			return runTUIFeature(idx)
		}
	}


	if !isTerminal(os.Stdin) {
		fmt.Fprintln(os.Stderr, "TUI requires a terminal. Use 'domain install', 'domain update', etc. for non-interactive mode.")
		printUsage()
		return 2
	}


	p := tea.NewProgram(app.New(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return 1
	}
	_ = finalModel
	return 0
}

// runTUIFeature lanza una feature especifica del menu (skip el menu).
func runTUIFeature(idx int) int {
	if !isTerminal(os.Stdin) {
		fmt.Fprintln(os.Stderr, "TUI requires a terminal.")
		return 2
	}
	p := tea.NewProgram(app.NewDirect(idx), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		return 1
	}
	return 0
}

// isTerminal chequea si fd es un TTY.
// Usa $TERM (heuristica) y os.Stat para evitar imports adicionales.
func isTerminal(fd *os.File) bool {


	term := os.Getenv("TERM")
	if term == "" || term == "dumb" || term == "unknown" {
		return false
	}

	info, err := fd.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
