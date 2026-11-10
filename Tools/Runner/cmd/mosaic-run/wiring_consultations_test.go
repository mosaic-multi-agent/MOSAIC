package main

// The interactive wiring connects every session it builds to the TUI as the
// consultation observer; without it the progress screen would never hear of a
// consultation. The non-interactive routing deps carry no observer, so CLI
// output cannot gain consultation lines.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/runconfig"
)

func TestInteractiveWiring_Consultations_TheSessionReportsToTheTUIProgramRef(t *testing.T) {
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	w := buildInteractiveWiring(in)

	deps := w.NewDeps(testRunFolder(t, "20260101T000000Z-0031"), true, "", runconfig.ConfigSelection{})

	if deps.Consultations == nil {
		t.Fatal("Deps.Consultations is nil: the progress screen would never show a consultation")
	}
	if deps.Consultations != domain.ConsultationObserver(in.ProgramRef) {
		t.Errorf("Deps.Consultations = %v, want the ProgramRef the TUI program is wired to (%p)", deps.Consultations, in.ProgramRef)
	}
}

func TestBuildDeps_LeavesTheConsultationObserverUnset(t *testing.T) {
	deps := buildDepsFromTransports(nil, nil, nil, nil)

	if deps.Consultations != nil {
		t.Errorf("buildDeps set Consultations = %v: the non-interactive path must announce nothing", deps.Consultations)
	}
}
