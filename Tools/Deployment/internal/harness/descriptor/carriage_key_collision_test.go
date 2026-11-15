package descriptor_test

// carriage_key_collision_test.go asserts that neither carriage key
// ("mosaic_carriage", "mosaic_carriage_markers") appears in the add, drop or
// key_order lists of any registered builtin harness descriptor.
//
// A collision would cause the descriptor to silently drop or reorder the
// carriage container, destroying every carried user key. The test lives in
// Deploy because the descriptors are Deploy's own files; the shared
// agent-format layer must not depend on them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentformat"
	"mosaic-deploy/internal/harness/descriptor"
)

// TestCarriageKeyCollision_NotInDescriptorFields verifies the rule above for
// every builtin descriptor.
func TestCarriageKeyCollision_NotInDescriptorFields(t *testing.T) {
	builtinDir, err := filepath.Abs(filepath.Join("..", "builtin"))
	if err != nil {
		t.Fatalf("resolve builtin directory: %v", err)
	}

	yamlFiles, err := findBuiltinDescriptorYAMLs(builtinDir)
	if err != nil {
		t.Fatalf("find builtin descriptor YAMLs in %s: %v", builtinDir, err)
	}
	if len(yamlFiles) == 0 {
		t.Fatalf("no builtin descriptor YAML files found under %s; cannot check for collisions", builtinDir)
	}

	for _, yamlPath := range yamlFiles {
		yamlPath := yamlPath
		t.Run(strings.TrimPrefix(yamlPath, builtinDir+string(filepath.Separator)), func(t *testing.T) {
			src, readErr := os.ReadFile(yamlPath)
			if readErr != nil {
				t.Fatalf("read %s: %v", yamlPath, readErr)
			}
			d, parseErr := descriptor.Parse(src, yamlPath)
			if parseErr != nil {
				t.Fatalf("parse %s: %v", yamlPath, parseErr)
			}

			for _, carriageKey := range agentformat.CarriageKeys() {
				for _, addField := range d.Frontmatter.Add {
					if addField.Key == carriageKey {
						t.Errorf("descriptor %s: carriage key %q appears in frontmatter.add; this would cause the descriptor to overwrite the carriage container", yamlPath, carriageKey)
					}
				}
				for _, dropKey := range d.Frontmatter.Drop {
					if dropKey == carriageKey {
						t.Errorf("descriptor %s: carriage key %q appears in frontmatter.drop; this would cause the descriptor to silently remove the carriage container", yamlPath, carriageKey)
					}
				}
				for _, orderKey := range d.Frontmatter.KeyOrder {
					if orderKey == carriageKey {
						t.Errorf("descriptor %s: carriage key %q appears in frontmatter.key_order; this would cause the descriptor to reorder or expose the carriage container", yamlPath, carriageKey)
					}
				}
			}
		})
	}
}

// findBuiltinDescriptorYAMLs returns all YAML files found in the immediate
// subdirectories of the builtin harness directory (one subdirectory per
// harness, each holding one descriptor YAML file).
func findBuiltinDescriptorYAMLs(builtinDir string) ([]string, error) {
	var results []string
	entries, err := os.ReadDir(builtinDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(builtinDir, entry.Name())
		subEntries, readErr := os.ReadDir(subdir)
		if readErr != nil {
			return nil, readErr
		}
		for _, sub := range subEntries {
			if !sub.IsDir() && strings.HasSuffix(sub.Name(), ".yaml") {
				results = append(results, filepath.Join(subdir, sub.Name()))
			}
		}
	}
	return results, nil
}
