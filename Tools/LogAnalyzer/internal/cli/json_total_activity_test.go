package cli_test

// Guard for the Go-side-only run model: RunTotal.Activity is never encoded by
// EncodeRunTotal, so the total output is byte-identical whether or not the run
// model is populated.

import (
	"bytes"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func TestEncodeRunTotal_PopulatedActivity_DoesNotChangeOutput(t *testing.T) {
	base := app.RunTotal{
		RunID:    "20260101T000000Z-abcd",
		Tokens:   domain.TokenUsage{Input: domain.Tokens(1000)},
		Complete: true,
	}
	withActivity := base
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	withActivity.Activity = domain.RunActivity{
		OrchestratorToolCalls: []domain.ToolCall{{CallID: "o1", ToolName: "Read", StartedAt: at, EndedAt: at.Add(time.Second)}},
		Compactions:           []domain.Compaction{{Trigger: "auto", At: at}},
		Invocations:           []domain.Invocation{{StatusCode: "SUCCESS", Model: "model-a"}},
	}

	var plain, populated bytes.Buffer
	if err := cli.EncodeRunTotal(&plain, base); err != nil {
		t.Fatalf("EncodeRunTotal without activity: %v", err)
	}
	if err := cli.EncodeRunTotal(&populated, withActivity); err != nil {
		t.Fatalf("EncodeRunTotal with activity: %v", err)
	}

	if !bytes.Equal(plain.Bytes(), populated.Bytes()) {
		t.Errorf("total output changed when Activity was populated:\n with %s\n without %s", populated.String(), plain.String())
	}
}
