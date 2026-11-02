package cli_test

// End-to-end currency display of the text report: a digest stores tokens only and
// no currency, so a digest-loaded run is re-priced with the current pricing files
// and shows the current effective currency, like a live run.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/digest"
	"mosaic-log-analyzer/internal/logread"
	"mosaic-log-analyzer/internal/logscan"
	"mosaic-log-analyzer/internal/pricing"
)

// e2eRunWithPricing runs the CLI on real adapters whose project pricing file holds
// yaml (the global level is an empty temporary folder).
func e2eRunWithPricing(t *testing.T, yaml string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, filepath.FromSlash(pricing.ConfigRelPath))
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatalf("create pricing dir: %v", err)
	}
	if err := os.WriteFile(cfg, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write pricing file: %v", err)
	}
	svc := app.New(app.Deps{
		Source:      logscan.NewScanner(),
		Reader:      logread.NewReader(),
		Pricing:     pricing.NewStore(t.TempDir(), root),
		Digests:     digest.NewStore(),
		Clock:       newFakeClock(),
		Interaction: cli.NewInteraction(&bytes.Buffer{}, cli.Answers{}),
		WorkDir:     t.TempDir(),
		ToolVersion: "e2e",
	})
	var out, errOut bytes.Buffer
	code = cli.Execute(cli.Options{Service: svc, Stdout: &out, Stderr: &errOut}, args)
	return code, out.String(), errOut.String()
}

func TestDigestRun_TextReport_ShowsTheCurrentEffectiveCurrency(t *testing.T) {
	digestPath := e2eExport(t, e2eFixture("logs"), e2eRichRun)
	cases := []struct {
		name, yaml, want, absent string
	}{
		{"eur", "currency: EUR\n" + e2ePricingYAML, euro, "$"},
		{"usd stated", "currency: USD\n" + e2ePricingYAML, "$", euro},
		{"no currency key", e2ePricingYAML, "$", euro},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := e2eRunWithPricing(t, tc.yaml, "report", "--format", "text", "--path", digestPath)

			if code != cli.ExitSuccess {
				t.Fatalf("report exited %d: %s", code, stderr)
			}
			if !strings.Contains(stdout, "Money: "+tc.want) {
				t.Errorf("digest-loaded run must show money with %q\n%s", tc.want, stdout)
			}
			if strings.Contains(stdout, tc.absent) {
				t.Errorf("digest-loaded run must not show %q\n%s", tc.absent, stdout)
			}
		})
	}
}

func TestLiveRun_TextReport_ShowsTheEffectiveCurrency(t *testing.T) {
	code, stdout, stderr := e2eRunWithPricing(t, "currency: EUR\n"+e2ePricingYAML,
		"report", "--format", "text", "--path", e2eFixture("logs"), "--run", e2eRichRun)

	if code != cli.ExitSuccess {
		t.Fatalf("report exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Money: "+euro) || strings.Contains(stdout, "$") {
		t.Errorf("a live run under an EUR pricing file must show euros only\n%s", stdout)
	}
}
