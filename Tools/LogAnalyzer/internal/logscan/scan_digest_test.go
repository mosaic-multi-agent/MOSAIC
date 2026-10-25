package logscan_test

// Tests that digest files never change how the scanner classifies or
// enumerates a path: the scanner stays logs-only, which keeps the total flow
// unchanged. Digest files and other JSON files are plain files to the scanner.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logscan"
)

func writePlainFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
		t.Fatalf("writePlainFile: %v", err)
	}
	return p
}

func TestClassify_DigestFilePath_StaysUnusable(t *testing.T) {
	p := writePlainFile(t, t.TempDir(), "run.digest.json")

	src := logscan.NewScanner().Classify(p)

	if src.Kind != domain.SourceUnusable || src.Reason == "" {
		t.Errorf("Classify(digest file) = %+v, want SourceUnusable with a reason", src)
	}
}

func TestClassify_DirectoryWithOnlyDigestAndJSONFiles_StaysUnusable(t *testing.T) {
	dir := t.TempDir()
	writePlainFile(t, dir, "a.digest.json")
	writePlainFile(t, dir, "settings.json")

	src := logscan.NewScanner().Classify(dir)

	if src.Kind != domain.SourceUnusable {
		t.Errorf("Classify(digest-only dir) kind = %v, want SourceUnusable", src.Kind)
	}
}

func TestClassifyAndEnumerate_LogsRootHoldingDigestFiles_IgnoresThemSilently(t *testing.T) {
	_, root := makeLogsRoot(t)
	makeRunFolder(t, root, validRunID1)
	writePlainFile(t, root, "a.digest.json")
	writePlainFile(t, root, "settings.json")
	sc := logscan.NewScanner()

	src := sc.Classify(root)
	inv, findings := sc.Enumerate(src)

	if src.Kind != domain.SourceLogsRoot {
		t.Errorf("Classify kind = %v, want SourceLogsRoot", src.Kind)
	}
	if len(inv.Runs) != 1 || len(findings) != 0 {
		t.Errorf("Enumerate = %d runs, findings %+v; want 1 run and no findings", len(inv.Runs), findings)
	}
}

func TestClassifyAndEnumerate_SingleRunFolderHoldingDigestFile_IgnoresIt(t *testing.T) {
	_, root := makeLogsRoot(t)
	runDir := makeRunFolder(t, root, validRunID1)
	makeOrchestratorFile(t, runDir)
	writePlainFile(t, runDir, "x.digest.json")
	sc := logscan.NewScanner()

	src := sc.Classify(runDir)
	_, findings := sc.Enumerate(src)

	if src.Kind != domain.SourceSingleRun || len(findings) != 0 {
		t.Errorf("Classify kind = %v, findings = %+v; want SourceSingleRun and none", src.Kind, findings)
	}
}

func TestEnumerate_DigestSourceKinds_YieldEmptyInventoryWithoutFindings(t *testing.T) {
	dir := t.TempDir()
	writePlainFile(t, dir, "a.digest.json")
	for _, kind := range []domain.SourceKind{domain.SourceDigestFile, domain.SourceDigestDir} {
		src := domain.Source{Kind: kind, Path: dir}

		inv, findings := logscan.NewScanner().Enumerate(src)

		if !inv.IsEmpty() || inv.Source != src || len(findings) != 0 {
			t.Errorf("Enumerate(kind %v) = (%+v, %+v), want an empty inventory carrying the source and no findings", kind, inv, findings)
		}
	}
}
