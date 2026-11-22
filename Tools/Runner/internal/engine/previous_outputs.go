package engine

// previousOutputs returns the output artifacts of the step the engine routes
// from. A non-nil in.LastOutputsOverride is used verbatim. Otherwise the result
// is the declared Output column of the routing row at pos, resolved for the
// recorded iteration without consulting any block schedule; it depends only on
// the recorded position, so live routing and resume derive the same list.
// A row whose Output column cannot be resolved yields no outputs.
func previousOutputs(in NextInput, pos position) []string {
	if in.LastOutputsOverride != nil {
		return *in.LastOutputsOverride
	}
	if pos.rowIdx < 0 || pos.rowIdx >= len(in.Workflow.Table.Rows) {
		return nil
	}
	row := in.Workflow.Table.Rows[pos.rowIdx]
	outputs, err := ResolveArtifactsForIteration(row.OutputArtifacts,
		blockOfRow(in.Workflow, pos.rowIdx), pos.iteration, ScheduleContext{}, false)
	if err != nil {
		return nil
	}
	return outputs
}
