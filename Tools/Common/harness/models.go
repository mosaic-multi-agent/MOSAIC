package harness

// ModelCatalog is one harness's model data: the identifiers it accepts, and
// how to describe their shape to a person. A harness having a ModelCatalog
// does not make it CLI-backed or spawnable.
type ModelCatalog struct {
	// HarnessID is the harness identity this catalog belongs to: a
	// CLIHarness.ID for CLI-backed harnesses, or another builtin harness ID
	// (e.g. "vscode-ghcp") for non-CLI entries.
	HarnessID string

	// IDs are the valid model identifiers, in stable declared order — the
	// order a selection UI presents and a usage text lists.
	IDs []string

	// FormatHint is human-readable guidance on the identifier shape, e.g.
	// "provider/model-id". Display only: nothing validates against it, and
	// nothing switches on it.
	FormatHint string
}

// modelCatalogs is the single declaration of every builtin harness's model
// data. CLI-backed harnesses come first, in the same stable order as
// cliHarnesses; non-CLI entries (e.g. "vscode-ghcp") follow.
//
// Every CLI-backed harness must have an entry; the coverage test
// TestModelCatalog_EveryIdentityCatalogEntryHasModelData enforces that at test
// time. Non-CLI entries are allowed, and membership here does not make a
// harness spawnable.
var modelCatalogs = []ModelCatalog{
	{
		HarnessID:  HarnessIDClaudeCode,
		FormatHint: "claude-<model>-<version>",
		IDs: []string{
			"claude-opus-5-5",
			"claude-sonnet-5-5",
			"claude-haiku-5-5",
			"claude-fable-5-1",
			"claude-fable-5",
			"claude-opus-5",
			"claude-sonnet-5",
			"claude-opus-4-8",
			"claude-opus-4-7",
			"claude-opus-4-6",
			"claude-sonnet-4-6",
			"claude-haiku-4-5",
		},
	},
	{
		HarnessID:  HarnessIDOpenCode,
		FormatHint: "provider/model-id",
		IDs: []string{
			"github-copilot/claude-opus-5.5",
			"github-copilot/claude-sonnet-5.5",
			"github-copilot/claude-haiku-5.5",
			"github-copilot/claude-fable-5-1",
			"github-copilot/claude-fable-5",
			"github-copilot/claude-sonnet-5",
			"github-copilot/claude-opus-5",
			"github-copilot/claude-opus-4.8",
			"github-copilot/claude-sonnet-4.6",
			"github-copilot/claude-haiku-4.5",
			"openai/gpt-6.1-sol",
			"openai/gpt-6-sol",
			"openai/gpt-6-astra",
			"openai/gpt-6-luna",
			"openai/gpt-5.6-luna",
			"openai/gpt-5.6-terra",
			"openai/gpt-5.6-sol",
			"anthropic/claude-opus-5-5",
			"anthropic/claude-sonnet-5-5",
			"anthropic/claude-fable-5",
			"anthropic/claude-sonnet-5",
			"anthropic/claude-opus-5",
			"anthropic/claude-opus-4-8",
			"anthropic/claude-opus-4-7",
			"anthropic/claude-opus-4-6",
			"anthropic/claude-sonnet-4-6",
			"anthropic/claude-sonnet-4-5",
			"anthropic/claude-haiku-4-5",
		},
	},
	// GHCP CLI and VS Code GHCP both use the GitHub Copilot provider, so the two
	// entries must offer the same model set in the same order. A test enforces
	// this by converting each GHCP CLI ID to its VS Code display name.
	{
		HarnessID:  HarnessIDGHCPCLI,
		FormatHint: "claude-<model>-<version> or gpt-<version>",
		IDs: []string{
			"claude-opus-5.5",
			"claude-sonnet-5.5",
			"claude-haiku-5.5",
			"claude-fable-5.1",
			"claude-fable-5",
			"claude-opus-5",
			"claude-sonnet-5",
			"claude-opus-4.8",
			"claude-opus-4.8-fast",
			"claude-sonnet-4.6",
			"claude-haiku-4-5",
			"gpt-6.1-sol",
			"gpt-6-astra",
			"gpt-6-sol",
			"gpt-6-luna",
			"gpt-5.6-sol",
			"gpt-5.6-terra",
			"gpt-5.6-luna",
			"gpt-5.5",
			"gpt-5.4",
			"gpt-5.4-mini",
			"gpt-5.4-nano",
			"gpt-5.3-codex",
			"gpt-5-mini",
			"gemini-3.8-flash",
			"gemini-3.7-flash",
			"mai-code-1.1-flash",
			"kimi-k3",
			"grok-4.7",
			"grok-4.6",
			"grok-4.5",
		},
	},
	{
		HarnessID:  "vscode-ghcp",
		FormatHint: "display-name",
		IDs: []string{
			"Claude Opus 5.5",
			"Claude Sonnet 5.5",
			"Claude Haiku 5.5",
			"Claude Fable 5.1",
			"Claude Fable 5",
			"Claude Opus 5",
			"Claude Sonnet 5",
			"Claude Opus 4.8",
			"Claude Opus 4.8 Fast",
			"Claude Sonnet 4.6",
			"Claude Haiku 4.5",
			"Gpt 6.1 Sol",
			"Gpt 6 Astra",
			"Gpt 6 Sol",
			"Gpt 6 Luna",
			"Gpt 5.6 Sol",
			"Gpt 5.6 Terra",
			"Gpt 5.6 Luna",
			"Gpt 5.5",
			"Gpt 5.4",
			"Gpt 5.4 Mini",
			"Gpt 5.4 Nano",
			"Gpt 5.3 Codex",
			"Gpt 5 Mini",
			"Gemini 3.8 Flash",
			"Gemini 3.7 Flash",
			"Mai Code 1.1 Flash",
			"Kimi K3",
			"Grok 4.7",
			"Grok 4.6",
			"Grok 4.5",
		},
	},
}

// copyIDs returns a fresh copy of ids so a caller mutating the returned slice
// cannot corrupt the catalog's own backing array.
func copyIDs(ids []string) []string {
	if ids == nil {
		return nil
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

// ModelCatalogs returns every catalog entry as fresh values the caller may not
// corrupt for the next caller. Entries whose HarnessID is a CLI-backed harness
// come first, in CLIHarnesses order; non-CLI entries (e.g. "vscode-ghcp")
// follow in declared order. Membership does not make a harness spawnable;
// callers needing spawnable harnesses filter by IsCLIHarness or their own
// predicate.
func ModelCatalogs() []ModelCatalog {
	out := make([]ModelCatalog, len(modelCatalogs))
	for i, e := range modelCatalogs {
		out[i] = ModelCatalog{
			HarnessID:  e.HarnessID,
			FormatHint: e.FormatHint,
			IDs:        copyIDs(e.IDs),
		}
	}
	return out
}

// LookupModelCatalog returns the model catalog for harnessID, CLI-backed or
// not. The second result distinguishes "no catalog entry" from an entry; it
// never panics. The returned IDs slice is a fresh copy.
func LookupModelCatalog(harnessID string) (ModelCatalog, bool) {
	for _, e := range modelCatalogs {
		if e.HarnessID == harnessID {
			return ModelCatalog{
				HarnessID:  e.HarnessID,
				FormatHint: e.FormatHint,
				IDs:        copyIDs(e.IDs),
			}, true
		}
	}
	return ModelCatalog{}, false
}

// ModelIDs returns harnessID's valid model identifiers as a fresh slice, or
// nil when the catalog has no entry for harnessID. It is the convenience
// form for a caller that only needs the list.
func ModelIDs(harnessID string) []string {
	cat, ok := LookupModelCatalog(harnessID)
	if !ok {
		return nil
	}
	return cat.IDs
}

// IsModelID reports whether modelID is valid for harnessID. It is false for
// an unknown harness, for an unknown model, and for a model that is valid for
// a different harness.
func IsModelID(harnessID, modelID string) bool {
	cat, ok := LookupModelCatalog(harnessID)
	if !ok {
		return false
	}
	for _, id := range cat.IDs {
		if id == modelID {
			return true
		}
	}
	return false
}
