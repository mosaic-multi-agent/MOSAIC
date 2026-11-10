package domain

// Removal is one workspace file a run deletes, after backing it up, optionally with its
// manifest entry.
type Removal struct {
	Ref       ArtifactRef // agent key, or the hook bundle a hook file belongs to
	Path      string      // file to delete, relative to the workspace root
	DropEntry bool        // true: the manifest entry (Ref, Path) is dropped with the file
	Reason    string      // one line, rendered by both frontends
}

// Replacement is one existing workspace file a run overwrites after backing it up. A
// registration target listed here is overwritten even though it exists.
type Replacement struct {
	Ref    ArtifactRef // the hook bundle (or agent) the file belongs to
	Path   string      // relative to the workspace root
	Reason string
}

// RemovalTaken records what the executor did with one Removal.
type RemovalTaken string

const (
	RemovalRemoved RemovalTaken = "removed"      // deleted after backup (dry run: would be deleted)
	RemovalAbsent  RemovalTaken = "absent"       // no file at Path; entry dropped when requested
	RemovalKept    RemovalTaken = "kept-written" // this run wrote Path; not deleted
	RemovalSkipped RemovalTaken = "skipped"      // fallback tier used; workspace not touched
	RemovalFailed  RemovalTaken = "failed"       // backup or delete failed; Err says why
)

// RemovalRecord is what happened to one Removal.
type RemovalRecord struct {
	Removal    Removal
	Taken      RemovalTaken
	BackupPath string // absolute; non-empty only when the file was backed up
	Err        string // non-empty only for RemovalFailed
}

// ReplacementRecord is what happened to one Replacement.
type ReplacementRecord struct {
	Replacement Replacement
	BackupPath  string // absolute; empty on dry run, when no file existed, or when the run did not write Path
	Err         string // backup or write failure
}
