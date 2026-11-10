package cli_test

// Tests for the report JSON source.kind values: the digest kinds are new, every
// existing value is unchanged.

import (
	"bytes"
	"encoding/json"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func TestEncodeReport_SourceKind_ByDomainKind(t *testing.T) {
	cases := []struct {
		kind domain.SourceKind
		want string
	}{
		{domain.SourceNotFound, "not_found"},
		{domain.SourceLogsRoot, "logs_root"},
		{domain.SourceSingleRun, "single_run"},
		{domain.SourceUnusable, "unusable"},
		{domain.SourceDigestFile, "digest_file"},
		{domain.SourceDigestDir, "digest_dir"},
	}
	for _, c := range cases {
		report := domain.Report{Source: domain.Source{Kind: c.kind, Path: "/p"}, Currency: domain.Currency}

		var buf bytes.Buffer
		if err := cli.EncodeReport(&buf, report); err != nil {
			t.Fatalf("EncodeReport returned error: %v", err)
		}
		var m struct {
			Source struct {
				Kind string `json:"kind"`
				Path string `json:"path"`
			} `json:"source"`
		}
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}

		if m.Source.Kind != c.want || m.Source.Path != "/p" {
			t.Errorf("kind %v encoded as %+v, want kind %q path /p", c.kind, m.Source, c.want)
		}
	}
}
