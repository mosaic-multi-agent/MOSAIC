package pricing_test

// Tests for the built-in pricing template that is written whenever the pricing
// file is created because it was absent.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/pricing"
)

var (
	commentedEntryName = regexp.MustCompile(`^#\s+([A-Za-z0-9][A-Za-z0-9._-]*):\s*$`)
	commentedLongCtx   = regexp.MustCompile(`^#\s*long_context:`)
)

// createdByPut writes one entry into an absent file and returns the store and
// the raw file content.
func createdByPut(t *testing.T, entry domain.ModelPricing) (*pricing.Store, string) {
	t.Helper()
	store := pricing.NewStore(t.TempDir(), t.TempDir())
	if _, err := store.Put(context.Background(), domain.LevelProject, entry); err != nil {
		t.Fatalf("Put: %v", err)
	}
	return store, readFile(t, store.Path())
}

// commentedExampleEntries counts commented entry headers whose next lines show
// an input rate, i.e. commented example entries.
func commentedExampleEntries(template string) int {
	lines := strings.Split(template, "\n")
	count := 0
	for i, line := range lines {
		m := commentedEntryName.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || m[1] == "models" || m[1] == "long_context" {
			continue
		}
		for j := i + 1; j < len(lines) && j <= i+8; j++ {
			if strings.Contains(lines[j], "input:") {
				count++
				break
			}
		}
	}
	return count
}

func TestTemplate_PutOnAbsentFile_WritesTemplateBeforeTheModelsMap(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))

	template := leadingCommentBlock(raw)
	if strings.TrimSpace(template) == "" {
		t.Fatal("creating an absent file must write the template as a leading comment block")
	}
	rest := raw[len(template):]
	if !strings.HasPrefix(strings.TrimSpace(rest), "models:") {
		t.Errorf("the models map must follow the template directly; got %.40q", rest)
	}
	if !strings.Contains(rest, "first-model") {
		t.Error("the entered entry must be written after the template")
	}
}

func TestTemplate_DocumentsEachWireKeyAndConvention(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	template := leadingCommentBlock(raw)

	for _, key := range []string{"models", "input", "cache_read", "cache_write_5m", "cache_write_1h", "output", "long_context", "threshold"} {
		if !strings.Contains(template, key) {
			t.Errorf("template must document the key %q", key)
		}
	}
	if !regexp.MustCompile(`(?i)million|1,?000,?000|per 1M`).MatchString(template) {
		t.Error("template must state that rates are USD per million tokens")
	}
	if !regexp.MustCompile(`(?i)quoted|in quotes|as a string`).MatchString(template) {
		t.Error("template must show or mention the quoted-string rate convention")
	}
	if !regexp.MustCompile(`(?i)fallback|assum|no (tier )?split|not (split|reported)`).MatchString(template) {
		t.Error("template must explain the 5m fallback for cache writes without a tier split")
	}
}

func TestTemplate_ContainsCommentedExamples_OneWithLongContext(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	template := leadingCommentBlock(raw)

	if n := commentedExampleEntries(template); n < 2 {
		t.Errorf("template must hold at least 2 commented example entries; found %d", n)
	}
	found := false
	for _, line := range strings.Split(template, "\n") {
		if commentedLongCtx.MatchString(strings.TrimSpace(line)) {
			found = true
		}
	}
	if !found {
		t.Error("template must include a commented example with a long_context block")
	}
}

func TestTemplate_ConsistsOnlyOfCommentAndBlankLines(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	template := leadingCommentBlock(raw)

	if strings.TrimSpace(template) == "" {
		t.Fatal("a created file must begin with a non-empty template")
	}
	if len(activeLines(template)) != 0 {
		t.Errorf("template must hold no active YAML; got %v", activeLines(template))
	}
	// The template is exactly the head of the file: nothing active precedes models.
	active := activeLines(raw)
	if len(active) == 0 || strings.TrimSpace(active[0]) != "models:" {
		t.Errorf("first active line must be models:; got %v", active)
	}
}

func TestTemplate_LoadingTheTemplateAlone_YieldsEmptyTableAndNoFindings(t *testing.T) {
	_, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	templateOnly := leadingCommentBlock(raw)
	if strings.TrimSpace(templateOnly) == "" {
		t.Fatal("a created file must begin with a non-empty template")
	}
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, templateOnly)

	table, findings, err := pricing.NewStore(t.TempDir(), root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load of the template alone: %v", err)
	}
	if table.Len() != 0 || len(findings) != 0 {
		t.Errorf("template alone must load as empty with no findings; got %d entries, %v", table.Len(), findings)
	}
}

func TestTemplate_WrittenFileLoadsBackToExactlyTheEnteredEntries(t *testing.T) {
	store, _ := createdByPut(t, newMinimalEntry(t, "first-model"))
	second := newEntryWithLongContext(t, "second-model")
	if _, err := store.Put(context.Background(), domain.LevelProject, second); err != nil {
		t.Fatalf("second Put: %v", err)
	}

	table, findings, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("no findings expected (commented examples are not entries); got %v", findings)
	}
	models := table.Models()
	if len(models) != 2 || models[0] != "first-model" || models[1] != "second-model" {
		t.Fatalf("table must hold exactly the entered entries; got %v", models)
	}
	if got, _ := table.Lookup("first-model"); got != asProject(newMinimalEntry(t, "first-model")) {
		t.Errorf("first-model = %+v", got)
	}
	if got, _ := table.Lookup("second-model"); got != asProject(second) {
		t.Errorf("second-model = %+v, want %+v", got, second)
	}
}

func TestTemplate_SurvivesLaterPuts(t *testing.T) {
	store, raw := createdByPut(t, newMinimalEntry(t, "first-model"))
	before := leadingCommentBlock(raw)

	if _, err := store.Put(context.Background(), domain.LevelProject, newEntryWithLongContext(t, "second-model")); err != nil {
		t.Fatalf("second Put: %v", err)
	}
	if _, err := store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "first-model")); err != nil {
		t.Fatalf("replacing Put: %v", err)
	}

	after := leadingCommentBlock(readFile(t, store.Path()))
	if after != before {
		t.Errorf("the template documentation must be kept verbatim across Puts\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if commentedExampleEntries(after) < 2 {
		t.Error("commented examples must survive later Puts")
	}
}

func TestTemplate_NotAddedToAnExistingFile(t *testing.T) {
	root, path := makeRootWithConfigDir(t)
	writePricingYAML(t, path, wellFormedOneModel)
	store := pricing.NewStore(t.TempDir(), root)

	if _, err := store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "added-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if tmpl := leadingCommentBlock(readFile(t, path)); strings.Contains(tmpl, "long_context") {
		t.Errorf("the template is written only when the file is absent; got head %q", tmpl)
	}
}

func TestTemplate_WrittenWhenOnlyTheParentDirectoryIsMissing(t *testing.T) {
	root := t.TempDir()
	store := pricing.NewStore(t.TempDir(), root)

	if _, err := store.Put(context.Background(), domain.LevelProject, newMinimalEntry(t, "first-model")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := os.Stat(filepath.Dir(store.Path())); err != nil {
		t.Fatalf("parent directory must be created: %v", err)
	}
	if !strings.Contains(leadingCommentBlock(readFile(t, store.Path())), "long_context") {
		t.Error("creating the file must write the template")
	}
}
