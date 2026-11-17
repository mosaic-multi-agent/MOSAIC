package opencodev2_test

// Shared helpers for the opencodev2 package tests: sandbox construction,
// tool-local fixture access and a map-backed scope probe. The test binary also
// doubles as a fake child process (see spawn_launch_test.go).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runEnvironmentHelper()
	}
	os.Exit(m.Run())
}

func testContext() context.Context { return context.Background() }

// newSandbox builds a domain.Sandbox rooted under dir, creating the subject
// and control directories.
func newSandbox(t *testing.T, dir string) domain.Sandbox {
	t.Helper()
	sb := domain.Sandbox{
		Key:        domain.RunKey{RunID: "opencodev2-test-run", TestName: "opencodev2-test", RunNumber: 1},
		Root:       dir,
		SubjectDir: filepath.Join(dir, "subject"),
		ControlDir: filepath.Join(dir, "control"),
	}
	for _, d := range []string{sb.SubjectDir, sb.ControlDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("opencodev2 test: create %q: %v", d, err)
		}
	}
	return sb
}

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(append([]string{"..", "..", "..", "testdata", "opencodev2"}, parts...)...))
	if err != nil {
		t.Fatalf("opencodev2 test: resolving testdata path: %v", err)
	}
	return abs
}

func fixtureBundleDir(t *testing.T, name string) string {
	t.Helper()
	return testdataPath(t, name)
}

func readScopeFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataPath(t, "scopes", name))
	if err != nil {
		t.Fatalf("opencodev2 test: reading scope fixture %q: %v", name, err)
	}
	return data
}

func readStreamFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataPath(t, "streams", name))
	if err != nil {
		t.Fatalf("opencodev2 test: reading stream fixture %q: %v", name, err)
	}
	return data
}

// fixtureScopeProbe is an in-memory ScopeProbe keyed by scope name. A scope
// without an entry is absent; a scope in errs fails with that error. It
// records every scope name it was asked to read.
type fixtureScopeProbe struct {
	docs map[string][]byte
	errs map[string]error
	read *[]string
}

func (p fixtureScopeProbe) Read(scope domain.ConfigScope) ([]byte, error) {
	if p.read != nil {
		*p.read = append(*p.read, scope.Name)
	}
	if err, ok := p.errs[scope.Name]; ok {
		return nil, err
	}
	doc, ok := p.docs[scope.Name]
	if !ok {
		return nil, opencodev2.ErrScopeAbsent
	}
	return doc, nil
}

func minimalProvisionRequest(sb domain.Sandbox) domain.ProvisionRequest {
	return domain.ProvisionRequest{
		Sandbox:         sb,
		InterceptorPath: "interceptor",
		InterceptorArgs: []string{"intercept"},
	}
}
