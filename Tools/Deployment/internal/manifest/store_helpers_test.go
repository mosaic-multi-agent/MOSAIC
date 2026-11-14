package manifest_test

// Builders for old-format (schema "1") manifest text used by the migration tests.

import "strings"

// oldManifestText returns an old-format manifest whose second line is harnessLine (for example
// "harness_id: opencode") followed by the given entry blocks.
func oldManifestText(harnessLine string, entries ...string) string {
	var b strings.Builder
	b.WriteString("schema_version: \"1\"\n")
	b.WriteString(harnessLine + "\n")
	b.WriteString("updated_at: \"2026-11-02T09:15:00Z\"\n")
	if len(entries) == 0 {
		b.WriteString("entries: []\n")
		return b.String()
	}
	b.WriteString("entries:\n")
	for _, e := range entries {
		b.WriteString(e)
	}
	return b.String()
}

// oldEntryText returns one old-format entry block.
func oldEntryText(kind, key, targetPath string) string {
	return "  - ref:\n" +
		"      kind: " + kind + "\n" +
		"      key: " + key + "\n" +
		"    target_path: " + targetPath + "\n" +
		"    version: \"1.0.0\"\n" +
		"    harness_version: \"\"\n" +
		"    injections_version: \"\"\n" +
		"    content_hash: sha256:00\n" +
		"    deployed_at: \"2026-11-01T10:00:00Z\"\n"
}
