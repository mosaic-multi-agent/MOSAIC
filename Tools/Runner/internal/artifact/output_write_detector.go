package artifact

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mosaic-run/internal/domain"
)

// outputWriteDetector is the filesystem implementation of
// domain.OutputWriteDetector. It compares file content by digest and expands
// wildcard segments against the filesystem.
type outputWriteDetector struct{}

// NewOutputWriteDetector returns the filesystem implementation of
// domain.OutputWriteDetector (content digest comparison; wildcard expansion
// against the filesystem).
func NewOutputWriteDetector() domain.OutputWriteDetector {
	return &outputWriteDetector{}
}

// writeSnapshot is the detector-private baseline state: the content digest of
// every concrete path that existed and was readable when the baseline was taken.
// Paths that existed but could not be read map to a nil-free marker entry in
// unreadable so a later readable state still counts as a change.
type writeSnapshot struct {
	digests    map[string][sha256.Size]byte
	unreadable map[string]bool
}

func (d *outputWriteDetector) Baseline(_ context.Context, q domain.OutputQuery) domain.OutputBaseline {
	snap := writeSnapshot{
		digests:    map[string][sha256.Size]byte{},
		unreadable: map[string]bool{},
	}
	for _, entry := range q.Declared {
		for _, p := range expandDeclared(q.Root, entry) {
			present, sum, readable := inspect(q.Root, p)
			if !present {
				continue
			}
			if readable {
				snap.digests[p] = sum
			} else {
				snap.unreadable[p] = true
			}
		}
	}
	return domain.OutputBaseline{Query: q, Snapshot: snap}
}

func (d *outputWriteDetector) Written(_ context.Context, baseline domain.OutputBaseline) []string {
	q := baseline.Query
	snap, _ := baseline.Snapshot.(writeSnapshot)
	var written []string
	seen := map[string]bool{}
	for _, entry := range q.Declared {
		for _, p := range expandDeclared(q.Root, entry) {
			if seen[p] {
				continue
			}
			seen[p] = true
			present, sum, readable := inspect(q.Root, p)
			if !present {
				continue
			}
			if !readable {
				written = append(written, p)
				continue
			}
			before, existed := snap.digests[p]
			if !existed || before != sum {
				written = append(written, p)
			}
		}
	}
	return written
}

// inspect reports whether p exists under root and, when it does, its content
// digest. A path that exists but cannot be read (permissions, directory, I/O
// error) is present and not readable.
func inspect(root, p string) (present bool, sum [sha256.Size]byte, readable bool) {
	full := filepath.Join(root, filepath.FromSlash(p))
	if _, err := os.Stat(full); err != nil {
		if errors.Is(err, fs.ErrNotExist) || absentFromParent(full) {
			return false, sum, false
		}
		return true, sum, false
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return true, sum, false
	}
	return true, sha256.Sum256(data), true
}

// absentFromParent reports whether full is provably absent: its directory is
// listable and holds no entry of that name. It covers names the operating
// system rejects outright (reported as an error other than "not exist"), which
// can never denote an existing file.
func absentFromParent(full string) bool {
	entries, err := os.ReadDir(filepath.Dir(full))
	if err != nil {
		return false
	}
	name := filepath.Base(full)
	for _, e := range entries {
		if e.Name() == name {
			return false
		}
	}
	return true
}

// expandDeclared returns the concrete paths a declared entry denotes now. An
// entry without '*' is returned as-is whether or not it exists. Wildcard
// entries yield the existing matches, sorted lexically; only '*' is a
// wildcard.
func expandDeclared(root, entry string) []string {
	if !strings.Contains(entry, "*") {
		return []string{entry}
	}
	return ExpandWildcard(os.DirFS(root), entry)
}
