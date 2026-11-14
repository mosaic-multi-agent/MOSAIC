package app_test

// thirdformat_parity_compare_test.go holds the parity comparison of two twin observations. Each
// observation field is turned into a JSON tree, normalised (empty collections count as absent;
// for flow results the workspace root and the twin agent extension are replaced) and compared,
// so a failure prints the differing field of both sides.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// twinParityConfig holds the options of one comparison.
type twinParityConfig struct {
	ignoreProblemDetail bool
}

// twinParityOption adjusts assertTwinParity.
type twinParityOption func(*twinParityConfig)

// twinIgnoreProblemDetail compares plan reasons only up to the first ": ", for steps over
// deliberately corrupt files whose decode/parse error text is format-specific.
func twinIgnoreProblemDetail() twinParityOption {
	return func(c *twinParityConfig) { c.ignoreProblemDetail = true }
}

// assertTwinParity fails with a per-field diff when the observations differ under the parity
// rule. name labels the step in failure messages.
func assertTwinParity(t *testing.T, name string, test, markdown twinObservation, opts ...twinParityOption) {
	t.Helper()
	var cfg twinParityConfig
	for _, o := range opts {
		o(&cfg)
	}
	keys := twinAgentKeys(test, markdown)
	fields := []struct {
		field      string
		test, md   any
		flowString bool
	}{
		{"plan items", test.Items, markdown.Items, false},
		{"executed actions", test.Actions, markdown.Actions, false},
		{"decoded agents", test.Agents, markdown.Agents, false},
		{"decoded counterpart agents", test.Counterparts, markdown.Counterparts, false},
		{"manifest records", test.Manifest, markdown.Manifest, false},
		{"owned key differences", test.OwnedKeyDifferences, markdown.OwnedKeyDifferences, false},
		{"flow result", twinFlowView(test), twinFlowView(markdown), true},
	}
	for _, f := range fields {
		a := twinCanonicalJSON(t, f.test, cfg, twinNormaliser(test, keys, f.flowString))
		b := twinCanonicalJSON(t, f.md, cfg, twinNormaliser(markdown, keys, f.flowString))
		if a != b {
			t.Errorf("%s: %s differ between the formats\n--- test format ---\n%s\n--- markdown ---\n%s", name, f.field, a, b)
		}
	}
}

// twinAgentKeys is the union of the decoded agent keys of both observations.
func twinAgentKeys(a, b twinObservation) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string]twinAgent{a.Agents, b.Agents} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	for _, k := range owAllKeys {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// twinFlowView is the comparable form of a flow result: the report plus the promoted file text.
func twinFlowView(o twinObservation) any {
	if o.Flow == nil {
		return nil
	}
	return map[string]any{
		"Transform":    o.Flow.Transform,
		"Promote":      o.Flow.Promote,
		"PromotedFile": string(o.Flow.PromotedFile),
	}
}

// twinNormaliser returns the string replacement applied to every string of a flow result: the
// workspace root becomes <ws> and "<agent key><twin ext>" becomes "<agent key><ext>". Other
// fields are compared as they are.
func twinNormaliser(o twinObservation, keys []string, flow bool) func(string) string {
	if !flow {
		return func(s string) string { return s }
	}
	return func(s string) string {
		for _, form := range []string{o.workspace, filepath.ToSlash(o.workspace)} {
			if form != "" {
				s = strings.ReplaceAll(s, form, "<ws>")
			}
		}
		for _, k := range keys {
			s = strings.ReplaceAll(s, k+o.ext, k+twinExtPlaceholder)
		}
		return s
	}
}

// twinCanonicalJSON renders v as indented JSON after normalisation.
func twinCanonicalJSON(t *testing.T, v any, cfg twinParityConfig, norm func(string) string) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal observation field: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal observation field: %v", err)
	}
	out, err := json.MarshalIndent(twinWalk(tree, "", cfg, norm), "", "  ")
	if err != nil {
		t.Fatalf("marshal normalised field: %v", err)
	}
	return string(out)
}

// twinWalk normalises a JSON tree: empty arrays and objects become null, strings go through
// norm, and (with ignoreProblemDetail) reason strings are cut at their first ": ".
func twinWalk(v any, key string, cfg twinParityConfig, norm func(string) string) any {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return nil
		}
		out := map[string]any{}
		for k, e := range x {
			out[k] = twinWalk(e, k, cfg, norm)
		}
		return out
	case []any:
		if len(x) == 0 {
			return nil
		}
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = twinWalk(e, key, cfg, norm)
		}
		return out
	case string:
		s := norm(x)
		lower := strings.ToLower(key)
		if cfg.ignoreProblemDetail && (lower == "reason" || lower == "reasons") {
			if at := strings.Index(s, ": "); at >= 0 {
				s = s[:at]
			}
		}
		return s
	}
	return v
}
