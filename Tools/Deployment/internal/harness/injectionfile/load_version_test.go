package injectionfile_test

// load_version_test.go verifies that LoadDir reports the frontmatter version of
// HarnessInjections.md as SharedVersion and the version of HarnessInjectionsOrchestrator.md as
// OrchestratorVersion, and that a file that exists without a frontmatter version fails loading
// with a typed error naming the file path.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/harness/injectionfile"
)

const versionedDocTemplate = "---\nversion: \"%s\"\nharness: claude-code\n---\n\n" +
	"<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"

func versionedDoc(version string) string {
	return strings.Replace(versionedDocTemplate, "%s", version, 1)
}

const (
	docWithoutVersionKey = "---\nharness: claude-code\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"
	docWithoutFrontmatter = "<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"
)

// docWithVersionLine builds a document whose frontmatter carries the given raw version line(s).
func docWithVersionLine(line string) string {
	return "---\n" + line + "\nharness: claude-code\n---\n\n" +
		"<HarnessConstraints type=\"managed\">\nbody\n</HarnessConstraints>\n"
}

func TestLoadDir_ReportsBothFrontmatterVersions(t *testing.T) {
	dir := loadErrDir(t, strPtr(versionedDoc("1.4.0")), strPtr(versionedDoc("2.5.1")))

	content, err := injectionfile.LoadDir(dir)

	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if content.SharedVersion != "1.4.0" {
		t.Errorf("SharedVersion = %q, want %q", content.SharedVersion, "1.4.0")
	}
	if content.OrchestratorVersion != "2.5.1" {
		t.Errorf("OrchestratorVersion = %q, want %q", content.OrchestratorVersion, "2.5.1")
	}
}

func TestLoadDir_FileWithoutFrontmatterVersion_FailsNamingTheFile(t *testing.T) {
	cases := []struct {
		name     string
		shared   string
		orch     string
		wantFile string
	}{
		{"shared without version key", docWithoutVersionKey, versionedDoc("1.0.0"), injectionfile.SharedFileName},
		{"shared without frontmatter", docWithoutFrontmatter, versionedDoc("1.0.0"), injectionfile.SharedFileName},
		{"orchestrator without version key", versionedDoc("1.0.0"), docWithoutVersionKey, injectionfile.OrchestratorFileName},
		{"orchestrator without frontmatter", versionedDoc("1.0.0"), docWithoutFrontmatter, injectionfile.OrchestratorFileName},
		{"shared with empty version", docWithVersionLine("version: \"\""), versionedDoc("1.0.0"), injectionfile.SharedFileName},
		{"shared with list version", docWithVersionLine("version: [1, 2]"), versionedDoc("1.0.0"), injectionfile.SharedFileName},
		{"orchestrator with mapping version", versionedDoc("1.0.0"), docWithVersionLine("version:\n  major: 1"), injectionfile.OrchestratorFileName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := loadErrDir(t, strPtr(tc.shared), strPtr(tc.orch))

			_, err := injectionfile.LoadDir(dir)

			var loadErr *injectionfile.LoadError
			if !errors.As(err, &loadErr) {
				t.Fatalf("LoadDir error = %v, want a *injectionfile.LoadError", err)
			}
			if loadErr.Kind != injectionfile.LoadErrorMissingVersion {
				t.Errorf("Kind = %v, want LoadErrorMissingVersion", loadErr.Kind)
			}
			if loadErr.File != tc.wantFile {
				t.Errorf("File = %q, want %q", loadErr.File, tc.wantFile)
			}
			if !strings.Contains(err.Error(), loadErr.Path) || !strings.Contains(loadErr.Path, tc.wantFile) {
				t.Errorf("error %q does not name the file path (Path = %q)", err.Error(), loadErr.Path)
			}
		})
	}
}
