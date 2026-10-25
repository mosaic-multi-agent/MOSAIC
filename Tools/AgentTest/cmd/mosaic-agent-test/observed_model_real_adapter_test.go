package main

// The real harness adapters from the factory, paired with the real
// observed-model provider (command execution faked), must yield model
// evidence for a matching run. Doubles elsewhere hide an adapter that never
// declares its model form: the provider then answers "unknown form" without
// calling the analyser, and every real run would be excluded.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/claudecode"
	"mosaic-agent-test/internal/harness/opencode"
	"mosaic-agent-test/internal/modelevidence"
)

const realAdapterRunID = "20261024T150000Z-ad01"

func realAdapterReport(provider, model string) string {
	return `{"schema_version":"1","runs":[{"run_id":"` + realAdapterRunID + `",` +
		`"orchestrator":{"actor":"orchestrator","models":["` + model + `"],` +
		`"model_keys":[{"provider":"` + provider + `","model":"` + model + `"}],"requests_without_model":0},` +
		`"agents":[]}],"unattributable":null}`
}

func TestRealAdapters_WithRealProvider_YieldObservedModelForSelectedModel(t *testing.T) {
	tests := []struct {
		name      string
		harnessID string
		report    string
		want      []string
	}{
		{"claude code reports the bare model id", claudecode.HarnessID,
			realAdapterReport("anthropic", "claude-sonnet-5-5"), []string{"claude-sonnet-5-5"}},
		{"opencode reports provider/model", opencode.HarnessID,
			realAdapterReport("anthropic", "claude-opus-5-5"), []string{"anthropic/claude-opus-5-5"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := newHarnessFactory(adapterOptions{}).Bundle(context.Background(), tc.harnessID)
			if err != nil {
				t.Fatalf("Bundle(%q): %v", tc.harnessID, err)
			}
			invoked := false
			provider := modelevidence.New(modelevidence.Options{
				ExecutablePath: "mosaic-log-analyzer",
				Invoke: func(context.Context, string, []string, string) ([]byte, int, error) {
					invoked = true
					return []byte(tc.report), 0, nil
				},
			})
			reporter, ok := bundle.Adapter.(domain.ObservedModelFormReporter)
			if !ok {
				t.Fatalf("adapter %T does not declare an observed-model form", bundle.Adapter)
			}
			if reporter.ObservedModelForm() == "" {
				t.Fatalf("adapter %T declares an empty observed-model form", bundle.Adapter)
			}

			got, err := provider.ObservedSubjectModels(context.Background(), domain.ObservedModelQuery{
				LogRoot: "/logs/" + realAdapterRunID, LogsRoot: "/logs",
				RunID: realAdapterRunID, Form: reporter.ObservedModelForm(),
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !invoked {
				t.Errorf("the analyser was never called; the form was treated as unknown")
			}
			if got.NoEvidence != "" || !reflect.DeepEqual(got.Models, tc.want) {
				t.Errorf("evidence = %+v, want models %v and no missing-evidence reason", got, tc.want)
			}
		})
	}
}
