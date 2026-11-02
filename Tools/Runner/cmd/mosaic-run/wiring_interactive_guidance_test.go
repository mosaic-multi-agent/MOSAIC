package main

// The interactive composition seam hands every session the guidance file store
// and the shared live guidance holder, as the non-interactive path does.

import (
	"testing"

	"mosaic-run/internal/session"
	"mosaic-run/internal/tui/screens/runconfig"
)

func TestInteractiveWiring_Deps_CarryTheGuidanceStoreAndTheSharedHolder(t *testing.T) {
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	in.Guidance = session.NewStandingGuidance()
	w := buildInteractiveWiring(in)

	deps := w.NewDeps(t.TempDir(), true, "", runconfig.ConfigSelection{})

	if deps.GuidanceStore == nil {
		t.Error("want Deps.GuidanceStore wired, got nil: a TUI run would never save or reuse guidance")
	}
	if deps.Guidance != in.Guidance {
		t.Error("want Deps.Guidance to be the holder shared with the TUI, got another instance")
	}
}

func TestInteractiveWiring_EverySessionShareOneGuidanceHolder(t *testing.T) {
	in, _ := recordedInput(t, resolvedTestIdentity(t))
	in.Guidance = session.NewStandingGuidance()
	w := buildInteractiveWiring(in)

	first := w.NewDeps(t.TempDir(), true, "", runconfig.ConfigSelection{})
	second := w.NewDeps(t.TempDir(), false, "", runconfig.ConfigSelection{})

	if first.Guidance != second.Guidance || first.Guidance == nil {
		t.Error("want every session built by the seam to share the one guidance holder")
	}
}
