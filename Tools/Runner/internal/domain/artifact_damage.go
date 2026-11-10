package domain

import (
	"errors"
	"strconv"
	"strings"
)

// ErrWriteAfterDamage is matched by every artifact write the store refuses
// because a read of the artifact found it damaged.
var ErrWriteAfterDamage = errors.New("artifact write refused: the artifact is damaged")

// ArtifactDamageProblem is one damage finding in an Orchestration.md.
type ArtifactDamageProblem struct {
	Section string // "frontmatter", "ExecutionLog", "Artifacts", "WorkflowNotes"
	Line    int    // 1-based line in the file; 0 when not tied to a line
	Problem string // what is wrong, e.g. "row has 8 cells, the header has 10"
}

// String renders "{Section}, line {Line}: {Problem}", or "{Section}: {Problem}"
// when Line is 0 (just "{Problem}" when Section is also "").
func (p ArtifactDamageProblem) String() string {
	var b strings.Builder
	if p.Section != "" {
		b.WriteString(p.Section)
		if p.Line > 0 {
			b.WriteString(", line " + strconv.Itoa(p.Line))
		}
		b.WriteString(": ")
	}
	b.WriteString(p.Problem)
	return b.String()
}

// ArtifactDamageError reports a damaged Orchestration.md. It is distinct from
// RefusalError and PositionUnresolvedError.
type ArtifactDamageError struct {
	Resource string                  // artifact file path; "" when parsed from bytes only
	Problems []ArtifactDamageProblem // every problem found, file order; never empty

	// Frontmatter holds the values the frontmatter records, when the frontmatter
	// has no problem of its own. Nil otherwise.
	Frontmatter *ArtifactState
}

// Error says the artifact is damaged, names Resource when set, and lists every
// problem with its section and line.
func (e *ArtifactDamageError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.String()
	}
	msg := "artifact damaged: "
	if e.Resource != "" {
		msg += e.Resource + ": "
	}
	return msg + strings.Join(parts, "; ")
}

// AsRefusal describes the damage as a *RefusalError for callers that still
// report an unreadable artifact as a refusal. The damage error is kept as the
// Cause, so errors.As still finds it; Section and Line name the first problem.
func (e *ArtifactDamageError) AsRefusal() *RefusalError {
	re := &RefusalError{
		Component: "artifact",
		Resource:  e.Resource,
		Reason:    e.Error(),
		Cause:     e,
	}
	if len(e.Problems) > 0 {
		re.Section, re.Line = e.Problems[0].Section, e.Problems[0].Line
	}
	return re
}

// DamagedFrontmatter reports whether err is (or wraps) an *ArtifactDamageError
// whose frontmatter is intact and, if so, the values that frontmatter records.
func DamagedFrontmatter(err error) (ArtifactState, bool) {
	var damage *ArtifactDamageError
	if errors.As(err, &damage) && damage.Frontmatter != nil {
		return *damage.Frontmatter, true
	}
	return ArtifactState{}, false
}

// IsArtifactDamage reports whether err is (or wraps) an *ArtifactDamageError.
func IsArtifactDamage(err error) bool {
	var damage *ArtifactDamageError
	return errors.As(err, &damage)
}
