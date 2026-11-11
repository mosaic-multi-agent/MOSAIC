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
// A document that parses without any problem yields its full state. Otherwise
// no state is returned (nothing is parsed around damage) and the error is:
//   - *domain.RefusalError for the pre-3.0 layout (errors.Is(err, ErrPre30Layout))
//     and for a missing, empty or malformed run_id; these take precedence over
//     damage because no repair can fix them;
//   - *domain.ArtifactDamageError for everything else: broken frontmatter, a
//     missing or unclosed section, table headers or rows that are not as
//     expected, a table that ends before its closing tag, Execution Log Seq and
//     Agent cells that are not valid, a last_agent without a log row. It lists
//     every problem found with its section and file line.
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
	// Split into frontmatter and body.
	fmContent, bodyContent, hasFM := splitDocument(data)
	if !hasFM {
		d := &damage{}
		d.addFrontmatter(1, "missing frontmatter (no --- delimiters found)")
		return domain.ArtifactState{}, d.err()
	}

	d := &damage{}
	entries := parseFrontmatterEntries(fmContent)
	topLevel, currentState, err := parseFrontmatter(fmContent)
	if err != nil {
		d.addFrontmatter(0, "failed to parse frontmatter: "+err.Error())
	}
	checkFrontmatterSyntax(fmContent, d)

	// A bad run identity and the pre-3.0 layout are refusals, returned alone.
	runID, err := parseRunID(topLevel)
	if err != nil {
		return domain.ArtifactState{}, err
	}
	// The pre-3.0 key "stage" marks the pre-3.0 layout and is not read.
	if _, ok := currentState["stage"]; ok {
		return domain.ArtifactState{}, refuseSection("current_state has the pre-3.0 key \"stage\": ", ErrPre30Layout)
	}

	if typeVal, ok := topLevel["type"]; !ok || typeVal != "orchestration-artifact" {
		line := frontmatterLine(fmContent, "type")
		if line == 0 {
			line = 1
		}
		d.addFrontmatter(line, "missing or incorrect 'type: orchestration-artifact' field")
	}

	state := buildStateFromFrontmatter(fmContent, entries, topLevel, currentState, runID, d)
	if d.frontmatterIntact() {
		recorded := state
		d.recorded = &recorded
	}

	// Locate the tagged sections and read each one; the required ones must be
	// present and closed.
	bodyFirstLine := strings.Count(fmContent, "\n") + 3
	lines := splitBodyLines(bodyContent, bodyFirstLine)

	logEntries, logClean, refusal := readExecutionLog(scanSection(lines, "ExecutionLog"), d)
	if refusal != nil {
		return domain.ArtifactState{}, refusal
	}
	state.ExecutionLog = logEntries
	state.ArtifactRegistry = readArtifactRegistry(scanSection(lines, "Artifacts"), d)
	state.WorkflowNotes = readWorkflowNotes(scanSection(lines, "WorkflowNotes"), d)

	if logClean {
		checkLastAgent(fmContent, state, d)
	}
	if d.found() {
		return domain.ArtifactState{}, d.err()
	}

	// Tolerated, not damage: a lagging global_sequence (an interrupted write) is
	// corrected to the highest logged Seq; a higher stored value (an interrupted
	// allocation) is kept.
	for _, e := range logEntries {
		if e.Seq > state.GlobalSequence {
			state.GlobalSequence = e.Seq
		}
	}
	return state, nil
}

// checkLastAgent reports a last_agent that names an instance with no Execution
// Log row. An empty last_agent is tolerated: it is what the artifact records
// before any invocation was accepted, and the Runner always writes a row
// together with the current_state that names it.
func checkLastAgent(fmContent string, state domain.ArtifactState, d *damage) {
	last := state.CurrentState.LastAgent
	if last == "" {
		return
	}
	for _, e := range state.ExecutionLog {
		if e.Agent == last {
			return
		}
	}
	d.addFrontmatter(frontmatterLine(fmContent, "last_agent"),
		"current_state.last_agent "+strconv.Quote(last)+" has no Execution Log row")
}

// buildStateFromFrontmatter constructs an ArtifactState from the parsed
// frontmatter maps, reporting every invalid value as a frontmatter problem and
// carrying on with the remaining keys. All field parsing and validation is done
// here so that Parse itself stays short.
func buildStateFromFrontmatter(fmContent string, entries []domain.FrontmatterEntry, topLevel, currentState map[string]string, runID string, d *damage) domain.ArtifactState {
	bad := func(key, reason string) { d.addFrontmatter(frontmatterLine(fmContent, key), reason) }

	state := domain.ArtifactState{
		Type:  "orchestration-artifact",
		RunID: runID,
	}

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
			bad("started", "invalid 'started' timestamp: "+err.Error())
		}
		state.Started = t
	}
	if v, ok := topLevel["last_updated"]; ok && v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			bad("last_updated", "invalid 'last_updated' timestamp: "+err.Error())
		}
		state.LastUpdated = t
	}
	if v, ok := topLevel["global_sequence"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			bad("global_sequence", "invalid 'global_sequence': "+err.Error())
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
			bad("checkpoints", "invalid 'checkpoints' value "+`"`+v+`"`+"; valid values: enabled, disabled")
		}
	}
	if v, ok := topLevel["commits"]; ok {
		switch v {
		case "enabled":
			state.Commits = true
		case "disabled":
			state.Commits = false
		default:
			bad("commits", "invalid 'commits' value "+`"`+v+`"`+"; valid values: enabled, disabled")
		}
	}
	if v, ok := topLevel["commit_branch"]; ok {
		state.CommitBranch = v
	}
	// The variant is never stored: it is derived from commit_branch. A legacy
	// commit_branch_variant key is consumed and ignored.
	state.CommitBranchVariant = domain.DeriveCommitBranchVariant(state.CommitBranch, state.RunID)

	if err := parseRunnerSettings(topLevel, &state.RunSettings); err != nil {
		d.addRefusal(firstSettingsLine(fmContent), err)
	}

	if v, ok := topLevel["review_loop_limit"]; ok {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n <= 0 {
			bad("review_loop_limit", "invalid 'review_loop_limit' value "+`"`+v+`"`+"; must be a positive integer")
		}
		state.ReviewLoopLimit = n
	}

	selections, err := parseInfrastructureSelections(entries)
	if err != nil {
		d.addRefusal(frontmatterLine(fmContent, "infrastructure_selections"), err)
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

	return state
}

// firstSettingsLine is the file line of the first runner setting key present
// (current or legacy name); 0 when none is.
func firstSettingsLine(fmContent string) int {
	for _, key := range []string{"runner_mode", "mode", "runner_pre_consultation", "pre_consultation", "runner_manual_resolution", "manual_resolution"} {
		if line := frontmatterLine(fmContent, key); line != 0 {
			return line
		}
	}
	return 0
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
