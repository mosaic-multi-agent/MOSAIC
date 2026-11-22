package app_test

// parity_claude_source_test.go holds the reference deployments for transforms whose source is
// Claude Code. A Claude Code deployment never grants the user-interaction tool, so a transform
// from it cannot carry that tool to the target. Its reference is a fresh deploy of the sample
// catalog with the user-interaction tool removed from every agent.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parityNoUserInteractionDeploys returns, per harness, a fresh deploy of the sample catalog
// without the user-interaction tool.
func parityNoUserInteractionDeploys(t *testing.T) map[string]string {
	t.Helper()
	root := parityCatalogWithoutUserInteraction(t)
	out := map[string]string{}
	for _, id := range parityHarnesses {
		out[id] = parityDeployUsing(t, parityServiceAt(t, root), id)
	}
	return out
}

// parityCatalogWithoutUserInteraction copies the sample catalog into a temp dir with
// user_interaction removed from every tools list.
func parityCatalogWithoutUserInteraction(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "parity", "catalog"))
	if err != nil {
		t.Fatalf("resolve parity catalog root: %v", err)
	}
	dst := t.TempDir()
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(string(b), ", user_interaction", "")
		return os.WriteFile(target, []byte(content), 0o644)
	})
	if err != nil {
		t.Fatalf("copy parity catalog: %v", err)
	}
	return dst
}
