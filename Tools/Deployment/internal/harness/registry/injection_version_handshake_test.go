package registry_test

// injection_version_handshake_test.go specifies that the injection version an external module
// receives over the protocol (the "versions" payload of a frontmatter request) is the
// frontmatter version of the injection file next to the harness descriptor, or empty when
// that file does not exist, and never a version declared in the descriptor.
//
// The external module is this test binary re-invoked as a subprocess; it writes the
// "versions" object of each frontmatter request it receives into a capture file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/external"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/transform"
)

const (
	captureModeEnv = "MOSAIC_TEST_REGISTRY_VERSIONS_CAPTURE"
	captureFileEnv = "MOSAIC_TEST_REGISTRY_CAPTURE_FILE"
)

func TestMain(m *testing.M) {
	if os.Getenv(captureModeEnv) == "1" {
		runVersionsCaptureModule()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runVersionsCaptureModule answers handshake, descriptor and frontmatter requests; for each
// frontmatter request it writes the raw "versions" params object to the capture file.
func runVersionsCaptureModule() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	reply := func(id string, result map[string]any) {
		_ = enc.Encode(map[string]any{"protocol": external.ProtocolVersion, "id": id, "result": result})
	}
	for {
		var req map[string]any
		if err := dec.Decode(&req); err != nil {
			return
		}
		id, _ := req["id"].(string)
		method, _ := req["method"].(string)
		switch method {
		case "handshake":
			reply(id, map[string]any{
				"protocol": external.ProtocolVersion,
				"harness":  map[string]any{"id": "capture", "display_name": "capture", "tier": "external", "usable": true},
			})
		case "descriptor":
			reply(id, map[string]any{"descriptor": map[string]any{"schema_version": "1", "id": "capture", "display_name": "capture"}})
		case "frontmatter":
			params, _ := req["params"].(map[string]any)
			raw, _ := json.Marshal(params["versions"])
			_ = os.WriteFile(os.Getenv(captureFileEnv), raw, 0o644)
			reply(id, map[string]any{"set": []any{}, "remove": []any{}, "key_order": []any{}})
		default:
			_ = enc.Encode(map[string]any{
				"protocol": external.ProtocolVersion, "id": id,
				"error": map[string]any{"code": "unsupported_method", "message": "capture: " + method},
			})
		}
	}
}

// capturedInjectionVersions resolves an external harness (declaring injections_version 9.9.9
// in its descriptor, with the given shared injection file or none), deploys an agent
// through the production transform, and returns the injections versions the module
// received on the wire.
func capturedInjectionVersions(t *testing.T, sharedFile *string) (injections, orchestrator string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	capture := filepath.Join(t.TempDir(), "versions.json")
	t.Setenv(captureModeEnv, "1")
	t.Setenv(captureFileEnv, capture)

	id := uniqueID(t, "wire")
	root := makeRootWithOpenCodeContent(t)
	addProtocolBinary(t, root, id, id, self)
	dir := filepath.Join(root, "MosaicDeploy", "harnesses", id)
	must(t, os.WriteFile(filepath.Join(dir, "harness.yaml"), []byte(tierYAML(id, "injections_version: \"9.9.9\"\n")), 0o644))
	writeTierFiles(t, root, id, sharedFile, tierStr(tierVersionedDoc("4.2.0")))

	reg, err := registry.Discover(registry.Options{MosaicRoot: root, AllowExternal: true})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	m := resolveTier(t, reg, id)
	src := []byte(agentSource)
	if _, err := transform.Apply(transform.Request{
		Source: src,
		Kind:   domain.ArtifactAgent,
		Key:    "some-agent",
		Module: m,
	}); err != nil {
		t.Fatalf("transform.Apply: %v", err)
	}

	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("module captured no frontmatter request: %v", err)
	}
	var wire struct {
		InjectionsVersion             string `json:"injections_version"`
		OrchestratorInjectionsVersion string `json:"orchestrator_injections_version"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("decode captured versions %q: %v", raw, err)
	}
	return wire.InjectionsVersion, wire.OrchestratorInjectionsVersion
}

// assertFileOrAbsent checks the contract for a wire version: the file's version or nothing,
// never the descriptor-declared value.
func assertFileOrAbsent(t *testing.T, field, got, fileVersion string) {
	t.Helper()
	if got == "9.9.9" {
		t.Errorf("wire %s = %q: the descriptor-declared version must not reach the module", field, got)
	}
	if got != "" && got != fileVersion {
		t.Errorf("wire %s = %q, want the file version %q or nothing", field, got, fileVersion)
	}
}

func TestExternalHandshake_InjectionsVersionIsFileVersionOrAbsent(t *testing.T) {
	got, gotOrch := capturedInjectionVersions(t, tierStr(tierVersionedDoc("3.1.0")))

	assertFileOrAbsent(t, "injections_version", got, "3.1.0")
	assertFileOrAbsent(t, "orchestrator_injections_version", gotOrch, "4.2.0")
}

func TestExternalHandshake_NoInjectionFile_NoDescriptorVersionOnTheWire(t *testing.T) {
	got, _ := capturedInjectionVersions(t, nil)

	assertFileOrAbsent(t, "injections_version", got, "")
}

// agentSource is a minimal generic agent file.
const agentSource = `---
name: some-agent
version: 1.0.0
---
body
`
