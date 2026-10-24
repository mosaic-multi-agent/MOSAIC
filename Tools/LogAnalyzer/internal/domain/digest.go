package domain

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// Finding kinds raised while loading digest files. They are load-time findings:
// an export never carries them.
const (
	// FindingInvalidDigest reports a digest file that is unreadable, not JSON or
	// structurally invalid. The file contributes nothing. Warning severity; data loss.
	FindingInvalidDigest FindingKind = "invalid_digest"

	// FindingUnsupportedDigestVersion reports a digest whose major version this
	// tool does not support. The file contributes nothing. Warning severity; data loss.
	FindingUnsupportedDigestVersion FindingKind = "unsupported_digest_version"

	// FindingDuplicateRun reports a run (or bucket) seen more than once; the
	// discarded copy is named by the finding. Info severity; not data loss.
	FindingDuplicateRun FindingKind = "duplicate_run"
)

// DigestStore finds, reads and writes run digest files.
// Implementations: digest.Store (production), in-memory fake (tests).
type DigestStore interface {
	// Discover lists the digest files at path, without reading their contents:
	//   - path is a regular file whose name satisfies IsDigestFileName -> [path]
	//   - path is a directory -> its direct children that are regular files with
	//     a digest name (not recursive; dot-prefixed names skipped), as
	//     filepath.Join(path, name), sorted ascending
	//   - anything else (absent, unreadable, non-digest file) -> nil
	Discover(path string) []string

	// Read decodes one digest file. It returns an error ONLY when ctx is
	// cancelled. A same-major digest yields its Digest (Header.Version as read).
	// A different major version yields a zero Digest and exactly one
	// FindingUnsupportedDigestVersion. An unreadable, non-JSON or structurally
	// invalid file yields a zero Digest and exactly one FindingInvalidDigest.
	// Those findings carry Path = path and a short Detail with no file content.
	Read(ctx context.Context, path string) (Digest, []Finding, error)

	// Write encodes d to path at the adapter's own digest version, using
	// d.Header.ToolVersion and d.Header.WrittenAt as given.
	// It creates missing parent directories.
	// When path exists and overwrite is false, it returns an error for which
	// errors.Is(err, ErrDigestExists) holds, and leaves the file untouched.
	// On any error, path holds either its previous content or nothing (no partial file).
	Write(ctx context.Context, path string, d Digest, overwrite bool) error
}

// Digest is the content-free snapshot of one or more runs.
type Digest struct {
	Header    DigestHeader
	Aggregate Aggregate // Runs sorted by run id; Unattributable nil when absent
	Findings  []Finding // carried findings; only Kind, Severity, Run, Actor are meaningful
}

// DigestHeader is the self-describing part of a digest file.
type DigestHeader struct {
	Version     string    // digest_version as read; ignored by Write (the adapter writes its own)
	ToolVersion string    // LogAnalyzer version that wrote the file
	WrittenAt   time.Time // zero when absent or unparseable
}

// ErrDigestExists is returned (wrapped or bare) by DigestStore.Write when the
// target exists and overwrite was not requested.
var ErrDigestExists = errors.New("digest file already exists")

// DigestFileSuffix is the file-name suffix that marks a digest file.
const DigestFileSuffix = ".digest.json"

// IsDigestFileName reports whether the base name of path ends in DigestFileSuffix,
// case-insensitively.
func IsDigestFileName(path string) bool {
	return strings.HasSuffix(strings.ToLower(filepath.Base(path)), DigestFileSuffix)
}
