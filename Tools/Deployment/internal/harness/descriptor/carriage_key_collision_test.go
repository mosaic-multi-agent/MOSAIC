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
	"testing"

	"mosaic-common/agentformat"
	"mosaic-common/harness/builtindesc"
	"mosaic-deploy/internal/harness/descriptor"
)

// TestCarriageKeyCollision_NotInDescriptorFields verifies the rule above for
// every builtin descriptor.
func TestCarriageKeyCollision_NotInDescriptorFields(t *testing.T) {
	ids := builtindesc.IDs()
	if len(ids) == 0 {
		t.Fatal("no builtin descriptors found in Common; cannot check for collisions")
	}

	for _, id := range ids {
		yamlPath := "builtin:" + id
		t.Run(id, func(t *testing.T) {
			src, ok := builtindesc.Raw(id)
			if !ok || len(src) == 0 {
				t.Fatalf("builtin descriptor %s missing from Common", id)
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
