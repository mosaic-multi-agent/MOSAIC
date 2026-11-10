package engine

import (
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// IterationArtifactCandidates returns the candidate artifact paths of the
// ended iteration of block, in table order, for a commit-class ITERATION_END
// dispatch.
//
// Rows considered: table.Rows[block.StartRow : block.EndRow] (EndRow is
// exclusive), every group included. For each row the Input cell entries come
// before the Output cell entries (RoutingRow.InputArtifacts, then
// OutputArtifacts, verbatim cell entries).
//
// An entry is a candidate only if its raw text contains the fill-in
// "{" + block.Name + "}". The fill-in is replaced with iteration verbatim
// (same rule as ResolveArtifactsForIteration). An entry that also contains '*'
// is returned resolved with its '*' kept (e.g. "*-{Topic}-Findings.md" ->
// "*-Pricing-Findings.md"); it is not expanded. Entries without the fill-in
// (including wildcard entries without it) are never returned.
//
// Identical resolved strings are kept once, at their first position.
// Returns (nil, nil) when no entry qualifies.
//
// Errors: iteration == "" or block.Name == ""; block.StartRow < 0,
// block.EndRow > len(table.Rows) or StartRow > EndRow.
//
// Pure: no file-system access, no wildcard expansion.
func IterationArtifactCandidates(
	table domain.RoutingTable,
	block domain.AdmittedBlock,
	iteration domain.IterationID,
) ([]string, error) {
	if iteration == "" {
		return nil, fmt.Errorf("iteration artifacts: empty iteration ID")
	}
	if block.Name == "" {
		return nil, fmt.Errorf("iteration artifacts: empty block name")
	}
	if block.StartRow < 0 || block.EndRow > len(table.Rows) || block.StartRow > block.EndRow {
		return nil, fmt.Errorf("iteration artifacts: block %s rows [%d, %d) are outside the table of %d rows",
			block.Name, block.StartRow, block.EndRow, len(table.Rows))
	}
	fillIn := "{" + string(block.Name) + "}"
	var out []string
	seen := map[string]bool{}
	add := func(entries []string) {
		for _, entry := range entries {
			if !strings.Contains(entry, fillIn) {
				continue
			}
			resolved := strings.ReplaceAll(entry, fillIn, string(iteration))
			if seen[resolved] {
				continue
			}
			seen[resolved] = true
			out = append(out, resolved)
		}
	}
	for _, row := range table.Rows[block.StartRow:block.EndRow] {
		add(row.InputArtifacts)
		add(row.OutputArtifacts)
	}
	return out, nil
}
