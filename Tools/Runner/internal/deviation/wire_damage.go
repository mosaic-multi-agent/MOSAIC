package deviation

import "mosaic-run/internal/domain"

// wireRoutingRequest is the routing consultation payload: the common members
// plus artifact_damage, which is always present and null without damage. The
// pre-consultation request never carries it and uses wireRequest alone.
type wireRoutingRequest struct {
	wireRequest
	ArtifactDamage *wireArtifactDamage `json:"artifact_damage"` // no omitempty: null must be emitted
}

// wireArtifactDamage is the artifact_damage object of a repair consultation.
// last_good_copy and dispatch_log are always present and null when the report
// holds no path. The saved damaged copy is never part of it.
type wireArtifactDamage struct {
	Problems     []string `json:"problems"`
	LastGoodCopy *string  `json:"last_good_copy"`
	DispatchLog  *string  `json:"dispatch_log"`
}

// damageWire converts a damage report to its wire form; nil stays nil.
func damageWire(r *domain.ArtifactDamageReport) *wireArtifactDamage {
	if r == nil {
		return nil
	}
	problems := make([]string, len(r.Problems))
	for i, p := range r.Problems {
		problems[i] = p.String()
	}
	return &wireArtifactDamage{
		Problems:     problems,
		LastGoodCopy: optionalPath(r.LastGoodCopy),
		DispatchLog:  optionalPath(r.DispatchLog),
	}
}

// optionalPath returns nil for an empty path, else a pointer to it verbatim.
func optionalPath(p string) *string {
	if p == "" {
		return nil
	}
	return &p
}
