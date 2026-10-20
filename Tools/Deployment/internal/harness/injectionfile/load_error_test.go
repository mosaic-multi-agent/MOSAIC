package injectionfile_test

// load_error_test.go verifies that LoadDir reports each read or parse failure as a typed
// *LoadError naming the failing file and the failure kind, with the same message text as the
// untyped error it replaces.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/harness/injectionfile"
)

const loadErrValidDoc = "---\nversion: \"1.0.0\"\nharness: claude-code\n---\n\n" +
	"<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"

// loadErrUnparseableDoc opens a managed region and never closes it.
const loadErrUnparseableDoc = "---\nversion: \"1.0.0\"\nharness: claude-code\n---\n\n" +
	"<HarnessConstraints type=\"managed\">\nnever closed\n"

// loadErrDir writes the given file contents into a fresh directory; a nil entry leaves the file out.
func loadErrDir(t *testing.T, shared, orch *string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]*string{
		injectionfile.SharedFileName:       shared,
		injectionfile.OrchestratorFileName: orch,
	} {
		if content == nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(*content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func strPtr(s string) *string { return &s }

func TestLoadDir_FailingFile_ReturnsTypedLoadErrorNamingFileAndKind(t *testing.T) {
	cases := []struct {
		name       string
		shared     *string
		orch       *string
		wantFile   string
		wantKind   injectionfile.LoadErrorKind
		wantNoFile bool
	}{
		{"shared file missing", nil, strPtr(loadErrValidDoc), injectionfile.SharedFileName, injectionfile.LoadErrorRead, true},
		{"orchestrator file missing", strPtr(loadErrValidDoc), nil, injectionfile.OrchestratorFileName, injectionfile.LoadErrorRead, true},
		{"shared file unparseable", strPtr(loadErrUnparseableDoc), strPtr(loadErrValidDoc), injectionfile.SharedFileName, injectionfile.LoadErrorParse, false},
		{"orchestrator file unparseable", strPtr(loadErrValidDoc), strPtr(loadErrUnparseableDoc), injectionfile.OrchestratorFileName, injectionfile.LoadErrorParse, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := loadErrDir(t, tc.shared, tc.orch)

			_, err := injectionfile.LoadDir(dir)

			var le *injectionfile.LoadError
			if !errors.As(err, &le) {
				t.Fatalf("LoadDir error = %v; want an error that is a *injectionfile.LoadError", err)
			}
			if le.File != tc.wantFile {
				t.Errorf("LoadError.File = %q, want %q", le.File, tc.wantFile)
			}
			if le.Kind != tc.wantKind {
				t.Errorf("LoadError.Kind = %v, want %v", le.Kind, tc.wantKind)
			}
			if want := filepath.Join(dir, tc.wantFile); le.Path != want {
				t.Errorf("LoadError.Path = %q, want %q", le.Path, want)
			}
			if got := errors.Is(err, fs.ErrNotExist); got != tc.wantNoFile {
				t.Errorf("errors.Is(err, fs.ErrNotExist) = %v, want %v", got, tc.wantNoFile)
			}
		})
	}
}

func TestLoadDir_FailingFile_MessageKeepsTheReadAndParseText(t *testing.T) {
	missing := loadErrDir(t, nil, strPtr(loadErrValidDoc))
	unparseable := loadErrDir(t, strPtr(loadErrUnparseableDoc), strPtr(loadErrValidDoc))

	_, readErr := injectionfile.LoadDir(missing)
	_, parseErr := injectionfile.LoadDir(unparseable)

	if readErr == nil || !strings.HasPrefix(readErr.Error(), "injectionfile: read "+filepath.Join(missing, injectionfile.SharedFileName)) {
		t.Errorf("read failure text = %v, want the prefix %q", readErr, "injectionfile: read <path>")
	}
	if parseErr == nil || !strings.HasPrefix(parseErr.Error(), "injectionfile: parse "+filepath.Join(unparseable, injectionfile.SharedFileName)) {
		t.Errorf("parse failure text = %v, want the prefix %q", parseErr, "injectionfile: parse <path>")
	}
}
