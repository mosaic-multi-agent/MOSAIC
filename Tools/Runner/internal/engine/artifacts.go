package engine

import (
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// ScheduleContext carries what artifact-path resolution needs besides the row.
type ScheduleContext struct {
	// Schedules is the current schedule per block.
	Schedules domain.BlockSchedules
}

// ResolveArtifactsForIteration expands the block fill-in of artifact paths.
// block and iteration are "" for a row outside every block.
//
// The fill-in "{<block>}", for example "{Stage}" for block Stage, is replaced
// with iteration verbatim in Input and Output entries when block != "". Such an
// entry with iteration == "" returns an error. No other braced text is
// interpreted.
//
// Wildcards: an entry containing '*' is passed through unchanged, and the
// session expands Input wildcards against the run folder. Input and Output
// entries are resolved alike, so the trailing flag does not change the result.
func ResolveArtifactsForIteration(
	arts []string,
	block domain.BlockName,
	iteration domain.IterationID,
	_ ScheduleContext,
	_ bool,
) ([]string, error) {
	var result []string
	for _, art := range arts {
		if block != "" {
			fillIn := "{" + string(block) + "}"
			if strings.Contains(art, fillIn) {
				if iteration == "" {
					return nil, fmt.Errorf(
						"unresolvable %s in artifact path %q: no iteration of block %s at this row", fillIn, art, block)
				}
				art = strings.ReplaceAll(art, fillIn, string(iteration))
			}
		}
		result = append(result, art)
	}
	return result, nil
}

// injectReviewArtifacts appends entries from lastOutputArtifacts to tableArts,
// skipping any path already present once run-prefixed and unprefixed forms are
// treated as equal. The result is the table row's entries in their original
// order, followed by the unique new entries in the order they appear in
// lastOutputArtifacts.
func injectReviewArtifacts(runID string, tableArts, lastOutputArtifacts []string) []string {
	if len(lastOutputArtifacts) == 0 {
		return tableArts
	}
	merged := make([]string, 0, len(tableArts)+len(lastOutputArtifacts))
	merged = append(merged, tableArts...)
	merged = append(merged, lastOutputArtifacts...)
	return domain.DedupArtifactPaths(runID, merged)
}
