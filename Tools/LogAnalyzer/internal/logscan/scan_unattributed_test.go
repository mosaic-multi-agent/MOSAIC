package logscan_test

// Tests for discovery of the run-level unattributed stream
// (00_unattributed_events.jsonl in a run folder's root).
//
// Rules under test:
//   - The file is discovered next to the orchestrator file, in logs roots and in
//     single-run sources.
//   - A run folder without the file keeps an empty UnattributedFile.
//   - Dot-prefixed entries (such as .otel-ingest/) stay skipped, also when they
//     hold a file of that name.
//   - The unknown-run/ bucket never gets an unattributed file.
//   - The file raises no finding and is not mistaken for an agent folder.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logscan"
)

func makeUnattributedFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, logscan.UnattributedEventFile)
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatalf("makeUnattributedFile: %v", err)
	}
	return path
}

func enumerateOneRun(t *testing.T, root string) (domain.RunEntry, []domain.Finding) {
	t.Helper()
	s := logscan.NewScanner()
	inv, findings := s.Enumerate(s.Classify(root))
	if len(inv.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(inv.Runs))
	}
	return inv.Runs[0], findings
}

func TestEnumerate_UnattributedFile_IsDiscoveredNextToTheOrchestratorFile(t *testing.T) {
	root := t.TempDir()
	runDir := makeRunFolder(t, root, validRunID1)
	orchFile := makeOrchestratorFile(t, runDir)
	unattrFile := makeUnattributedFile(t, runDir)

	run, findings := enumerateOneRun(t, root)

	if run.UnattributedFile != unattrFile {
		t.Errorf("UnattributedFile = %q, want %q", run.UnattributedFile, unattrFile)
	}
	if run.OrchestratorFile != orchFile {
		t.Errorf("OrchestratorFile = %q, want %q (must be unchanged)", run.OrchestratorFile, orchFile)
	}
	if len(run.Agents) != 0 {
		t.Errorf("Agents = %d, want 0: the file is not an agent folder", len(run.Agents))
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestEnumerate_UnattributedFileWithAgentFolders_AgentsAreUnaffected(t *testing.T) {
	root := t.TempDir()
	runDir := makeRunFolder(t, root, validRunID1)
	makeOrchestratorFile(t, runDir)
	unattrFile := makeUnattributedFile(t, runDir)
	agentEvents := makeAgentEventFile(t, makeAgentFolder(t, runDir, "coder#1"))

	run, _ := enumerateOneRun(t, root)

	if run.UnattributedFile != unattrFile {
		t.Errorf("UnattributedFile = %q, want %q", run.UnattributedFile, unattrFile)
	}
	if len(run.Agents) != 1 || run.Agents[0].EventFile != agentEvents {
		t.Errorf("Agents = %+v, want the single coder#1 folder with its event file", run.Agents)
	}
}

func TestEnumerate_UnattributedFileAlone_IsStillDiscovered(t *testing.T) {
	root := t.TempDir()
	runDir := makeRunFolder(t, root, validRunID1)
	unattrFile := makeUnattributedFile(t, runDir)

	run, _ := enumerateOneRun(t, root)

	if run.UnattributedFile != unattrFile {
		t.Errorf("UnattributedFile = %q, want %q", run.UnattributedFile, unattrFile)
	}
	if run.OrchestratorFile != "" {
		t.Errorf("OrchestratorFile = %q, want empty", run.OrchestratorFile)
	}
}

func TestEnumerate_RunFolderWithoutUnattributedFile_HasEmptyUnattributedFile(t *testing.T) {
	root := t.TempDir()
	runDir := makeRunFolder(t, root, validRunID1)
	makeOrchestratorFile(t, runDir)
	makeAgentEventFile(t, makeAgentFolder(t, runDir, "coder#1"))

	run, findings := enumerateOneRun(t, root)

	if run.UnattributedFile != "" {
		t.Errorf("UnattributedFile = %q, want empty when the file is absent", run.UnattributedFile)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestEnumerate_UnattributedFileOnlyInsideOtelIngestFolder_IsIgnored(t *testing.T) {
	root := t.TempDir()
	runDir := makeRunFolder(t, root, validRunID1)
	makeOrchestratorFile(t, runDir)
	ingestDir := filepath.Join(runDir, ".otel-ingest")
	if err := os.MkdirAll(ingestDir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	makeUnattributedFile(t, ingestDir)

	run, findings := enumerateOneRun(t, root)

	if run.UnattributedFile != "" {
		t.Errorf("UnattributedFile = %q, want empty: dot-prefixed entries stay skipped", run.UnattributedFile)
	}
	if len(run.Agents) != 0 {
		t.Errorf("Agents = %d, want 0", len(run.Agents))
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestEnumerate_UnattributedFileInRunRootAndOtelIngestFolder_TakesTheRunRootFile(t *testing.T) {
	root := t.TempDir()
	runDir := makeRunFolder(t, root, validRunID1)
	makeOrchestratorFile(t, runDir)
	ingestDir := filepath.Join(runDir, ".otel-ingest")
	if err := os.MkdirAll(ingestDir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	makeUnattributedFile(t, ingestDir)
	rootFile := makeUnattributedFile(t, runDir)

	run, _ := enumerateOneRun(t, root)

	if run.UnattributedFile != rootFile {
		t.Errorf("UnattributedFile = %q, want the run-root file %q", run.UnattributedFile, rootFile)
	}
}

func TestEnumerate_SingleRunSource_DiscoversTheUnattributedFile(t *testing.T) {
	runDir := makeRunFolder(t, t.TempDir(), validRunID1)
	makeOrchestratorFile(t, runDir)
	unattrFile := makeUnattributedFile(t, runDir)
	s := logscan.NewScanner()
	src := s.Classify(runDir)
	if src.Kind != domain.SourceSingleRun {
		t.Fatalf("source kind = %v, want SourceSingleRun", src.Kind)
	}

	inv, _ := s.Enumerate(src)

	if len(inv.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(inv.Runs))
	}
	if inv.Runs[0].UnattributedFile != unattrFile {
		t.Errorf("UnattributedFile = %q, want %q", inv.Runs[0].UnattributedFile, unattrFile)
	}
}

func TestEnumerate_UnknownRunFolderWithUnattributedFile_IsNotRead(t *testing.T) {
	root := t.TempDir()
	unknownDir := filepath.Join(root, domain.UnknownRunDirName)
	if err := os.MkdirAll(unknownDir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	orchFile := makeOrchestratorFile(t, unknownDir)
	makeUnattributedFile(t, unknownDir)
	s := logscan.NewScanner()

	inv, _ := s.Enumerate(s.Classify(root))

	if inv.Unattributable == nil {
		t.Fatal("Inventory.Unattributable is nil, want the unknown-run bucket")
	}
	if inv.Unattributable.UnattributedFile != "" {
		t.Errorf("Unattributable.UnattributedFile = %q, want empty: the ingest never writes to unknown-run/", inv.Unattributable.UnattributedFile)
	}
	if inv.Unattributable.OrchestratorFile != orchFile {
		t.Errorf("Unattributable.OrchestratorFile = %q, want %q (bucket otherwise unchanged)", inv.Unattributable.OrchestratorFile, orchFile)
	}
}

func TestEnumerate_UnattributedFileInOneRun_DoesNotLeakToSiblingRuns(t *testing.T) {
	root := t.TempDir()
	first := makeRunFolder(t, root, validRunID1)
	makeOrchestratorFile(t, first)
	want := makeUnattributedFile(t, first)
	second := makeRunFolder(t, root, validRunID2)
	makeOrchestratorFile(t, second)
	s := logscan.NewScanner()

	inv, _ := s.Enumerate(s.Classify(root))

	if len(inv.Runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(inv.Runs))
	}
	if inv.Runs[0].UnattributedFile != want || inv.Runs[1].UnattributedFile != "" {
		t.Errorf("unattributed files = %q / %q, want %q / empty", inv.Runs[0].UnattributedFile, inv.Runs[1].UnattributedFile, want)
	}
}
