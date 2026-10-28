package screens_test

// The mapping from domain facts to the palette's signal roles, and the text
// markers that keep meaning when colour is off.

import (
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	good    = tuicommon.RoleSignalGood
	warning = tuicommon.RoleSignalWarning
	bad     = tuicommon.RoleSignalBad
	muted   = tuicommon.RoleSignalMuted
)

func closedWith(code string) domain.Invocation {
	return domain.Invocation{State: domain.InvocationClosed, StatusCode: code, EndEvents: 1}
}

func TestStatusRole_MapsEveryStatusCodeAndOpenState(t *testing.T) {
	cases := []struct {
		name string
		inv  domain.Invocation
		want tuicommon.StyleRole
	}{
		{"SUCCESS", closedWith("SUCCESS"), good},
		{"COMPLETED_NEEDS_ACTION", closedWith("COMPLETED_NEEDS_ACTION"), warning},
		{"PARTIALLY_DONE", closedWith("PARTIALLY_DONE"), warning},
		{"NEEDS_CLARIFICATION", closedWith("NEEDS_CLARIFICATION"), warning},
		{"unknown code", closedWith("SOMETHING_NEW"), warning},
		{"closed without status", closedWith(""), warning},
		{"open", domain.Invocation{State: domain.InvocationOpen}, warning},
		{"CAPABILITY_EXCEEDED", closedWith("CAPABILITY_EXCEEDED"), bad},
		{"BLOCKED", closedWith("BLOCKED"), bad},
	}
	for _, c := range cases {
		if got := screens.StatusRole(c.inv); got != c.want {
			t.Errorf("StatusRole(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestStatusGroupRole_UsesTheSameMappingAsInvocations(t *testing.T) {
	cases := []struct {
		name  string
		group domain.StatusGroup
		want  tuicommon.StyleRole
	}{
		{"SUCCESS", domain.StatusGroup{Kind: domain.StatusReported, Code: "SUCCESS"}, good},
		{"COMPLETED_NEEDS_ACTION", domain.StatusGroup{Kind: domain.StatusReported, Code: "COMPLETED_NEEDS_ACTION"}, warning},
		{"PARTIALLY_DONE", domain.StatusGroup{Kind: domain.StatusReported, Code: "PARTIALLY_DONE"}, warning},
		{"NEEDS_CLARIFICATION", domain.StatusGroup{Kind: domain.StatusReported, Code: "NEEDS_CLARIFICATION"}, warning},
		{"unknown code", domain.StatusGroup{Kind: domain.StatusReported, Code: "WEIRD_CODE"}, warning},
		{"no status", domain.StatusGroup{Kind: domain.StatusNone}, warning},
		{"open", domain.StatusGroup{Kind: domain.StatusOpen}, warning},
		{"CAPABILITY_EXCEEDED", domain.StatusGroup{Kind: domain.StatusReported, Code: "CAPABILITY_EXCEEDED"}, bad},
		{"BLOCKED", domain.StatusGroup{Kind: domain.StatusReported, Code: "BLOCKED"}, bad},
	}
	for _, c := range cases {
		if got := screens.StatusGroupRole(c.group); got != c.want {
			t.Errorf("StatusGroupRole(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMoneyRole_AbsentMutedUnpricedOrPartialWarningKnownIsCost(t *testing.T) {
	cases := []struct {
		name     string
		value    domain.MoneyValue
		complete bool
		want     tuicommon.StyleRole
	}{
		{"absent", domain.NoMoneyData(), true, muted},
		{"unpriced", domain.UnpricedMoney(), true, warning},
		{"unpriced partial", domain.UnpricedMoney(), false, warning},
		{"known partial total", known(500), false, warning},
		{"known complete", known(500), true, tuicommon.RoleDataCost},
		{"known zero complete", known(0), true, tuicommon.RoleDataCost},
	}
	for _, c := range cases {
		if got := screens.MoneyRole(c.value, c.complete); got != c.want {
			t.Errorf("MoneyRole(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTokensRole_AbsentIsMutedPresentZeroIsStillTokens(t *testing.T) {
	cases := []struct {
		name   string
		tokens domain.TokenCount
		want   tuicommon.StyleRole
	}{
		{"absent", domain.AbsentTokens(), muted},
		{"present", domain.Tokens(1200), tuicommon.RoleDataTokens},
		{"present zero", domain.Tokens(0), tuicommon.RoleDataTokens},
	}
	for _, c := range cases {
		if got := screens.TokensRole(c.tokens); got != c.want {
			t.Errorf("TokensRole(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBandRole_MapsEveryContextBand(t *testing.T) {
	cases := map[domain.ContextBand]tuicommon.StyleRole{
		domain.BandBelowOptimal: warning,
		domain.BandOptimal:      good,
		domain.BandAboveOptimal: warning,
		domain.BandDegraded:     bad,
	}
	for band, want := range cases {
		if got := screens.BandRole(band); got != want {
			t.Errorf("BandRole(%s) = %q, want %q", band.Label(), got, want)
		}
	}
}

func TestFindingRole_MalformedOrUnreadableDataIsBadEveryOtherKindWarns(t *testing.T) {
	badKinds := []domain.FindingKind{
		domain.FindingMalformedLine, domain.FindingTruncatedLine, domain.FindingUnreadableFile,
		domain.FindingInvalidDigest, domain.FindingUnsupportedDigestVersion,
	}
	warnKinds := []domain.FindingKind{
		domain.FindingMissingTokenUsage, domain.FindingMissingModel, domain.FindingUnknownSchema,
		domain.FindingUnrecognisedHarness, domain.FindingInvalidPricingEntry,
		domain.FindingCrossStreamRecord, domain.FindingCompactionMismatch,
	}
	for _, k := range badKinds {
		if got := screens.FindingRole(k); got != bad {
			t.Errorf("FindingRole(%s) = %q, want %q", k, got, bad)
		}
	}
	for _, k := range warnKinds {
		if got := screens.FindingRole(k); got != warning {
			t.Errorf("FindingRole(%s) = %q, want %q", k, got, warning)
		}
	}
}

func TestChangeRole_IncreaseIsBadDecreaseIsGoodOtherwiseMuted(t *testing.T) {
	cases := map[domain.ChangeDirection]tuicommon.StyleRole{
		domain.ChangeIncrease:  bad,
		domain.ChangeDecrease:  good,
		domain.ChangeUnchanged: muted,
		domain.ChangeUnknown:   muted,
	}
	for dir, want := range cases {
		if got := screens.ChangeRole(dir); got != want {
			t.Errorf("ChangeRole(%d) = %q, want %q", dir, got, want)
		}
	}
}

func TestToolErrorsRole_AnyErrorIsBadNoneIsMuted(t *testing.T) {
	cases := map[int]tuicommon.StyleRole{0: muted, 1: bad, 25: bad}
	for errors, want := range cases {
		if got := screens.ToolErrorsRole(errors); got != want {
			t.Errorf("ToolErrorsRole(%d) = %q, want %q", errors, got, want)
		}
	}
}

// With colour off the markers are the only carrier of meaning, so every one of
// them must still be in the text of the view.
func TestMarkers_RemainAsTextInTheViewsWithColourOff(t *testing.T) {
	colourOff(t)
	run := overviewRun()
	run.Provisional = true
	run.UnpricedModels = []domain.PricingKey{{Model: "model-b"}}
	run.Overview.Totals.Money.Total = known(500)
	run.Overview.Totals.Money.Complete = false
	run.Overview.Totals.AssumedTierCacheWrite = 4321
	run.Overview.Subagents.MissingUsage = 1
	run.Overview.Subagents.Open = 2
	run.Overview.OrchestratorShare = domain.Ratio{}
	run.Totals = run.Overview.Totals

	view := screens.NewOverviewScreen(run, 140, 60, paletteStyles()).View(140, 60)

	assertContains(t, view, screens.ProvisionalLabel, screens.UnpricedMarker, "(partial)",
		screens.AssumedTierLabel, screens.MissingUsageLabel, screens.OpenLabel, screens.AbsentMarker)
	assertNotContains(t, view, "$0.00")
}
