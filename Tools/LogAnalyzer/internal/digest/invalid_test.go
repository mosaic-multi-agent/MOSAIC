package digest_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
)

// withRun returns a valid doc whose single run is run.
func withRun(run map[string]any) map[string]any {
	doc := validDoc("1.0.0")
	doc["runs"] = []any{run}
	return doc
}

func invalidDocs() map[string]any {
	badFormat := validDoc("1.0.0")
	badFormat["format"] = "something-else"
	noFormat := validDoc("1.0.0")
	delete(noFormat, "format")
	noVersion := validDoc("1.0.0")
	delete(noVersion, "digest_version")
	runsWrongType := validDoc("1.0.0")
	runsWrongType["runs"] = "x"
	tokenWrongType := withRun(map[string]any{"run_id": runA, "orchestrator": map[string]any{
		"usage": []any{map[string]any{"tokens": map[string]any{"input_tokens": "many"}}}}})
	return map[string]any{
		"json array":               []any{1, 2},
		"json string":              "hello",
		"json null":                nil,
		"unrelated object":         map[string]any{"name": "x", "runs": []any{}},
		"format different":         badFormat,
		"format missing":           noFormat,
		"version missing":          noVersion,
		"version two parts":        validDoc("1.0"),
		"version prefixed":         validDoc("v1.0.0"),
		"version non numeric":      validDoc("1.x.0"),
		"runs wrong type":          runsWrongType,
		"token wrong type":         tokenWrongType,
		"run without run_id":       withRun(map[string]any{"provisional": true}),
		"run with empty run_id":    withRun(map[string]any{"run_id": ""}),
		"unknown invocation state": withRun(map[string]any{"run_id": runA, "invocations": []any{map[string]any{"instance_id": "a#1", "state": "weird"}}}),
		"unknown pairing": withRun(map[string]any{"run_id": runA, "orchestrator": map[string]any{
			"tool_calls": []any{map[string]any{"call_id": "c", "pairing": "sideways"}}}}),
		"unknown actor kind": withRun(map[string]any{"run_id": runA, "turns": []any{map[string]any{"actor": map[string]any{"kind": "robot"}}}}),
		"unknown severity": func() map[string]any {
			d := validDoc("1.0.0")
			d["findings"] = []any{map[string]any{"kind": "k", "severity": "fatal", "scope": "general"}}
			return d
		}(),
		"unknown scope": func() map[string]any {
			d := validDoc("1.0.0")
			d["findings"] = []any{map[string]any{"kind": "k", "severity": "info", "scope": "galaxy"}}
			return d
		}(),
	}
}

func TestRead_InvalidDigestYieldsOneFindingAndNoData(t *testing.T) {
	for name, doc := range invalidDocs() {
		t.Run(name, func(t *testing.T) {
			path := writeRawDoc(t, doc)

			got, findings := readDigest(t, path)

			assertOneInvalidFinding(t, path, got, findings)
		})
	}
}

func TestRead_NonJSONContentYieldsInvalidFindingWithoutEchoingContent(t *testing.T) {
	const marker = "CONTENT-MARKER-5521"
	cases := map[string][]byte{
		"plain text":  []byte("this is not json " + marker),
		"empty file":  {},
		"truncated":   []byte(`{"format":"mosaic-run-digest","digest_version":"1.0.0","runs":[{"run_id":"` + marker),
		"binary junk": {0x00, 0xff, 0xfe, 0x01},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeRawBytes(t, raw)

			got, findings := readDigest(t, path)

			assertOneInvalidFinding(t, path, got, findings)
			if strings.Contains(findings[0].Detail, marker) {
				t.Errorf("finding detail echoes file content: %q", findings[0].Detail)
			}
		})
	}
}

func TestRead_MissingFileYieldsInvalidFindingNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.digest.json")

	got, findings := readDigest(t, path)

	assertOneInvalidFinding(t, path, got, findings)
}

func TestRead_DuplicateRunIdInOneFileKeepsEarlierAndReportsLater(t *testing.T) {
	doc := validDoc("1.0.0")
	doc["runs"] = []any{
		map[string]any{"run_id": runA, "outcome": "first"},
		map[string]any{"run_id": runB},
		map[string]any{"run_id": runA, "outcome": "second"},
	}
	path := writeRawDoc(t, doc)

	got, findings := readDigest(t, path)

	if len(got.Aggregate.Runs) != 2 || got.Aggregate.Runs[0].Facts.Outcome != "first" {
		t.Fatalf("runs = %+v, want the earlier entry kept", got.Aggregate.Runs)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one", findings)
	}
	f := findings[0]
	if f.Kind != domain.FindingDuplicateRun || f.Severity != domain.SeverityInfo || f.Path != path ||
		f.Run != domain.NamedRun(runA) {
		t.Errorf("finding = %+v", f)
	}
}

func TestRead_CancelledContextReturnsError(t *testing.T) {
	path := writeDigest(t, fullDigest())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := digest.NewStore().Read(ctx, path)

	if err == nil {
		t.Fatal("Read with a cancelled context returned no error")
	}
}

func assertOneInvalidFinding(t *testing.T, path string, got domain.Digest, findings []domain.Finding) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", findings)
	}
	f := findings[0]
	if f.Kind != domain.FindingInvalidDigest || f.Severity != domain.SeverityWarning || f.Path != path {
		t.Errorf("finding = %+v", f)
	}
	if !got.Aggregate.IsEmpty() || len(got.Findings) != 0 {
		t.Errorf("an invalid file contributed data: %+v", got)
	}
}
