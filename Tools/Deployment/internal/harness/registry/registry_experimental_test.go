package registry_test

// Tests that the harness list reflects the experimental declaration of a harness descriptor
// for the descriptor-only and external tiers.
//
// Coverage:
//   - A harness whose descriptor declares experimental is listed with that reason.
//   - A harness whose descriptor declares nothing is listed as not experimental.
//   - Resolve(id).Ref() reports the same reason as the listed entry.
//   - An external harness without the opt-in is still listed with its reason, and listing
//     does not make it usable.
//   - A harness whose declaration is invalid is listed unusable, never as a usable harness.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

const experimentalYAML = "experimental:\n  reason: \"Only agents deploy.\"\n"

func TestList_DeclaredExperimental_ListedWithReason(t *testing.T) {
	for _, th := range tierHarnesses() {
		t.Run(th.name, func(t *testing.T) {
			id := uniqueID(t, "exp")
			reg := th.discover(t, th.setup(t, id, experimentalYAML))

			resolveTier(t, reg, id) // closes any subprocess so the temp dir can be removed
			ref, ok := findRef(reg.List(), id)

			if !ok {
				t.Fatalf("harness %q not listed", id)
			}
			if ref.ExperimentalReason != "Only agents deploy." {
				t.Errorf("listed ExperimentalReason = %q, want the declared reason", ref.ExperimentalReason)
			}
			if !ref.Usable {
				t.Errorf("experimental harness listed unusable: %q", ref.UnusableReason)
			}
		})
	}
}

func TestList_NoDeclaration_ListedAsNotExperimental(t *testing.T) {
	for _, th := range tierHarnesses() {
		t.Run(th.name, func(t *testing.T) {
			id := uniqueID(t, "plain")
			reg := th.discover(t, th.setup(t, id, ""))

			resolveTier(t, reg, id) // closes any subprocess so the temp dir can be removed
			ref, ok := findRef(reg.List(), id)

			if !ok {
				t.Fatalf("harness %q not listed", id)
			}
			if ref.ExperimentalReason != "" {
				t.Errorf("ExperimentalReason = %q, want empty", ref.ExperimentalReason)
			}
		})
	}
}

func TestResolve_DeclaredExperimental_RefMatchesListedEntry(t *testing.T) {
	for _, th := range tierHarnesses() {
		t.Run(th.name, func(t *testing.T) {
			id := uniqueID(t, "resolve")
			reg := th.discover(t, th.setup(t, id, experimentalYAML))

			m := resolveTier(t, reg, id)

			if m.Ref().ExperimentalReason != "Only agents deploy." {
				t.Errorf("Ref().ExperimentalReason = %q, want the declared reason", m.Ref().ExperimentalReason)
			}
			if m.Descriptor().ExperimentalReason != "Only agents deploy." {
				t.Errorf("Descriptor().ExperimentalReason = %q, want the declared reason", m.Descriptor().ExperimentalReason)
			}
		})
	}
}

// Listing an external harness that is not opted in must not spawn its subprocess; the reason
// comes from the descriptor YAML the host holds.
func TestList_ExternalWithoutOptIn_StillCarriesDeclaredReason(t *testing.T) {
	id := uniqueID(t, "noptin-exp")
	root := tierHarnesses()[1].setup(t, id, experimentalYAML)
	reg, err := registry.Discover(registry.Options{MosaicRoot: root, AllowExternal: false})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	ref, ok := findRef(reg.List(), id)

	if !ok {
		t.Fatalf("harness %q not listed", id)
	}
	if ref.Tier != domain.TierExternal || ref.Usable || !ref.RequiresOptIn {
		t.Fatalf("ref = %+v, want unusable external harness awaiting opt-in", ref)
	}
	if ref.ExperimentalReason != "Only agents deploy." {
		t.Errorf("ExperimentalReason = %q, want the declared reason", ref.ExperimentalReason)
	}
}

func TestList_InvalidExperimentalDeclaration_ListedUnusable(t *testing.T) {
	for _, th := range tierHarnesses() {
		t.Run(th.name, func(t *testing.T) {
			id := uniqueID(t, "bad")
			reg := th.discover(t, th.setup(t, id, "experimental:\n  reason: \"   \"\n"))

			ref, ok := findRef(reg.List(), id)

			if !ok {
				t.Fatalf("harness %q not listed", id)
			}
			if ref.Usable {
				t.Errorf("harness with blank experimental reason listed usable: %+v", ref)
			}
			if ref.UnusableReason == "" {
				t.Error("UnusableReason is empty, want the load error")
			}
			// Only the new validation names this field; the generic unknown-field error does not.
			if !strings.Contains(ref.UnusableReason, "experimental.reason") {
				t.Errorf("UnusableReason = %q, want it to name experimental.reason", ref.UnusableReason)
			}
			if ref.ExperimentalReason != "" {
				t.Errorf("load-error entry ExperimentalReason = %q, want empty", ref.ExperimentalReason)
			}
		})
	}
}
