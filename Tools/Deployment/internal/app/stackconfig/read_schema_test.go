package stackconfig_test

// read_schema_test.go covers Read over both manifest schemas: the persisted catalog stack is
// read from an old single-harness manifest and from a multi-harness manifest alike.

import (
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
)

const oldFormatWithStack = `schema_version: "1"
harness_id: ghcp-cli
updated_at: "2026-11-02T09:15:00Z"
entries:
  - ref:
      kind: agent
      key: reviewer
    target_path: .github/agents/reviewer.agent.md
    version: "1.0.0"
    harness_version: ""
    injections_version: ""
    content_hash: sha256:00
    deployed_at: "2026-11-02T09:00:00Z"
catalog_stack:
  - default: true
`

const newFormatWithStack = `schema_version: "2"
harnesses:
  - harness_id: ghcp-cli
    updated_at: "2026-11-02T09:15:00Z"
  - harness_id: opencode
    updated_at: "2026-11-03T09:15:00Z"
catalog_stack:
  - default: true
entries:
  - harness: ghcp-cli
    ref:
      kind: agent
      key: reviewer
    target_path: .github/agents/reviewer.agent.md
    version: "1.0.0"
    harness_version: ""
    injections_version: ""
    content_hash: sha256:00
    deployed_at: "2026-11-02T09:00:00Z"
  - harness: opencode
    ref:
      kind: agent
      key: reviewer
    target_path: .opencode/agents/reviewer.md
    version: "1.0.0"
    harness_version: ""
    injections_version: ""
    content_hash: sha256:01
    deployed_at: "2026-11-03T09:00:00Z"
`

func requireDefaultOnlyStack(t *testing.T, workspace string) {
	t.Helper()

	s, ok, err := stackconfig.Read(workspace)

	if err != nil || !ok {
		t.Fatalf("Read = (_, %v, %v), want the persisted stack", ok, err)
	}
	if len(s.Layers) != 1 || !s.Layers[0].Default {
		t.Errorf("Layers = %+v, want the single default layer", s.Layers)
	}
}

func TestRead_OldFormatManifest_ReadsTheStackEvenWithEntriesOfAnyHarness(t *testing.T) {
	requireDefaultOnlyStack(t, writeRawManifest(t, oldFormatWithStack))
}

func TestRead_MultiHarnessManifest_ReadsTheStack(t *testing.T) {
	requireDefaultOnlyStack(t, writeRawManifest(t, newFormatWithStack))
}
