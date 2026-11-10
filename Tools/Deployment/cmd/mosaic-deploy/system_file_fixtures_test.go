package main

// system_file_fixtures_test.go holds the copy helper that gives a fixture catalog root the
// protocol and the bundle. A catalog used as a stack layer on its own, or as the only layer,
// must carry both files: no layer of the stack means no fallback to another root.

import (
	"path/filepath"
	"testing"
)

// writeSystemFileCopies writes CommunicationProtocol.md and DeployedSections.md into the
// catalog root, using the fixture documents of this package.
func writeSystemFileCopies(t *testing.T, catalogRoot string) {
	t.Helper()
	writeStackFile(t, catalogRoot, "CommunicationProtocol.md", []byte(stackFixtureProtocol))
	writeStackFile(t, catalogRoot, filepath.FromSlash("DeployedSections.md"), []byte(minimalBundleDoc))
}
