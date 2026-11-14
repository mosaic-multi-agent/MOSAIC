package app

// deployed_owned_written.go supplies the write-side Deploy-owned content record of an agent.
// The executor stores whatever this returns in the manifest entry, so the record is computed
// the same way the probe computes it for the file on disk: from the decoded form of the
// exact bytes written.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// agentOwnedContentFor returns the executor callback that computes the owned-content record
// of written agent bytes under the destination harness's descriptor. It returns nil when the
// bytes cannot be decoded or the record cannot be determined, so the next run makes no edit
// check rather than failing the write.
func agentOwnedContentFor(desc *domain.HarnessDescriptor) func(domain.PlanItem, []byte) *domain.OwnedContent {
	return func(_ domain.PlanItem, written []byte) *domain.OwnedContent {
		read := decodeDeployedBytes(written, domain.ArtifactAgent, desc)
		if read.DecodeErr != nil {
			return nil
		}
		owned, err := transform.OwnedContentOf(domain.ArtifactAgent, read.Canonical)
		if err != nil {
			return nil
		}
		return &owned
	}
}
