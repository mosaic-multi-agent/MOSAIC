package session

import (
	"os"
	"path/filepath"
	"strings"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// expandInputWildcards replaces every Input entry containing '*' with the
// artifacts that exist in the run folder and match it, in lexical order,
// through the shared matcher. An entry without a match yields nothing. Entries
// are matched run-scoped, so an unprefixed entry is first placed under the run
// folder; the result holds run-scoped paths for expanded entries and leaves
// the other entries unchanged.
func expandInputWildcards(runFolder, runID string, inputs []string) []string {
	if !anyWildcard(inputs) {
		return inputs
	}
	fsys := os.DirFS(filepath.Dir(runFolder))
	prefix := domain.RunScopedFolder(runID) + "/"
	out := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if !strings.Contains(in, "*") {
			out = append(out, in)
			continue
		}
		scoped := resolveToRunScoped([]string{in}, prefix)[0]
		out = append(out, artifact.ExpandWildcard(fsys, scoped)...)
	}
	return out
}

// listOutputWildcards replaces every declared Output containing '*' with the
// files under root that match it, through the shared matcher; the other
// entries are kept as declared. Without a match a wildcard entry yields
// nothing.
func listOutputWildcards(root string, declared []string) []string {
	if !anyWildcard(declared) {
		return declared
	}
	fsys := os.DirFS(root)
	out := make([]string, 0, len(declared))
	for _, d := range declared {
		if !strings.Contains(d, "*") {
			out = append(out, d)
			continue
		}
		out = append(out, artifact.ExpandWildcard(fsys, d)...)
	}
	return out
}

// anyWildcard reports whether any entry contains '*'.
func anyWildcard(paths []string) bool {
	for _, p := range paths {
		if strings.Contains(p, "*") {
			return true
		}
	}
	return false
}
