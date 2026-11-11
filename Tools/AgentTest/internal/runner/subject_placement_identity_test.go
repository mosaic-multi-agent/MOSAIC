package runner_test

// Placement tests that separate the subject identity (what `--agent` receives
// and what the definition's `name:` must equal) from the catalogue key (how the
// subject is looked up for provisioning), plus directory-boundary and quoted
// name cases. The base requests carry the catalogue key "orchestrator".

import (
	"context"
	"strings"
	"testing"

	"mosaic-agent-test/internal/runner"
)

const (
	divergentIdentity  = "lead"
	fixtureNamedLead   = "subject_named_lead.md"
	fixtureQuotedName  = "subject_quoted_name.md"
	siblingAgentsDir   = ".claude/agents-old"
	identityDiffersMsg = "identity differs from the catalogue key"
)

// withDivergentIdentity makes the subject identity differ from the catalogue key.
func withDivergentIdentity(build func(string) runner.Request) func(string) runner.Request {
	return func(testID string) runner.Request {
		req := build(testID)
		req.Test.Definition.Subject.Identity = divergentIdentity
		return req
	}
}

func TestSubjectPlacement_NameIsMatchedAgainstTheIdentityNotTheCatalogueKey(t *testing.T) {
	for _, p := range placementPaths() {
		request := withDivergentIdentity(p.request)
		t.Run(p.name+" named after the identity proceeds", func(t *testing.T) {
			h := newHarness(t)
			p.place(t, h, fixtureNamedLead, projectAgentsRelative)

			runProceeds(t, h, request("placement-identity-ok"))
		})
		t.Run(p.name+" named after the catalogue key is a setup error", func(t *testing.T) {
			h := newHarness(t)
			p.place(t, h, fixtureMatchingName, projectAgentsRelative)

			_, err := runner.Run(context.Background(), h.Deps, request("placement-identity-key"), nil)
			if err == nil {
				t.Fatalf("Run returned no error, want a setup error: %s", identityDiffersMsg)
			}
			for _, want := range []string{divergentIdentity, placementFileName} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Run error = %q, want it to name %q", err.Error(), want)
				}
			}
			for _, c := range h.Rec.all() {
				if c == "adapter.SpawnPlan" || c == "launcher.Launch" {
					t.Errorf("%s was called although the name does not match the identity", c)
				}
			}
		})
	}
}

func TestSubjectPlacement_QuotedNameMatchingTheIdentityLetsTheRunProceed(t *testing.T) {
	for _, p := range placementPaths() {
		t.Run(p.name, func(t *testing.T) {
			h := newHarness(t)
			p.place(t, h, fixtureQuotedName, projectAgentsRelative)

			runProceeds(t, h, p.request("placement-quoted-name"))
		})
	}
}

func TestSubjectPlacement_SiblingDirectoryOfTheProjectAgentsDirectoryIsASetupError(t *testing.T) {
	for _, p := range placementPaths() {
		t.Run(p.name, func(t *testing.T) {
			h := newHarness(t)
			p.place(t, h, fixtureMatchingName, siblingAgentsDir)

			runAbortedByPlacement(t, h, p.request("placement-sibling-dir"))
		})
	}
}
