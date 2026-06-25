











package main

import (
	"fmt"
	"io"
	"strings"
)

// InstallStep representa un step individual del wizard.
type InstallStep struct {
	Number  int    // 1-based index
	Name    string // "Backing up configs"
	Status  StepStatus
	Summary string // "3 files backed up"
}

type StepStatus string

const (
	StepOK       StepStatus = "ok"
	StepSkipped  StepStatus = "skipped"
	StepWarning  StepStatus = "warning"
	StepFailed   StepStatus = "failed"
)

// InstallProgress es el reporter del wizard. No es thread-safe;
// cada goroutine de step debe usar su propio reporter o coordinar.
type InstallProgress struct {
	Total  int           // total de steps esperados
	writer io.Writer     // donde escribir (stderr)
	steps  []InstallStep // acumulador
}

func NewInstallProgress(total int, w io.Writer) *InstallProgress {
	return &InstallProgress{Total: total, writer: w}
}

// StartStep emite "[N/Total] Name\n" y registra el step como in-progress.
func (p *InstallProgress) StartStep(name string) {
	n := len(p.steps) + 1
	p.steps = append(p.steps, InstallStep{Number: n, Name: name})
	fmt.Fprintf(p.writer, "[%d/%d] %s\n", n, p.Total, name)
}

// EndStep actualiza el status del step actual y emite el resultado.
func (p *InstallProgress) EndStep(status StepStatus, summary string) {
	if len(p.steps) == 0 {
		return // EndStep sin StartStep previo
	}
	last := &p.steps[len(p.steps)-1]
	last.Status = status
	last.Summary = summary
	fmt.Fprintf(p.writer, "    %s %s\n", statusGlyph(status), summary)
}

// Summary imprime el resumen final con conteo de OK/skipped/warning/failed.
func (p *InstallProgress) Summary() {
	counts := map[StepStatus]int{}
	for _, s := range p.steps {
		counts[s.Status]++
	}
	fmt.Fprintln(p.writer, "")
	fmt.Fprintln(p.writer, "Summary:")
	fmt.Fprintf(p.writer, "  ok=%d skipped=%d warning=%d failed=%d (total=%d)\n",
		counts[StepOK], counts[StepSkipped], counts[StepWarning], counts[StepFailed], len(p.steps))
	if counts[StepFailed] > 0 {
		fmt.Fprintln(p.writer, "")
		fmt.Fprintln(p.writer, "Failed steps:")
		for _, s := range p.steps {
			if s.Status == StepFailed {
				fmt.Fprintf(p.writer, "  [%d] %s: %s\n", s.Number, s.Name, s.Summary)
			}
		}
	}
}

// Steps retorna la lista de steps (para tests y reportar bugs).
func (p *InstallProgress) Steps() []InstallStep {
	out := make([]InstallStep, len(p.steps))
	copy(out, p.steps)
	return out
}

func statusGlyph(s StepStatus) string {
	switch s {
	case StepOK:
		return "✓"
	case StepSkipped:
		return "·"
	case StepWarning:
		return "⚠"
	case StepFailed:
		return "✗"
	}
	return "?"
}

// printBanner emite el header del wizard. Es estatico asi que vive
// aca para que el codigo principal se mantenga enfocado.
func printBanner(out io.Writer) {
	fmt.Fprintln(out, strings.Repeat("=", 50))
	fmt.Fprintln(out, "  Domain Install Wizard (issue-01.10)")
	fmt.Fprintln(out, strings.Repeat("=", 50))
	fmt.Fprintln(out, "")
}
