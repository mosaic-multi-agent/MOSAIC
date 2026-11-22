package opencodev2_test

// Tests for the interceptor bridges embedded in the generated plugin: one per
// phase, each naming the interceptor executable, the driver's arguments, the
// control directory, the harness and the phase explicitly. The control
// directory is never inferred from a working directory, because the plugin runs
// inside the V2 server wherever that happens to be.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/harness/opencodev2"
)

func jsonLiteral(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal %q: %v", s, err)
	}
	return string(b)
}

func TestBridges_EmbedTheInterceptorExecutableVerbatim(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.InterceptorPath = "/abs/path/to/mosaic-agent-test"

	content := provisionInterceptor(t, sb, req)

	if !strings.Contains(content, jsonLiteral(t, req.InterceptorPath)) {
		t.Errorf("plugin does not embed the interceptor path %q as a JSON literal; content=\n%s", req.InterceptorPath, content)
	}
}

func TestBridges_PreserveDriverSuppliedInterceptorArgsAheadOfTheAdditions(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.InterceptorArgs = []string{"intercept", "--workspace", sb.SubjectDir}

	content := provisionInterceptor(t, sb, req)

	last := -1
	for _, arg := range req.InterceptorArgs {
		idx := strings.Index(content, jsonLiteral(t, arg))
		if idx < 0 {
			t.Fatalf("plugin does not embed driver-supplied argument %q; content=\n%s", arg, content)
		}
		if idx < last {
			t.Errorf("driver-supplied argument %q appears before an earlier one; order must be preserved", arg)
		}
		last = idx
	}
	if ctl := strings.Index(content, jsonLiteral(t, "--control-dir")); ctl < strings.Index(content, jsonLiteral(t, "--workspace")) {
		t.Errorf("--control-dir precedes the driver-supplied arguments; it must follow them")
	}
}

func TestProvision_EmptyInterceptorPathFailsAndWritesNothing(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.InterceptorPath = ""

	_, err := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}}).Provision(testContext(), req)

	if err == nil {
		t.Fatalf("Provision: want an error for an empty InterceptorPath")
	}
	if !strings.Contains(err.Error(), "InterceptorPath") {
		t.Errorf("error = %q, want it to name InterceptorPath", err.Error())
	}
	if entries, rerr := os.ReadDir(pluginsDir(sb)); rerr == nil && len(entries) != 0 {
		t.Errorf("plugins directory holds %d entries after the refusal, want none", len(entries))
	}
}

func TestProvision_FailedBridgeBuildProvisionsNoPluginsEvenWithALoggerBundle(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.InterceptorPath = ""
	req.LoggerBundleDir = fixtureBundleDir(t, "fixture-bundle")

	_, err := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}}).Provision(testContext(), req)

	if err == nil || !strings.Contains(err.Error(), "InterceptorPath") {
		t.Fatalf("Provision error = %v, want one naming InterceptorPath", err)
	}
	for _, name := range []string{opencodev2.PluginFileName, "mosaic-logger.ts"} {
		if _, statErr := os.Stat(filepath.Join(pluginsDir(sb), name)); statErr == nil {
			t.Errorf("%s was written although the bridge build failed", name)
		}
	}
}
