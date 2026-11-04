package artifact_test

// Test that a store write after a tolerant read leaves the file in canonical
// form: every closing tag on its own line.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

func TestStoreWrite_AfterReadingTagsAfterContent_WritesCanonicalForm(t *testing.T) {
	canonical := canonicalCloseTags(loadCloseTagFixture(t))
	joined := canonical
	for _, section := range tolerantSections {
		joined = joinCloseTag(t, joined, section)
	}

	for _, eol := range lineEndings {
		t.Run(eol.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Orchestration.md")
			if err := os.WriteFile(path, []byte(eol.conv(joined)), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			store := artifact.NewFileStore(path)
			ctx := context.Background()
			state, err := store.Read(ctx)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			step := newTestStep(9, "builder#9", "PLANNING", "", domain.StatusSUCCESS,
				time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC), nil)

			if _, err := store.Apply(ctx, state, step); err != nil {
				t.Fatalf("Apply: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			assertClosingTagsOnOwnLines(t, string(data))
			reread, err := store.Read(ctx)
			if err != nil {
				t.Fatalf("Read after write: %v", err)
			}
			if len(reread.ExecutionLog) != 9 || len(reread.WorkflowNotes) != 2 {
				t.Errorf("after write: log=%d notes=%d, want 9/2", len(reread.ExecutionLog), len(reread.WorkflowNotes))
			}
		})
	}
}

// assertClosingTagsOnOwnLines fails unless each tolerant section's closing tag
// stands alone on its line.
func assertClosingTagsOnOwnLines(t *testing.T, doc string) {
	t.Helper()
	for _, section := range tolerantSections {
		tag := "</" + section + ">"
		if !strings.Contains(doc, tag) {
			t.Errorf("written file has no %s", tag)
			continue
		}
		alone := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(tag) + `[ \t\r]*$`)
		if !alone.MatchString(doc) {
			t.Errorf("%s is not on a line of its own in the written file", tag)
		}
		after := regexp.MustCompile(`(?m)\S[ \t]*` + regexp.QuoteMeta(tag))
		if after.MatchString(doc) {
			t.Errorf("%s still follows content on its line in the written file", tag)
		}
	}
}
