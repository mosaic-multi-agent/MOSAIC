package app_test

// rebuild_carry_test.go covers what the deployed-agent rebuild carries unchanged from the
// source file: the body, the four stamps where present (and none invented where absent), the
// protocol and bundle regions with their versions, workflow tables, infrastructure
// declarations, and project and custom regions. Fixtures are the fully stamped and the
// stamp-less deployed agent of each source harness, rebuilt over the four bounded pairs.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// carriedMarkers are the texts of the fully stamped fixtures that must reach the output
// exactly once, whatever the pair.
var carriedMarkers = []string{
	"BODY-IDENTITY-TEXT",
	"CARRIED-AUTHORITY-TEXT",
	"CARRIED-CLOSING-TEXT",
	"CARRIED-ERROR-COMMON-TEXT",
	"CARRIED-PHILOSOPHY-TEXT",
	"CARRIED-PROTOCOL-TEXT",
	"NESTED-CUSTOM-IN-PROTOCOL",
	"CUSTOM-REGION-TEXT",
	"PROJECT-REGION-TEXT",
	"| 1 | WORK | code-reviewer |",
	"| commit | ITERATION_END | - | continue | Commits completed work |",
}

func TestRebuild_FullyStamped_CarriesBodyAndRegionsOnce(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false))

			for _, marker := range carriedMarkers {
				assertOnce(t, out, marker)
			}
		})
	}
}

func TestRebuild_FullyStamped_CarriesRegionVersionsFromSource(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false))

			assertOnce(t, out, `<CommunicationProtocol type="managed" version="0.9">`)
			assertOnce(t, out, `<Workflow type="managed" name="demo-flow" version="4.0">`)
			assertOnce(t, out, `<InfrastructureAgent type="core" name="commit-manager-git" version="2.5.0">`)
		})
	}
}

func TestRebuild_FullyStamped_CarriesAllFourStampsUnchanged(t *testing.T) {
	want := map[string]string{
		"mosaic_id":             "55",
		"mosaic_version":        "2.3.0",
		"mosaic_role":           "subagent",
		"mosaic_bundle_version": "1.5.0",
	}
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, p, false))

			for key, value := range want {
				got, ok := rbScalar(t, out, key)
				if !ok || got != value {
					t.Errorf("%s: want %q carried from the source, got %q (present=%v)", key, value, got, ok)
				}
			}
			if res.Stamps.Version != "2.3.0" {
				t.Errorf("Stamps.Version: want the carried 2.3.0, got %q", res.Stamps.Version)
			}
		})
	}
}

func TestRebuild_PartiallyStamped_CarriesOnlyTheStampsThatArePresent(t *testing.T) {
	versionOnly := withSourceEdit(func(s string) string {
		return removeLine("mosaic_bundle_version:")(removeLine("mosaic_role:")(s))
	})
	bundleAndRoleOnly := withSourceEdit(removeLine("mosaic_version:"))
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, outVersion := rbRebuild(t, rbInput(t, p, false, versionOnly))
			_, outOthers := rbRebuild(t, rbInput(t, p, false, bundleAndRoleOnly))

			if got, _ := rbScalar(t, outVersion, "mosaic_version"); got != "2.3.0" {
				t.Errorf("mosaic_version: want the carried 2.3.0, got %q", got)
			}
			for _, key := range []string{"mosaic_role", "mosaic_bundle_version"} {
				if rbHasKey(t, outVersion, key) {
					t.Errorf("%s must not be invented next to a lone mosaic_version:\n%s", key, outVersion)
				}
			}
			if rbHasKey(t, outOthers, "mosaic_version") {
				t.Errorf("mosaic_version must not be invented for a source without it:\n%s", outOthers)
			}
			if got, _ := rbScalar(t, outOthers, "mosaic_bundle_version"); got != "1.5.0" {
				t.Errorf("mosaic_bundle_version: want the carried 1.5.0, got %q", got)
			}
		})
	}
}

func TestRebuild_FullyStamped_CarriesDescription(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false))

			if got, _ := rbScalar(t, out, "description"); got != "Reviews source code for quality" {
				t.Errorf("description: want the source value, got %q", got)
			}
		})
	}
}

func TestRebuild_FullyStamped_VersionsNeverUpgradedToCatalog(t *testing.T) {
	newer := MockCatalogAgentLookup{
		ByID: map[string]domain.Agent{"55": {Key: "code-reviewer", NumericID: "55", Version: "9.9.9", Role: domain.RoleSubagent}},
	}
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false, withCatalogLookup(newer)))

			if got, _ := rbScalar(t, out, "mosaic_version"); got != "2.3.0" {
				t.Errorf("mosaic_version: want the source's 2.3.0, got %q", got)
			}
			assertOnce(t, out, `<CommunicationProtocol type="managed" version="0.9">`)
		})
	}
}

func TestRebuild_FullyStamped_AgentAbsentFromCatalogStillCarriesContent(t *testing.T) {
	empty := MockCatalogAgentLookup{}
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false, withCatalogLookup(empty)))

			for _, marker := range carriedMarkers {
				assertOnce(t, out, marker)
			}
			if got, _ := rbScalar(t, out, "mosaic_version"); got != "2.3.0" {
				t.Errorf("mosaic_version: want 2.3.0, got %q", got)
			}
		})
	}
}

func TestRebuild_Stampless_CarriesBodyAndCustomRegion(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, true))

			assertOnce(t, out, "STAMPLESS-BODY-TEXT")
			assertOnce(t, out, "STAMPLESS-CUSTOM-TEXT")
		})
	}
}

func TestRebuild_Stampless_InventsNoSourceStamp(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, p, true))

			for _, key := range []string{"mosaic_id", "mosaic_version", "mosaic_role", "mosaic_bundle_version"} {
				if rbHasKey(t, out, key) {
					t.Errorf("%s must not be invented for a source without it:\n%s", key, out)
				}
			}
			if res.Stamps.Version != "" {
				t.Errorf("Stamps.Version: want empty for a source without mosaic_version, got %q", res.Stamps.Version)
			}
		})
	}
}
