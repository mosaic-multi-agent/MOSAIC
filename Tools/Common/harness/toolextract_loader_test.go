package harness_test

// Tests that tool extraction reads the agent file through the shared read-only
// loader: a file that cannot be read or decoded fails with the loader's error,
// and the tools of a loaded file are extracted unchanged.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/agentload"
	"mosaic-common/harness"
)

// malformedFrontmatterAgent has frontmatter the Markdown reader rejects.
const malformedFrontmatterAgent = "---\nname: reviewer\ndescription: [unterminated\n  tools: : :\n---\n\nbody\n"

var toolExtractors = []struct {
	name    string
	extract func(string) ([]string, error)
	tools   string
	want    []string
}{
	{"claude-code", harness.ExtractClaudeCodeTools, "tools: Read, Write", []string{"Read", "Write"}},
	{"ghcp-cli", harness.ExtractGHCPCLITools, "tools: ['edit', 'execute']", []string{"write", "shell"}},
}

func TestToolExtraction_MissingFile_FailsWithLoaderUnreadableError(t *testing.T) {
	for _, tc := range toolExtractors {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "gone", "agent.md")

			_, err := tc.extract(path)

			var loadErr *agentload.LoadError
			if !errors.As(err, &loadErr) {
				t.Fatalf("want a *agentload.LoadError, got %T: %v", err, err)
			}
			if !errors.Is(err, agentload.ErrUnreadable) {
				t.Errorf("want errors.Is(err, agentload.ErrUnreadable), got %v", err)
			}
			if !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("want the read error to stay reachable (fs.ErrNotExist), got %v", err)
			}
		})
	}
}

func TestToolExtraction_MalformedFrontmatter_FailsWithLoaderUndecodableError(t *testing.T) {
	for _, tc := range toolExtractors {
		t.Run(tc.name, func(t *testing.T) {
			path := writeAgentFile(t, malformedFrontmatterAgent)

			_, err := tc.extract(path)

			var loadErr *agentload.LoadError
			if !errors.As(err, &loadErr) {
				t.Fatalf("want a *agentload.LoadError, got %T: %v", err, err)
			}
			if !errors.Is(err, agentload.ErrUndecodable) {
				t.Errorf("want errors.Is(err, agentload.ErrUndecodable), got %v", err)
			}
		})
	}
}

func TestToolExtraction_LoadFailure_IsNeitherToolsSentinel(t *testing.T) {
	for _, tc := range toolExtractors {
		t.Run(tc.name, func(t *testing.T) {
			path := writeAgentFile(t, malformedFrontmatterAgent)

			_, err := tc.extract(path)

			if err == nil {
				t.Fatal("want an error for an undecodable agent file, got nil")
			}
			if errors.Is(err, harness.ErrToolsMissing) || errors.Is(err, harness.ErrToolsEmpty) {
				t.Errorf("a load failure is neither ErrToolsMissing nor ErrToolsEmpty: %v", err)
			}
		})
	}
}

func TestToolExtraction_LoadedFile_ExtractsToolsUnchanged(t *testing.T) {
	for _, tc := range toolExtractors {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "agent.md")
			content := "---\nname: reviewer\n" + tc.tools + "\n---\n\nBody.\n"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := tc.extract(path)

			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("tools = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("tools[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestToolExtraction_MissingToolsKey_StillReportsErrToolsMissing(t *testing.T) {
	for _, tc := range toolExtractors {
		t.Run(tc.name, func(t *testing.T) {
			path := writeAgentFile(t, "---\nname: reviewer\n---\n\nBody.\n")

			_, err := tc.extract(path)

			if !errors.Is(err, harness.ErrToolsMissing) {
				t.Errorf("want ErrToolsMissing, got %v", err)
			}
		})
	}
}
