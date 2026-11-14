package app_test

// experimental_json_test.go pins the JSON contract of the operation results: the encoded result
// of an experimental run carries the experimental harnesses, the encoded result of a run with no
// experimental harness has exactly the keys it had before (no new key, also not inside the
// harness reference), and the read-only render result never carries the field.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// Key names of the experimental field in the documents of each result type.
const (
	summaryExperimentalKey = "ExperimentalHarnesses"
	resultExperimentalKey  = "experimentalHarnesses"
)

// jsonObject encodes v as JSON and decodes it into a generic object.
func jsonObject(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode result: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return m
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// without returns the keys of m except the named one.
func without(m map[string]any, key string) []string {
	var out []string
	for _, k := range sortedKeys(m) {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

// assertExperimentalArray checks m[key] is an array of one object holding the harness id,
// display name and reason of ref under the keys harnessId, displayName and reason.
func assertExperimentalArray(t *testing.T, m map[string]any, key string, ref domain.HarnessRef) {
	t.Helper()
	arr, ok := m[key].([]any)
	if !ok || len(arr) != 1 {
		t.Fatalf("%s = %#v, want an array of one entry", key, m[key])
	}
	entry, _ := arr[0].(map[string]any)
	want := map[string]any{"harnessId": ref.ID, "displayName": ref.DisplayName, "reason": ref.ExperimentalReason}
	if !reflect.DeepEqual(entry, want) {
		t.Errorf("%s[0] = %#v, want %#v", key, entry, want)
	}
}

// summaryBaselineKeys are the top-level keys of a RunSummary document before the experimental
// field existed.
var summaryBaselineKeys = []string{
	"Actions", "ConflictReport", "DeploymentRoot", "External", "Fallback", "Harness", "LogDegraded",
	"Logs", "Mode", "Outcome", "OwnedKeyDifferences", "Todos", "TodoFilePath", "WorkspacePath",
}

func TestRunSummaryJSON_PlainRun_KeepsEveryKeyAndAddsNone(t *testing.T) {
	// Arrange
	mod := newTestExperimentalModule(t, expPlainID, "")
	svc := newTestExperimentalService(t, []domain.HarnessModule{mod})

	// Act
	sum, err := opFlows[0].run(svc, expPlainID, t.TempDir(), opFlags{autoConfirm: true})
	doc := jsonObject(t, sum)

	// Assert
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	want := append([]string(nil), summaryBaselineKeys...)
	sort.Strings(want)
	if got := sortedKeys(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("document keys = %v, want exactly %v", got, want)
	}
}

func TestRunSummaryJSON_PlainRun_HarnessReferenceKeepsItsKeys(t *testing.T) {
	// Arrange
	plain := newTestExperimentalModule(t, expPlainID, "")
	exp := newTestExperimentalModule(t, expHarnessID, expReason)

	// Act
	var harnessKeys [2][]string
	for i, mod := range []domain.HarnessModule{plain, exp} {
		svc := newTestExperimentalService(t, []domain.HarnessModule{mod})
		sum, err := opFlows[0].run(svc, mod.Ref().ID, t.TempDir(), opFlags{autoConfirm: true})
		if err != nil {
			t.Fatalf("deploy: %v", err)
		}
		harnessKeys[i] = sortedKeys(jsonObject(t, sum)["Harness"].(map[string]any))
	}

	// Assert
	want := []string{"DisplayName", "ExecutablePath", "ID", "RequiresOptIn", "SourcePath", "Tier",
		"UnusableReason", "Usable"}
	if !reflect.DeepEqual(harnessKeys[0], want) {
		t.Errorf("plain Harness keys = %v, want %v", harnessKeys[0], want)
	}
	if !reflect.DeepEqual(harnessKeys[1], want) {
		t.Errorf("experimental Harness keys = %v, want %v (the declaration must not leak into the reference)",
			harnessKeys[1], want)
	}
}

func TestRunSummaryJSON_ExperimentalRun_AddsOnlyTheExperimentalKeyInEveryFlow(t *testing.T) {
	for _, flow := range opFlows {
		t.Run(flow.name, func(t *testing.T) {
			// Arrange
			mod := newTestExperimentalModule(t, expHarnessID, expReason)
			svc := newTestExperimentalService(t, []domain.HarnessModule{mod})

			// Act
			sum, err := runOpFlow(t, flow, svc, expHarnessID, opFlags{autoConfirm: true})
			doc := jsonObject(t, sum)

			// Assert
			if err != nil {
				t.Fatalf("%s: %v", flow.name, err)
			}
			assertExperimentalArray(t, doc, summaryExperimentalKey, mod.Ref())
			want := append([]string(nil), summaryBaselineKeys...)
			sort.Strings(want)
			if got := without(doc, summaryExperimentalKey); !reflect.DeepEqual(got, want) {
				t.Errorf("other keys = %v, want the unchanged %v", got, want)
			}
		})
	}
}

func TestPromoteResultJSON_ExperimentalHarness_AddsOnlyTheExperimentalKey(t *testing.T) {
	// Arrange
	plain := jsonObject(t, promoteResultFor(t, "", false))
	exp := promoteResultFor(t, expReason, false)

	// Act
	doc := jsonObject(t, exp)

	// Assert
	assertExperimentalArray(t, doc, resultExperimentalKey, domain.HarnessRef{
		ID: expHarnessID, DisplayName: "Display " + expHarnessID, ExperimentalReason: expReason})
	if got, want := without(doc, resultExperimentalKey), sortedKeys(plain); !reflect.DeepEqual(got, want) {
		t.Errorf("other keys = %v, want the unchanged %v", got, want)
	}
}

func TestPromoteResultJSON_PlainHarness_HasNoExperimentalKey(t *testing.T) {
	// Act
	doc := jsonObject(t, promoteResultFor(t, "", false))

	// Assert
	if _, present := doc[resultExperimentalKey]; present {
		t.Errorf("document of a non-experimental run has key %q: %v", resultExperimentalKey, sortedKeys(doc))
	}
}

func TestTransformResultJSON_ExperimentalSourceOnly_AddsOnlyTheExperimentalKey(t *testing.T) {
	// Arrange
	srcExp := newTestExperimentalModule(t, expHarnessID, expReason)
	tgtPlain := newTestExperimentalModuleOn(t, hClaude, expPlainID, "")
	plainSrc := newTestExperimentalModule(t, expHarnessID, "")
	plainTgt := newTestExperimentalModuleOn(t, hClaude, expPlainID, "")

	// Act
	doc := jsonObject(t, transformResult(t, srcExp, tgtPlain, false))
	plain := jsonObject(t, transformResult(t, plainSrc, plainTgt, false))

	// Assert
	assertExperimentalArray(t, doc, resultExperimentalKey, srcExp.Ref())
	if got, want := without(doc, resultExperimentalKey), sortedKeys(plain); !reflect.DeepEqual(got, want) {
		t.Errorf("other keys = %v, want the unchanged %v", got, want)
	}
}

func TestTransformResultJSON_PlainHarnesses_HasNoExperimentalKey(t *testing.T) {
	// Arrange
	src := newTestExperimentalModule(t, expPlainID, "")
	tgt := newTestExperimentalModuleOn(t, hClaude, "other-plain", "")

	// Act
	doc := jsonObject(t, transformResult(t, src, tgt, false))

	// Assert
	if _, present := doc[resultExperimentalKey]; present {
		t.Errorf("document of a non-experimental run has key %q: %v", resultExperimentalKey, sortedKeys(doc))
	}
}

// renderDocumentKeys renders a generic agent for the render test harness, declared experimental
// with the given reason, and returns the keys of the encoded result.
func renderDocumentKeys(t *testing.T, reason string) []string {
	t.Helper()
	deps := newRenderDeps(t)
	mod := newRenderTargetModule()
	mod.ref.ExperimentalReason = reason
	deps.Registry = &stubRegistry{list: []domain.HarnessRef{mod.ref},
		modules: map[string]domain.HarnessModule{mod.ref.ID: mod}}
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	res, err := app.New(deps).RenderAgent(context.Background(),
		newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md")))
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	return sortedKeys(jsonObject(t, res))
}

// Tools/AgentTest consumes `render --output json` as an external process: the document must not
// change for an experimental harness.
func TestRenderResultJSON_ExperimentalHarness_IsUnchanged(t *testing.T) {
	// Act
	plain := renderDocumentKeys(t, "")
	exp := renderDocumentKeys(t, expReason)

	// Assert
	if !reflect.DeepEqual(exp, plain) {
		t.Errorf("render document keys = %v for an experimental harness, want the unchanged %v", exp, plain)
	}
	for _, k := range exp {
		if k == resultExperimentalKey || k == summaryExperimentalKey {
			t.Errorf("render document carries %q", k)
		}
	}
}
