package app

// deployed_bytes.go holds the single rule by which every flow (update, workflows, deploy,
// agents) obtains the deployed bytes of a plan item for the content step. Handing the content
// step the bytes of the file it is about to rewrite is what lets the transform keep project
// regions, custom regions, tool choices and, when asked, infrastructure declarations; an item
// without deployed bytes is rendered as a create.

import "mosaic-deploy/internal/domain"

// deployedBytes decides, per plan item, which workspace file the deployed bytes come from.
type deployedBytes struct {
	workspace string
	// readPaths maps an item's target path to a different file to read instead, for flows whose
	// item is deployed under another path (for example a recorded legacy file).
	readPaths map[string]string
	// captured holds, per target path, bytes read before the run wrote or removed anything.
	captured map[string][]byte
}

// newDeployedBytes returns the rule for a run over workspace. readPaths may be nil.
func newDeployedBytes(workspace string, readPaths map[string]string) deployedBytes {
	return deployedBytes{workspace: workspace, readPaths: readPaths}
}

// readPathFor is the one place where the read path of an item is decided: the item's own target
// path unless the flow named a different file for it.
func (d deployedBytes) readPathFor(item domain.PlanItem) string {
	if p, ok := d.readPaths[item.TargetPath]; ok && p != "" {
		return p
	}
	return item.TargetPath
}

// read returns the deployed bytes for item, or nil when no file is deployed for it.
func (d deployedBytes) read(item domain.PlanItem) []byte {
	if b, ok := d.captured[item.TargetPath]; ok {
		return b
	}
	return readDeployedFile(d.workspace, d.readPathFor(item))
}

// reader returns the callback form buildContent takes.
func (d deployedBytes) reader() func(domain.PlanItem) []byte {
	return d.read
}
