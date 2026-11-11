package transform_test

// owned_content_fixtures_test.go holds the tool-local deployed agent text and the edit helpers
// shared by the Deploy-owned content tests.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// ownedAgentLF is a deployed agent: frontmatter, text outside any region, two top-level core
// regions, a project region nested in one of them, and a custom region nested in the other.
const ownedAgentLF = `---
id: 99
version: 1.0.0
name: owned-agent
description: Agent used for Deploy-owned content tests
tools: [file_read]
---

Outside prose that Deploy owns.

<Identity type="core">
Identity core prose.
<IdentityExtension type="project">
</IdentityExtension>
</Identity>

<Constraints type="core">
Constraint core prose.
<HarnessConstraints type="managed">
Harness constraint text.
</HarnessConstraints>
<TeamNotes type="custom">
</TeamNotes>
</Constraints>
`

// replaceOnce returns text with the first occurrence of old replaced by repl; it fails the
// test when old is absent, so a fixture change cannot silently turn a case into a no-op.
func replaceOnce(t *testing.T, text, old, repl string) string {
	t.Helper()
	if !strings.Contains(text, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(text, old, repl, 1)
}

// mustOwned computes the owned record of an agent text or fails the test.
func mustOwned(t *testing.T, kind domain.ArtifactKind, content []byte) domain.OwnedContent {
	t.Helper()
	got, err := transform.OwnedContentOf(kind, content)
	if err != nil {
		t.Fatalf("OwnedContentOf(%s): %v", kind, err)
	}
	return got
}

// mustOwnedAgent computes the owned record of an agent text or fails the test.
func mustOwnedAgent(t *testing.T, text string) domain.OwnedContent {
	t.Helper()
	return mustOwned(t, domain.ArtifactAgent, []byte(text))
}
