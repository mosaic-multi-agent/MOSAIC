package app

// deployed_bytes.go holds the single rule by which every flow (update, workflows, deploy,
// agents) obtains the deployed bytes of a plan item for the content step. Handing the content
// step the bytes of the file it is about to rewrite is what lets the transform keep project
// regions, custom regions, tool choices and, when asked, infrastructure declarations; an item
// without deployed bytes is rendered as a create.

import (
	"fmt"
	"path/filepath"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
)

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

// funnelReader returns the callback form buildContent takes: for each plan item it picks the
// deployed bytes by the deployed-bytes rule (captured migration bytes first, then the item's
// read path), decodes them through the funnel for the item's artifact kind (Layer 1/2 in
// deployed_read.go), and derives the item's agentformat.Operation via actionToOp.
//
// This is where the per-item byte choice (this file) meets the format decode (deployed_read.go):
// the transform always receives canonical bytes plus the raw bytes the translator needs to
// preserve user-owned keys, whatever file the bytes were taken from.
func (d deployedBytes) funnelReader(desc *domain.HarnessDescriptor) func(domain.PlanItem) (DeployedRead, agentformat.Operation, error) {
	return func(item domain.PlanItem) (DeployedRead, agentformat.Operation, error) {
		op, err := actionToOp(item.Action)
		if err != nil {
			return DeployedRead{}, agentformat.OpUnspecified,
				fmt.Errorf("deployed reader: %w for item %q", err, item.TargetPath)
		}
		if b, ok := d.captured[item.TargetPath]; ok {
			read := decodeDeployedBytes(b, item.Ref.Kind, desc)
			read.Present = true
			return read, op, nil
		}
		return readDeployedArtifact(filepath.Join(d.workspace, d.readPathFor(item)), item.Ref.Kind, desc), op, nil
	}
}

// canonicalReader returns the deployed bytes of each plan item in canonical form, decoded
// through funnelReader, for callers that only inspect the deployed file (no write). It yields
// nil when the item has no deployed file or its bytes cannot be read or decoded.
func (d deployedBytes) canonicalReader(desc *domain.HarnessDescriptor) func(domain.PlanItem) []byte {
	read := d.funnelReader(desc)
	return func(item domain.PlanItem) []byte {
		dr, _, err := read(item)
		if err != nil || !dr.Present || dr.ReadErr != nil || dr.DecodeErr != nil {
			return nil
		}
		return dr.Canonical
	}
}
