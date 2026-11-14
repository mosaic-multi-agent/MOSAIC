package app

// deployed_owned.go fills the Deploy-owned content record of a probed deployed file from the
// bytes read from disk, so the planner compares against what is on disk now and never against
// anything stored.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// attachOwnedContent computes the Deploy-owned content record of a present agent or skill
// file from its bytes and stores it in state. An agent whose body cannot be parsed gets no
// record and is marked ParseFailed, so it stays a conflict rather than escaping the edit
// check. Other kinds (and the unknown kind "") are left untouched.
func attachOwnedContent(state *domain.DeployedArtifactState, kind domain.ArtifactKind, data []byte) {
	if kind != domain.ArtifactAgent && kind != domain.ArtifactSkill {
		return
	}
	owned, err := transform.OwnedContentOf(kind, data)
	if err != nil {
		state.ParseFailed = true
		state.ParseProblem = "Deploy-owned content cannot be determined: " + err.Error()
		return
	}
	state.Owned = &owned
}
