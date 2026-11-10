package app_test

// rebuild_contract_test.go covers the error path, determinism, tool-mappings stamp and
// name carry of the deployed-agent rebuild.

import (
	"bytes"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestRebuild_UnparseableOrFrontmatterlessSourceReturnsError(t *testing.T) {
	cases := map[string]string{
		"unterminated frontmatter": "---\ndescription: [unclosed\n",
		"no frontmatter block":     "Just a body without any frontmatter.\n",
		"empty":                    "",
	}
	// Control: the same pipeline succeeds on a valid source, so the errors below come from the source.
	rbRebuild(t, rbInput(t, rbPairs[0], false))
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			in := rbInput(t, rbPairs[0], false)
			in.Source = []byte(src)

			_, err := app.RebuildDeployedAgent(in)

			if err == nil {
				t.Errorf("expected an error for source %q", src)
			}
		})
	}
}

func TestRebuild_IdenticalInputGivesIdenticalOutputAndUniqueGaps(t *testing.T) {
	unresolved := withModelSelection(domain.ModelSelection{Origin: domain.OriginUnresolved})
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			first, _ := rbRebuild(t, rbInput(t, p, false, unresolved))
			second, _ := rbRebuild(t, rbInput(t, p, false, unresolved))

			if !bytes.Equal(first.Output, second.Output) {
				t.Errorf("output differs between identical calls")
			}
			seen := map[domain.Gap]int{}
			for _, g := range first.Gaps {
				seen[g]++
				if seen[g] > 1 {
					t.Errorf("duplicate gap: %+v", g)
				}
			}
		})
	}
}

func TestRebuild_ToolMappingsVersionIsStamped(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			in := rbInput(t, p, false)
			in.ToolMappingsVersion = "tmv-abc123"

			res, out := rbRebuild(t, in)

			if got, _ := rbScalar(t, out, "mosaic_tool_mappings_version"); got != "tmv-abc123" {
				t.Errorf("mosaic_tool_mappings_version: want tmv-abc123, got %q", got)
			}
			if res.Stamps.ToolMappingsVersion != "tmv-abc123" {
				t.Errorf("Stamps.ToolMappingsVersion: want tmv-abc123, got %q", res.Stamps.ToolMappingsVersion)
			}
		})
	}
}

func TestRebuild_CarriesSourceNameDistinctFromFileKey(t *testing.T) {
	rename := withSourceEdit(replaceText("name: code-reviewer", "name: Display Reviewer"))
	for _, p := range []rbPair{rbPairs[2], rbPairs[3]} {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false, rename))

			if got, _ := rbScalar(t, out, "name"); got != "Display Reviewer" {
				t.Errorf("name: want the source value, got %q", got)
			}
		})
	}
}
