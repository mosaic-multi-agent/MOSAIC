package harness_test

// Tests for the VS Code GHCP model catalog entry and its parity with the
// GHCP CLI entry. Both harnesses use the GitHub Copilot provider and must
// offer the same model set in the same order; the VS Code list uses display
// names, the CLI list uses lowercase hyphenated IDs.

import (
	"strings"
	"testing"
	"unicode"

	"mosaic-common/harness"
)

const vscodeGHCPHarnessID = "vscode-ghcp"

// ghcpIDToDisplayName converts a GHCP CLI model ID to its VS Code display
// name: tokens split on '-', a '-' between two digits becomes '.', existing
// dots are kept, each remaining token is Title-Cased, tokens joined by space.
func ghcpIDToDisplayName(id string) string {
	parts := strings.Split(id, "-")
	var words []string
	for _, p := range parts {
		if len(words) > 0 && p != "" {
			prev := words[len(words)-1]
			last := rune(prev[len(prev)-1])
			first := rune(p[0])
			if unicode.IsDigit(last) && unicode.IsDigit(first) {
				words[len(words)-1] = prev + "." + p
				continue
			}
		}
		words = append(words, p)
	}
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// TestGHCPIDToDisplayName_MatchesPinnedExamples guards the conversion helper
// so a wrong helper cannot make the parity test vacuous.
func TestGHCPIDToDisplayName_MatchesPinnedExamples(t *testing.T) {
	pairs := [][2]string{
		{"claude-opus-5-5", "Claude Opus 5.5"},
		{"claude-opus-4.8-fast", "Claude Opus 4.8 Fast"},
		{"gpt-6.1-sol", "Gpt 6.1 Sol"},
		{"gpt-6-astra", "Gpt 6 Astra"},
		{"kimi-k3", "Kimi K3"},
		{"mai-code-1.1-flash", "Mai Code 1.1 Flash"},
	}
	for _, p := range pairs {
		if got := ghcpIDToDisplayName(p[0]); got != p[1] {
			t.Errorf("ghcpIDToDisplayName(%q) = %q, want %q", p[0], got, p[1])
		}
	}
}

func TestLookupModelCatalog_VSCodeGHCPHasDisplayNameEntry(t *testing.T) {
	cat, ok := harness.LookupModelCatalog(vscodeGHCPHarnessID)
	if !ok {
		t.Fatalf("LookupModelCatalog(%q) returned not-found; want a catalog entry", vscodeGHCPHarnessID)
	}
	if cat.HarnessID != vscodeGHCPHarnessID {
		t.Errorf("want HarnessID %q, got %q", vscodeGHCPHarnessID, cat.HarnessID)
	}
	if len(cat.IDs) == 0 {
		t.Errorf("want at least one model ID for %q, got none", vscodeGHCPHarnessID)
	}
	if cat.FormatHint != "display-name" {
		t.Errorf("want FormatHint %q, got %q", "display-name", cat.FormatHint)
	}
}

func TestModelCatalogs_VSCodeGHCPImmediatelyFollowsGHCPCLI(t *testing.T) {
	entries := harness.ModelCatalogs()
	cliIdx := -1
	for i, e := range entries {
		if e.HarnessID == harness.HarnessIDGHCPCLI {
			cliIdx = i
		}
	}
	if cliIdx < 0 {
		t.Fatalf("no %q entry in ModelCatalogs()", harness.HarnessIDGHCPCLI)
	}
	if cliIdx+1 >= len(entries) {
		t.Fatalf("%q is the last entry; want %q immediately after it", harness.HarnessIDGHCPCLI, vscodeGHCPHarnessID)
	}
	if got := entries[cliIdx+1].HarnessID; got != vscodeGHCPHarnessID {
		t.Errorf("entry after %q is %q, want %q", harness.HarnessIDGHCPCLI, got, vscodeGHCPHarnessID)
	}
}

func TestModelCatalogs_GHCPListsHaveNoDuplicateIDs(t *testing.T) {
	for _, hid := range []string{harness.HarnessIDGHCPCLI, vscodeGHCPHarnessID} {
		cat, ok := harness.LookupModelCatalog(hid)
		if !ok {
			t.Errorf("no catalog entry for %q", hid)
			continue
		}
		seen := map[string]int{}
		for i, id := range cat.IDs {
			if first, dup := seen[id]; dup {
				t.Errorf("%q: duplicate model ID %q at positions %d and %d", hid, id, first, i)
				continue
			}
			seen[id] = i
		}
	}
}

func TestModelCatalogs_VSCodeGHCPMatchesConvertedGHCPCLIList(t *testing.T) {
	cli, ok := harness.LookupModelCatalog(harness.HarnessIDGHCPCLI)
	if !ok {
		t.Fatalf("no catalog entry for %q", harness.HarnessIDGHCPCLI)
	}
	vs, ok := harness.LookupModelCatalog(vscodeGHCPHarnessID)
	if !ok {
		t.Fatalf("no catalog entry for %q", vscodeGHCPHarnessID)
	}
	if len(vs.IDs) != len(cli.IDs) {
		t.Fatalf("%q has %d models, %q has %d; both must offer the same model set", vscodeGHCPHarnessID, len(vs.IDs), harness.HarnessIDGHCPCLI, len(cli.IDs))
	}
	for i, id := range cli.IDs {
		if want := ghcpIDToDisplayName(id); vs.IDs[i] != want {
			t.Errorf("index %d: %q entry is %q, want %q (converted from %q)", i, vscodeGHCPHarnessID, vs.IDs[i], want, id)
		}
	}
}

// TestIsCLIHarness_VSCodeGHCPIsNotCLIBacked documents that catalog membership
// does not imply spawnability.
func TestIsCLIHarness_VSCodeGHCPIsNotCLIBacked(t *testing.T) {
	if harness.IsCLIHarness(vscodeGHCPHarnessID) {
		t.Errorf("IsCLIHarness(%q) = true, want false: it has a model catalog but is not CLI-backed", vscodeGHCPHarnessID)
	}
}
