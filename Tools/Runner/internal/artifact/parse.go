package artifact

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"

	"mosaic-run/internal/domain"
)

// Parse parses the canonical orchestration artifact format from the given bytes
// and returns the in-memory ArtifactState.
//
// Returns *domain.RefusalError if the content is not in the canonical format
// (missing type field, missing required sections, or a truncated file).
//
// Empty-value convention: cells that contain "-" in the execution log Iteration or
// Checkpoint columns are returned as "" in the ArtifactState. No other component
// ever sees or produces "-" for these fields.
func Parse(data []byte) (domain.ArtifactState, error) {
	state, err := parseDocument(data)
	var re *domain.RefusalError
	if errors.As(err, &re) && re.Section == "" {
		re.Section = "frontmatter"
	}
	return state, err
}

// parseDocument does the work of Parse; refusals raised before the body
// sections are read carry no section yet.
func parseDocument(data []byte) (domain.ArtifactState, error) {
	refuse := func(reason string) (domain.ArtifactState, error) {
		return domain.ArtifactState{}, &domain.RefusalError{
			Component: "artifact",
			Resource:  "",
			Reason:    reason,
		}
	}

	// Split into frontmatter and body.
	fmContent, bodyContent, hasFM := splitDocument(data)
	if !hasFM {
		return refuse("missing frontmatter (no --- delimiters found)")
	}

	// Parse frontmatter key-value pairs.
	entries := parseFrontmatterEntries(fmContent)
	topLevel, currentState, err := parseFrontmatter(fmContent)
	if err != nil {
		return refuse("failed to parse frontmatter: " + err.Error())
	}

	// Check required type field first.
	typeVal, ok := topLevel["type"]
	if !ok || typeVal != "orchestration-artifact" {
		return refuse("missing or incorrect 'type: orchestration-artifact' field")
	}

	// Locate the tagged sections; the required ones must be present and closed.
	bodyFirstLine := strings.Count(fmContent, "\n") + 3
	lines := splitBodyLines(bodyContent, bodyFirstLine)
	execSec := scanSection(lines, "ExecutionLog")
	artsSec := scanSection(lines, "Artifacts")
	if err := requireSections(execSec, artsSec); err != nil {
		return domain.ArtifactState{}, err
	}

	// Build ArtifactState from frontmatter.
	state, err := buildStateFromFrontmatter(fmContent, entries, topLevel, currentState)
	if err != nil {
		return domain.ArtifactState{}, err
	}

	logEntries, err := parseExecutionLog(execSec.content)
	if err != nil {
		return domain.ArtifactState{}, execSec.refuse("failed to parse execution log: ", err)
	}
	state.ExecutionLog = logEntries

	// A lagging global_sequence (an interrupted write) is corrected to the
	// highest logged Seq; a higher stored value (an interrupted allocation) is kept.
	for _, e := range logEntries {
		if e.Seq > state.GlobalSequence {
			state.GlobalSequence = e.Seq
		}
	}

	regEntries, err := parseArtifactRegistry(artsSec.content)
	if err != nil {
		return domain.ArtifactState{}, artsSec.refuse("failed to parse artifact registry: ", err)
	}
	state.ArtifactRegistry = regEntries

	// WorkflowNotes section is optional; an opening tag makes it required to be readable.
	notesSec := scanSection(lines, "WorkflowNotes")
	if notesSec.openFound {
		if !notesSec.closed {
			return domain.ArtifactState{}, &domain.RefusalError{
				Component: "artifact",
				Reason:    "file appears truncated: missing </WorkflowNotes> closing tag",
				Section:   "WorkflowNotes",
			}
		}
		notes, err := parseWorkflowNotes(notesSec.content)
		if err != nil {
			return domain.ArtifactState{}, notesSec.refuse("failed to parse workflow notes: ", err)
		}
		state.WorkflowNotes = notes
	}

	return state, nil
}

// buildStateFromFrontmatter constructs an ArtifactState from the parsed
// frontmatter maps. All field parsing and validation is done here so that
// Parse itself stays short.
func buildStateFromFrontmatter(fmContent string, entries []domain.FrontmatterEntry, topLevel, currentState map[string]string) (domain.ArtifactState, error) {
	refuse := func(reason string) (domain.ArtifactState, error) {
		return domain.ArtifactState{}, &domain.RefusalError{
			Component: "artifact",
			Reason:    reason,
		}
	}

	state := domain.ArtifactState{
		Type: "orchestration-artifact",
	}

	runID, err := parseRunID(topLevel)
	if err != nil {
		return domain.ArtifactState{}, err
	}
	state.RunID = runID
	if v, ok := topLevel["workflow"]; ok {
		state.Workflow = domain.WorkflowID(v)
	}
	if v, ok := topLevel["workflow_version"]; ok {
		state.WorkflowVersion = domain.WorkflowVersion(v)
	}
	if v, ok := topLevel["task"]; ok {
		state.Task = v
	}
	if v, ok := topLevel["started"]; ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return refuse("invalid 'started' timestamp: " + err.Error())
		}
		state.Started = t
	}
	if v, ok := topLevel["last_updated"]; ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return refuse("invalid 'last_updated' timestamp: " + err.Error())
		}
		state.LastUpdated = t
	}
	if v, ok := topLevel["global_sequence"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return refuse("invalid 'global_sequence': " + err.Error())
		}
		state.GlobalSequence = n
	}
	if v, ok := topLevel["dispatch"]; ok {
		state.Dispatch = domain.DispatchField{Present: true, Value: v}
	}
	if v, ok := topLevel["checkpoints"]; ok {
		switch v {
		case "enabled":
			state.Checkpoints = true
		case "disabled":
			state.Checkpoints = false
		default:
			return refuse("invalid 'checkpoints' value " + `"` + v + `"` + "; valid values: enabled, disabled")
		}
	}
	if v, ok := topLevel["commits"]; ok {
		switch v {
		case "enabled":
			state.Commits = true
		case "disabled":
			state.Commits = false
		default:
			return refuse("invalid 'commits' value " + `"` + v + `"` + "; valid values: enabled, disabled")
		}
	}
	if v, ok := topLevel["commit_branch"]; ok {
		state.CommitBranch = v
	}
	// The variant is never stored: it is derived from commit_branch. A legacy
	// commit_branch_variant key is consumed and ignored.
	state.CommitBranchVariant = domain.DeriveCommitBranchVariant(state.CommitBranch, state.RunID)

	if err := parseRunnerSettings(topLevel, &state.RunSettings); err != nil {
		return domain.ArtifactState{}, err
	}

	if v, ok := topLevel["review_loop_limit"]; ok {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n <= 0 {
			return refuse("invalid 'review_loop_limit' value " + `"` + v + `"` + "; must be a positive integer")
		}
		state.ReviewLoopLimit = n
	}

	selections, err := parseInfrastructureSelections(entries)
	if err != nil {
		return domain.ArtifactState{}, err
	}
	state.InfraClassSelections = selections

	state.UnknownFrontmatter = unknownEntries(entries)

	// Parse infrastructure_overrides block (optional; nil when absent).
	overrides := parseInfrastructureOverrides(fmContent)
	if len(overrides) > 0 {
		state.InfrastructureOverrides = overrides
	}

	// Parse current_state nested block.
	cs := domain.CurrentState{}
	if v, ok := currentState["phase"]; ok {
		cs.Phase = v
	}
	// The pre-3.0 key "stage" marks the pre-3.0 layout and is not read.
	if _, ok := currentState["stage"]; ok {
		return domain.ArtifactState{}, refuseSection("current_state has the pre-3.0 key \"stage\": ", ErrPre30Layout)
	}
	if v, ok := currentState["iteration"]; ok {
		cs.Stage = v
	}
	if v, ok := currentState["last_status"]; ok {
		cs.LastStatus = domain.StatusCode(v)
	}
	if v, ok := currentState["last_agent"]; ok {
		cs.LastAgent = v
	}
	if v, ok := currentState["error_code"]; ok {
		cs.ErrorCode = domain.ErrorCode(v)
	}
	state.CurrentState = cs

	return state, nil
}

// splitDocument splits the raw document bytes into frontmatter content and body content.
// Returns hasFM=false when no frontmatter delimiters are found.
// CRLF line endings are normalised to LF before splitting so that the function
// behaves correctly on Windows where git may check out files with CRLF endings.
func splitDocument(data []byte) (fmContent string, bodyContent string, hasFM bool) {
	// Normalise CRLF to LF so the parser works on Windows checkouts.
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))

	if !bytes.HasPrefix(data, []byte("---\n")) {
		return "", string(data), false
	}

	rest := data[4:] // skip "---\n"
	closingMarker := []byte("\n---\n")
	idx := bytes.Index(rest, closingMarker)
	if idx < 0 {
		return "", string(data), false
	}

	// fmContent is between the two "---\n" delimiters.
	fmContent = string(rest[:idx+1])   // include the \n before the closing ---
	bodyContent = string(rest[idx+5:]) // skip \n---\n
	return fmContent, bodyContent, true
}
