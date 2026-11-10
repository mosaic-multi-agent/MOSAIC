// Package domain defines all value types, enumerations, and port interfaces that
// every other runner package depends on. It imports only standard library and
// mosaic-common packages -- never any internal runner package.
package domain

// WorkflowID is the identifier from the name attribute of a
// <Workflow type="core" name="{id}"> region tag.
type WorkflowID string

// WorkflowVersion is the version string from the version attribute of a
// <Workflow type="core" name="{id}" version="{version}"> region tag.
type WorkflowVersion string

// WorkflowInfo identifies a workflow definition.
type WorkflowInfo struct {
	ID      WorkflowID      // identifier from the region's name attribute
	Version WorkflowVersion // from the region's version attribute
}

// BlockName is a block name from the **Blocks:** table (letters only).
type BlockName string

// BlockDecl is one row of the **Blocks:** table.
type BlockDecl struct {
	Name    BlockName
	Source  string // declared path, verbatim (may fill in enclosing blocks)
	Section string // heading text of the schedule section
}

// PhaseParsed is the structured decomposition of a phase string.
type PhaseParsed struct {
	Name     string      // bare phase name, e.g. "RESEARCH", "PLANNING", "EXECUTION", "REVIEW"
	IsStaged bool        // true when the row is in a block (len(Blocks) > 0)
	Group    GroupName   // the row's group; "" when none declared
	Blocks   []BlockName // block references, outermost first; nil outside every block
}

// RoutingRow is one row of a workflow routing table. The row, not the agent,
// is the unit of execution.
type RoutingRow struct {
	Index           int           // zero-based position in the table; the row's identity
	Phase           string        // literal phase string from the Phase column
	PhaseParsed     PhaseParsed   // structured parse of the phase
	Agent           string        // whole Subagent cell, verbatim (e.g. "build-review(tests)")
	AgentID         string        // bare agent id ("build-review")
	Qualifier       string        // mode qualifier ("tests"); "" when absent
	HITL            bool          // decoded from checkbox emoji
	InputArtifacts  []string      // comma-separated paths, trimmed, verbatim (templates preserved)
	OutputArtifacts []string      // same
	Success         SuccessEntry  // typed On Success cell
	Findings        FindingsEntry // typed On Findings cell
	WaitsFor        []RowRef      // Waits For targets; nil when the column is absent or the cell is "-"
}

// Block returns the block the row is in.
//
//   - When PhaseParsed.Blocks is non-empty it returns (Blocks[0], true): the
//     outermost reference.
//   - Otherwise it returns ("", false): the row is outside every block.
func (r RoutingRow) Block() (BlockName, bool) {
	if len(r.PhaseParsed.Blocks) > 0 {
		return r.PhaseParsed.Blocks[0], true
	}
	return "", false
}

// BareAgentID returns the bare agent id of the row: the id that is dispatched,
// resolved to an agent file and recorded in instance ids.
//
// It returns AgentID, the Subagent cell without its mode qualifier.
func (r RoutingRow) BareAgentID() string {
	return r.AgentID
}

// RoutingTable is the complete routing model for one workflow.
type RoutingTable struct {
	Info          WorkflowInfo
	Rows          []RoutingRow  // declaration order, zero-based indexed
	ApproachTable ApproachTable // zero value when the workflow declares none
	Blocks        []BlockDecl   // **Blocks:** rows in declaration order
}

// WorkflowRegion is one workflow definition found in an orchestrator agent file.
type WorkflowRegion struct {
	Info    WorkflowInfo // identifier and version
	Content []byte       // raw bytes between the boundary tags (for workflow.Parse)
}
