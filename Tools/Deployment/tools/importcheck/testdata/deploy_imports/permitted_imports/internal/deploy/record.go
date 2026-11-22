package deploy

// record.go is a test fixture: the deploy package using only imports it legitimately
// needs. The guard must accept it, so the rule does not degrade into rejecting every
// import. The fixture is never compiled against the real module.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

var _ = domain.ArtifactAgent
var _ = manifest.Hash
var _ = transform.OwnedContentOf
