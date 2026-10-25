package digest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/domain"
)

func TestWrite_NewPathIsWrittenAndReadsBack(t *testing.T) {
	path := digestPath(t)
	d := fullDigest()

	err := digest.NewStore().Write(context.Background(), path, d, false)

	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, findings := readDigest(t, path)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireSame(t, "aggregate", d.Aggregate, got.Aggregate)
}

func TestWrite_CreatesMissingParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "out.digest.json")

	err := digest.NewStore().Write(context.Background(), path, fullDigest(), false)

	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("file missing: %v", statErr)
	}
}

func TestWrite_ExistingPathIsRefusedAndLeftUntouched(t *testing.T) {
	path := digestPath(t)
	original := []byte("precious existing content")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	err := digest.NewStore().Write(context.Background(), path, fullDigest(), false)

	if !errors.Is(err, domain.ErrDigestExists) {
		t.Fatalf("err = %v, want ErrDigestExists", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || string(after) != string(original) {
		t.Errorf("existing file changed: %q (%v)", after, readErr)
	}
	assertOnlyFile(t, filepath.Dir(path), filepath.Base(path))
}

func TestWrite_ExistingPathIsReplacedWhenOverwriteRequested(t *testing.T) {
	path := digestPath(t)
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := fullDigest()

	err := digest.NewStore().Write(context.Background(), path, d, true)

	if err != nil {
		t.Fatalf("Write with overwrite: %v", err)
	}
	got, findings := readDigest(t, path)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
	requireSame(t, "aggregate", d.Aggregate, got.Aggregate)
	assertOnlyFile(t, filepath.Dir(path), filepath.Base(path))
}

func TestWrite_SuccessLeavesNoTemporaryFiles(t *testing.T) {
	path := digestPath(t)

	if err := digest.NewStore().Write(context.Background(), path, fullDigest(), false); err != nil {
		t.Fatalf("Write: %v", err)
	}

	assertOnlyFile(t, filepath.Dir(path), filepath.Base(path))
}

func TestWrite_HeaderCarriesVersionToolVersionAndWrittenAtAsGiven(t *testing.T) {
	d := domain.Digest{
		Header:    domain.DigestHeader{ToolVersion: "3.2.1", WrittenAt: at(1234)},
		Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{sparseRun()}},
	}
	path := writeDigest(t, d)

	doc := readJSONObject(t, path)

	if doc["format"] != "mosaic-run-digest" {
		t.Errorf("format = %v", doc["format"])
	}
	if doc["digest_version"] != digest.Version || digest.Version != "1.0.0" {
		t.Errorf("digest_version = %v, package Version = %q", doc["digest_version"], digest.Version)
	}
	if doc["tool_version"] != "3.2.1" {
		t.Errorf("tool_version = %v", doc["tool_version"])
	}
	text, _ := doc["written_at"].(string)
	if parsed, err := time.Parse(time.RFC3339Nano, text); err != nil || !parsed.Equal(at(1234)) {
		t.Errorf("written_at = %v (parse error %v), want the instant %v", doc["written_at"], err, at(1234))
	}
}

func TestWrite_AbsentHeaderValuesAreOmitted(t *testing.T) {
	path := writeDigest(t, domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{sparseRun()}}})

	doc := readJSONObject(t, path)

	if _, has := doc["tool_version"]; has {
		t.Error("empty tool_version was written")
	}
	if _, has := doc["written_at"]; has {
		t.Error("zero written_at was written")
	}
	if doc["digest_version"] == nil || doc["format"] == nil {
		t.Errorf("required header keys missing: %v", doc)
	}
}

func TestWrite_FailureReturnsErrorAndLeavesNoStrayFiles(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("a regular file"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "sub", "out.digest.json")
	// Control: the same digest writes fine elsewhere, so the failure below is
	// caused by the blocked path and not by a store that cannot write at all.
	_ = writeDigest(t, fullDigest())

	err := digest.NewStore().Write(context.Background(), path, fullDigest(), true)

	if err == nil {
		t.Fatal("Write under a regular file returned no error")
	}
	if errors.Is(err, domain.ErrDigestExists) {
		t.Errorf("a creation failure was reported as already exists: %v", err)
	}
	assertOnlyFile(t, dir, "blocker")
	if content, readErr := os.ReadFile(blocker); readErr != nil || string(content) != "a regular file" {
		t.Errorf("blocker changed: %q (%v)", content, readErr)
	}
}

func TestWrite_AlwaysWritesProvisionalEndEventsAndPairing(t *testing.T) {
	run := domain.RunAggregate{
		Run:         domain.NamedRun(runA),
		Provisional: false,
		Activity: domain.RunActivity{
			Invocations: []domain.Invocation{{
				Actor:     domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "a#1", Type: "a"},
				State:     domain.InvocationOpen,
				EndEvents: 0,
			}},
			OrchestratorToolCalls: []domain.ToolCall{{CallID: "c1", Pairing: domain.ToolCallStartOnly}},
		},
	}
	path := writeDigest(t, domain.Digest{Aggregate: domain.Aggregate{Runs: []domain.RunAggregate{run}}})

	doc := readJSONObject(t, path)

	runs, _ := doc["runs"].([]any)
	requireAtLeast(t, "runs", len(runs), 1)
	obj, _ := runs[0].(map[string]any)
	if v, ok := obj["provisional"]; !ok || v != false {
		t.Errorf("provisional must be written as false, got %v present=%v", v, ok)
	}
	invs, _ := obj["invocations"].([]any)
	requireAtLeast(t, "invocations", len(invs), 1)
	if v, ok := invs[0].(map[string]any)["end_events"]; !ok || v != float64(0) {
		t.Errorf("end_events must be written as 0, got %v present=%v", v, ok)
	}
	orch, _ := obj["orchestrator"].(map[string]any)
	calls, _ := orch["tool_calls"].([]any)
	requireAtLeast(t, "orchestrator tool calls", len(calls), 1)
	if v, ok := calls[0].(map[string]any)["pairing"]; !ok || v != "start_only" {
		t.Errorf("pairing must be written, got %v present=%v", v, ok)
	}
}

func TestDiscover_FileDirectoryAndOtherPaths(t *testing.T) {
	dir := t.TempDir()
	touch := func(rel string) string {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	b, a, upper := touch("b.digest.json"), touch("a.digest.json"), touch("z.DIGEST.JSON")
	touch("other.json")
	touch(".hidden.digest.json")
	touch("nested/deep.digest.json")
	if err := os.MkdirAll(filepath.Join(dir, "dir.digest.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := digest.NewStore()

	if got := store.Discover(dir); !reflect.DeepEqual(got, []string{a, b, upper}) {
		t.Errorf("directory discover = %v, want [%s %s %s]", got, a, b, upper)
	}
	if got := store.Discover(a); !reflect.DeepEqual(got, []string{a}) {
		t.Errorf("file discover = %v", got)
	}
	if got := store.Discover(filepath.Join(dir, "other.json")); len(got) != 0 {
		t.Errorf("non-digest file discovered: %v", got)
	}
	if got := store.Discover(filepath.Join(dir, "absent.digest.json")); len(got) != 0 {
		t.Errorf("absent path discovered: %v", got)
	}
}

// assertOnlyFile fails when dir holds anything besides the named file.
func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only %s", names, name)
	}
}
