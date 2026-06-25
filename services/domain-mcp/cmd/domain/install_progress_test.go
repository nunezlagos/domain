




package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallProgress_StartStep_Format(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(3, &buf)

	p.StartStep("Detecting state")
	p.StartStep("Backing up")
	p.StartStep("Migrating")

	out := buf.String()
	require.Contains(t, out, "[1/3] Detecting state")
	require.Contains(t, out, "[2/3] Backing up")
	require.Contains(t, out, "[3/3] Migrating")
}

func TestInstallProgress_EndStep_OK(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(2, &buf)
	p.StartStep("Backups")
	p.EndStep(StepOK, "3 files backed up")

	out := buf.String()
	require.Contains(t, out, "✓")
	require.Contains(t, out, "3 files backed up")
}

func TestInstallProgress_EndStep_Warning(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(1, &buf)
	p.StartStep("Init")
	p.EndStep(StepWarning, ".env not found, skipped")

	require.Contains(t, buf.String(), "⚠")
	require.Contains(t, buf.String(), ".env not found, skipped")
}

func TestInstallProgress_EndStep_WithoutStartStep_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(3, &buf)




	require.NotPanics(t, func() {
		p.EndStep(StepOK, "orphan")
	})


	p.Summary()
	require.Contains(t, buf.String(), "total=0")
	require.NotContains(t, buf.String(), "ok=1",
		"orphan EndStep must NOT be counted in summary")
}

func TestInstallProgress_Summary_CountsCorrect(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(4, &buf)
	p.StartStep("a"); p.EndStep(StepOK, "ok")
	p.StartStep("b"); p.EndStep(StepOK, "ok")
	p.StartStep("c"); p.EndStep(StepSkipped, "no change")
	p.StartStep("d"); p.EndStep(StepFailed, "docker not running")

	p.Summary()

	out := buf.String()
	require.Contains(t, out, "ok=2")
	require.Contains(t, out, "skipped=1")
	require.Contains(t, out, "failed=1")
	require.Contains(t, out, "warning=0")
}

func TestInstallProgress_Summary_ListsFailedSteps(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(2, &buf)
	p.StartStep("Migrate"); p.EndStep(StepFailed, "schema mismatch")
	p.StartStep("Seed"); p.EndStep(StepOK, "ok")

	p.Summary()

	out := buf.String()
	require.Contains(t, out, "Failed steps:")
	require.Contains(t, out, "Migrate")
	require.Contains(t, out, "schema mismatch")
}

func TestInstallProgress_Summary_ZeroSteps(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(5, &buf)

	p.Summary()
	require.Contains(t, buf.String(), "total=0")
}

func TestInstallProgress_Steps_ReturnsCopy(t *testing.T) {
	var buf bytes.Buffer
	p := NewInstallProgress(1, &buf)
	p.StartStep("x")
	p.EndStep(StepOK, "ok")

	steps := p.Steps()
	require.Len(t, steps, 1)
	require.Equal(t, "x", steps[0].Name)
	require.Equal(t, StepOK, steps[0].Status)
	require.Equal(t, "ok", steps[0].Summary)


	steps[0].Name = "mutated"
	steps2 := p.Steps()
	require.Equal(t, "x", steps2[0].Name, "Steps() must return a defensive copy")
}

func TestStatusGlyph_AllStatuses(t *testing.T) {


	cases := map[StepStatus]string{
		StepOK:      "✓",
		StepSkipped: "·",
		StepWarning: "⚠",
		StepFailed:  "✗",
	}
	for status, want := range cases {
		require.Equal(t, want, statusGlyph(status), "glyph for %s", status)
	}

	require.Equal(t, "?", statusGlyph(StepStatus("unknown")))
}

func TestPrintBanner_ContainsTitle(t *testing.T) {
	var buf bytes.Buffer
	printBanner(&buf)
	out := buf.String()
	require.Contains(t, out, "Domain Install Wizard")
	require.Contains(t, out, "issue-01.10")
	require.True(t, strings.HasPrefix(out, "="), "banner debe empezar con ====")
}
