package config_test

// Tests for SeedConfigFiles: a missing user-config.yaml / tool-config.yaml is created from
// the commented template, an existing path is never touched, and the seeded files load to
// today's defaults. Templates come from this package's own testdata/, never the live ones.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"mosaic-deploy/internal/config"
)

const (
	userTemplateFixture = "testdata/user-config.template.yaml"
	toolTemplateFixture = "testdata/tool-config.template.yaml"
)

// readFixture returns the bytes of a testdata fixture.
func readFixture(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", rel, err)
	}
	return data
}

// templateFS returns an in-memory template FS holding both commented templates at its root.
func templateFS(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"user-config.yaml": &fstest.MapFile{Data: readFixture(t, userTemplateFixture)},
		"tool-config.yaml": &fstest.MapFile{Data: readFixture(t, toolTemplateFixture)},
	}
}

func seededPath(root, name string) string {
	return filepath.Join(root, "MosaicDeploy", "config", name)
}

func TestSeedConfigFiles_MissingFiles_CreatedWithTemplateBytes(t *testing.T) {
	root := t.TempDir() // no MosaicDeploy/config directory yet

	err := config.SeedConfigFiles(root, templateFS(t))
	if err != nil {
		t.Fatalf("SeedConfigFiles: %v", err)
	}

	for name, fixture := range map[string]string{
		"user-config.yaml": userTemplateFixture,
		"tool-config.yaml": toolTemplateFixture,
	} {
		got, err := os.ReadFile(seededPath(root, name))
		if err != nil {
			t.Fatalf("%s was not created: %v", name, err)
		}
		if string(got) != string(readFixture(t, fixture)) {
			t.Errorf("%s content differs from the template (comments must be preserved byte for byte)", name)
		}
	}
}

func TestSeedConfigFiles_ExistingFile_LeftByteIdentical(t *testing.T) {
	root := makeRoot(t)
	existing := []byte("# my own file\nschema_version: \"1\"\ntier_models:\n  claude-code:\n    HIGH: x\n")
	writeUserConfig(t, root, existing)

	err := config.SeedConfigFiles(root, templateFS(t))
	if err != nil {
		t.Fatalf("SeedConfigFiles: %v", err)
	}

	got, rerr := os.ReadFile(seededPath(root, "user-config.yaml"))
	if rerr != nil {
		t.Fatalf("reading user-config.yaml: %v", rerr)
	}
	if string(got) != string(existing) {
		t.Errorf("existing user-config.yaml was modified by seeding:\n%s", got)
	}
	// The sibling that was missing is still seeded: both names are attempted.
	tool, terr := os.ReadFile(seededPath(root, "tool-config.yaml"))
	if terr != nil {
		t.Fatalf("missing tool-config.yaml was not seeded: %v", terr)
	}
	if string(tool) != string(readFixture(t, toolTemplateFixture)) {
		t.Errorf("seeded tool-config.yaml differs from the template")
	}
}

func TestSeedConfigFiles_EmptyExistingFile_NotOverwritten(t *testing.T) {
	root := makeRoot(t)
	writeToolConfig(t, root, []byte{})

	err := config.SeedConfigFiles(root, templateFS(t))
	if err != nil {
		t.Fatalf("SeedConfigFiles: %v", err)
	}

	got, rerr := os.ReadFile(seededPath(root, "tool-config.yaml"))
	if rerr != nil {
		t.Fatalf("reading tool-config.yaml: %v", rerr)
	}
	if len(got) != 0 {
		t.Errorf("an existing empty tool-config.yaml was overwritten (%d bytes)", len(got))
	}
	if _, serr := os.Stat(seededPath(root, "user-config.yaml")); serr != nil {
		t.Errorf("missing user-config.yaml was not seeded: %v", serr)
	}
}

func TestSeedConfigFiles_SeededToolConfig_LoadsToDefaults(t *testing.T) {
	root := t.TempDir()

	if err := config.SeedConfigFiles(root, templateFS(t)); err != nil {
		t.Fatalf("SeedConfigFiles: %v", err)
	}

	if _, err := os.Stat(seededPath(root, "tool-config.yaml")); err != nil {
		t.Fatalf("tool-config.yaml was not created: %v", err)
	}
	got, err := config.NewToolConfigStore(root).Load()
	if err != nil {
		t.Fatalf("Load seeded tool-config.yaml: %v", err)
	}
	if want := config.DefaultToolConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("seeded tool config = %+v, want defaults %+v", got, want)
	}
}

func TestSeedConfigFiles_SeededUserConfig_LoadsTemplateValues(t *testing.T) {
	root := t.TempDir()

	if err := config.SeedConfigFiles(root, templateFS(t)); err != nil {
		t.Fatalf("SeedConfigFiles: %v", err)
	}

	if _, err := os.Stat(seededPath(root, "user-config.yaml")); err != nil {
		t.Fatalf("user-config.yaml was not created: %v", err)
	}
	got, err := config.NewUserConfigStore(root).Load()
	if err != nil {
		t.Fatalf("Load seeded user-config.yaml: %v", err)
	}
	if got.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want \"1\"", got.SchemaVersion)
	}
	if len(got.TierModels) != 0 || len(got.CustomModelIDs) != 0 || len(got.ToolDestinations) != 0 {
		t.Errorf("seeded user config should hold no entries, got %+v", got)
	}
}

func TestSeedConfigFiles_TemplateMissingFromFS_CreatesNothingForIt(t *testing.T) {
	root := t.TempDir()
	templates := fstest.MapFS{
		"tool-config.yaml": &fstest.MapFile{Data: readFixture(t, toolTemplateFixture)},
	}

	err := config.SeedConfigFiles(root, templates)
	if err != nil {
		t.Fatalf("a missing template must not raise an error, got: %v", err)
	}

	if _, serr := os.Stat(seededPath(root, "tool-config.yaml")); serr != nil {
		t.Errorf("tool-config.yaml should still be seeded: %v", serr)
	}
	if _, serr := os.Stat(seededPath(root, "user-config.yaml")); !os.IsNotExist(serr) {
		t.Errorf("user-config.yaml must not be created without a template, stat err = %v", serr)
	}
}

func TestSeedConfigFiles_TemplateIsADirectory_CreatesNothingForIt(t *testing.T) {
	root := t.TempDir()
	templates := fstest.MapFS{
		"user-config.yaml/inner.yaml": &fstest.MapFile{Data: []byte("x: 1\n")},
		"tool-config.yaml":            &fstest.MapFile{Data: readFixture(t, toolTemplateFixture)},
	}

	err := config.SeedConfigFiles(root, templates)
	if err != nil {
		t.Fatalf("a directory template must not raise an error, got: %v", err)
	}

	if _, serr := os.Stat(seededPath(root, "user-config.yaml")); !os.IsNotExist(serr) {
		t.Errorf("user-config.yaml must not be created from a directory, stat err = %v", serr)
	}
	if _, serr := os.Stat(seededPath(root, "tool-config.yaml")); serr != nil {
		t.Errorf("tool-config.yaml should still be seeded: %v", serr)
	}
}

func TestSeedConfigFiles_ConfigDirectoryNotCreatable_ReturnsErrorWithoutPanic(t *testing.T) {
	root := t.TempDir()
	// A regular file where the MosaicDeploy directory belongs makes both creations fail.
	if err := os.WriteFile(filepath.Join(root, "MosaicDeploy"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := config.SeedConfigFiles(root, templateFS(t))

	if err == nil {
		t.Fatal("an I/O failure while creating the config files must be returned")
	}
}

func TestSeedConfigFiles_CalledTwice_SecondCallLeavesEditsAlone(t *testing.T) {
	root := t.TempDir()
	if err := config.SeedConfigFiles(root, templateFS(t)); err != nil {
		t.Fatalf("first SeedConfigFiles: %v", err)
	}
	edited := []byte("# edited after first start\nschema_version: \"1\"\n")
	if err := os.WriteFile(seededPath(root, "user-config.yaml"), edited, 0o644); err != nil {
		t.Fatal(err)
	}

	err := config.SeedConfigFiles(root, templateFS(t))
	if err != nil {
		t.Fatalf("second SeedConfigFiles: %v", err)
	}

	got, rerr := os.ReadFile(seededPath(root, "user-config.yaml"))
	if rerr != nil || string(got) != string(edited) {
		t.Errorf("second call changed the edited file: err=%v content=%q", rerr, got)
	}
	tool, terr := os.ReadFile(seededPath(root, "tool-config.yaml"))
	if terr != nil || string(tool) != string(readFixture(t, toolTemplateFixture)) {
		t.Errorf("second call changed tool-config.yaml: err=%v", terr)
	}
}

func TestSeedConfigFiles_NilTemplates_CreatesNothing(t *testing.T) {
	root := t.TempDir()

	if err := config.SeedConfigFiles(root, nil); err != nil {
		t.Fatalf("nil templates must behave as an empty FS, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "MosaicDeploy")); !os.IsNotExist(err) {
		t.Errorf("nothing should be created for nil templates, stat err = %v", err)
	}
}

func TestSeedConfigFiles_DirectoryAtDestination_NotTouchedAndNoError(t *testing.T) {
	root := makeRoot(t)
	dest := seededPath(root, "user-config.yaml")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	err := config.SeedConfigFiles(root, templateFS(t))
	if err != nil {
		t.Fatalf("an occupied destination must not be an error, got: %v", err)
	}

	info, serr := os.Stat(dest)
	if serr != nil || !info.IsDir() {
		t.Errorf("the directory at the destination must stay a directory, stat = %v, %v", info, serr)
	}
	if _, serr := os.Stat(seededPath(root, "tool-config.yaml")); serr != nil {
		t.Errorf("tool-config.yaml should still be seeded: %v", serr)
	}
}
