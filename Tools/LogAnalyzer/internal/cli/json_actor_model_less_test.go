package cli_test

// Per-actor model evidence in the report JSON, driven end to end through the
// CLI on the tool-local fixture testdata/modelkeys.
//
// Rules under test:
//   - Every actor object (orchestrator, agents, and the same actors inside the
//     unattributable bucket) carries "model_keys" and "requests_without_model".
//   - requests_without_model counts the actor's token-bearing requests whose
//     model is empty; entries without a token_usage key are not requests.
//   - model_keys holds the distinct (provider, model) pairs of token-bearing
//     requests with a model, sorted by model then provider; an empty provider
//     stays "" and an actor with no such request gets [].
//   - model_providers is no longer emitted.
//   - A report built from an exported digest agrees with the report from logs.
//   - `total --format json` carries neither new key.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/cli"
)

const (
	modelKeysFixtureLogs = "../../testdata/modelkeys/runs"
	modelKeysFixtureRun  = "20261030T090000Z-c001"
)

// modelLessCount decodes requests_without_model of an actor object.
func modelLessCount(t *testing.T, actor map[string]any) int {
	t.Helper()
	v, ok := actor["requests_without_model"].(float64)
	if !ok {
		t.Fatalf("actor has no numeric requests_without_model: %v", actor)
	}
	return int(v)
}

func fixtureRun(t *testing.T) map[string]any {
	t.Helper()
	run := e2eRunsOf(e2eReport(t, modelKeysFixtureLogs))[modelKeysFixtureRun]
	if run == nil {
		t.Fatalf("fixture run %s missing from the report", modelKeysFixtureRun)
	}
	return run
}

func fixtureBucket(t *testing.T) map[string]any {
	t.Helper()
	b, ok := e2eReport(t, modelKeysFixtureLogs)["unattributable"].(map[string]any)
	if !ok {
		t.Fatal("fixture should report an unattributable bucket")
	}
	return b
}

func TestReportActors_Orchestrator_CountsModelLessTokenBearingRequests(t *testing.T) {
	orch := objectAt(t, fixtureRun(t), "orchestrator")

	if got := modelLessCount(t, orch); got != 2 {
		t.Errorf("orchestrator requests_without_model = %d, want 2 (the entry without tokens is not a request)", got)
	}
	if got, want := pairsOf(t, orch), []string{"openai|gpt-5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("orchestrator model_keys = %q, want %q", got, want)
	}
}

func TestReportActors_Agent_KeysSortedByModelThenProviderAndEmptyProviderKept(t *testing.T) {
	agent := agentByLabel(t, fixtureRun(t), "worker#1")

	got := pairsOf(t, agent)

	want := []string{"|bare", "p1|shared", "p2|shared", "anthropic|zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("worker#1 model_keys = %q, want %q", got, want)
	}
	if n := modelLessCount(t, agent); n != 1 {
		t.Errorf("worker#1 requests_without_model = %d, want 1 (model-less entry without tokens is not counted)", n)
	}
	if models := stringList(t, agent["models"]); !reflect.DeepEqual(models, []string{"bare", "shared", "zeta"}) {
		t.Errorf("worker#1 models = %q, want [bare shared zeta]", models)
	}
}

func TestReportActors_AgentWithOnlyModelLessRequests_HasEmptyKeysAndCount(t *testing.T) {
	agent := agentByLabel(t, fixtureRun(t), "idle#1")

	items, ok := agent["model_keys"].([]any)
	if !ok || len(items) != 0 {
		t.Errorf("idle#1 model_keys = %v, want an empty array", agent["model_keys"])
	}
	if n := modelLessCount(t, agent); n != 3 {
		t.Errorf("idle#1 requests_without_model = %d, want 3", n)
	}
}

func TestReportActors_Unattributable_ActorsCarryBothKeys(t *testing.T) {
	bucket := fixtureBucket(t)

	orch := objectAt(t, bucket, "orchestrator")
	stray := agentByLabel(t, bucket, "stray#1")

	if got, want := pairsOf(t, orch), []string{"anthropic|claude-opus-4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("unattributable orchestrator model_keys = %q, want %q", got, want)
	}
	if n := modelLessCount(t, orch); n != 0 {
		t.Errorf("unattributable orchestrator requests_without_model = %d, want 0", n)
	}
	if items, ok := stray["model_keys"].([]any); !ok || len(items) != 0 {
		t.Errorf("stray#1 model_keys = %v, want an empty array", stray["model_keys"])
	}
	if n := modelLessCount(t, stray); n != 2 {
		t.Errorf("stray#1 requests_without_model = %d, want 2", n)
	}
}

func TestReportActors_NoActorEmitsModelProviders(t *testing.T) {
	run := fixtureRun(t)
	actors := []map[string]any{objectAt(t, run, "orchestrator")}
	for _, a := range vwList(t, run, "agents") {
		actors = append(actors, vwObj(t, a))
	}

	for _, actor := range actors {
		if _, present := actor["model_providers"]; present {
			t.Errorf("actor %v still emits model_providers", actor["actor"])
		}
	}
}

func TestReportActors_ReportFromDigest_AgreesOnModelEvidence(t *testing.T) {
	digestPath := e2eExport(t, modelKeysFixtureLogs, "")
	fromLogs := fixtureRun(t)
	fromDigest := e2eRunsOf(e2eReport(t, digestPath))[modelKeysFixtureRun]
	if fromDigest == nil {
		t.Fatalf("digest report is missing run %s", modelKeysFixtureRun)
	}

	for _, label := range []string{"worker#1", "idle#1"} {
		want, got := agentByLabel(t, fromLogs, label), agentByLabel(t, fromDigest, label)
		if a, b := modelLessCount(t, got), modelLessCount(t, want); a != b {
			t.Errorf("%s requests_without_model from digest = %d, from logs = %d", label, a, b)
		}
		if a, b := pairsOf(t, got), pairsOf(t, want); !reflect.DeepEqual(a, b) {
			t.Errorf("%s model_keys from digest = %q, from logs = %q", label, a, b)
		}
	}
	wantOrch, gotOrch := objectAt(t, fromLogs, "orchestrator"), objectAt(t, fromDigest, "orchestrator")
	if a, b := modelLessCount(t, gotOrch), modelLessCount(t, wantOrch); a != b || b != 2 {
		t.Errorf("orchestrator requests_without_model from digest = %d, from logs = %d, want both 2", a, b)
	}
}

func TestTotalJSON_OnModelLessFixture_CarriesNeitherNewKey(t *testing.T) {
	code, stdout, stderr := e2eRun(t, "total", "--format", "json", "--run", modelKeysFixtureRun, "--path", modelKeysFixtureLogs)
	if code != cli.ExitSuccess {
		t.Fatalf("total exited %d: %s", code, stderr)
	}

	got := decodeObject(t, []byte(stdout))

	for _, key := range []string{"complete", "currency", "money", "provisional", "run_id", "schema_version", "tokens", "unpriced_models"} {
		if _, present := got[key]; !present {
			t.Errorf("total lost its %q key: %v", key, sortedKeys(got))
		}
	}
	if strings.Contains(stdout, "model_keys") || strings.Contains(stdout, "requests_without_model") {
		t.Errorf("total output must not mention the per-actor model fields: %s", stdout)
	}
}
