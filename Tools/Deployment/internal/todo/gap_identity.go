package todo

import "mosaic-deploy/internal/domain"

// gapIdentity returns the key under which a gap is stored at most once per collector lifetime
// (until Reset). The planner and the transform both report a missing model and an unmapped tool
// with different wording, so for those kinds the identity is the agent (and tool) alone; every
// other kind is identified by all of its fields, so gaps that differ in detail or fragment stay.
func gapIdentity(g domain.Gap) string {
	const sep = "\x00"
	switch g.Kind {
	case domain.GapNoModel:
		return string(g.Kind) + sep + g.Subject
	case domain.GapUnmappedTool:
		return string(g.Kind) + sep + g.Owner + sep + g.Subject
	default:
		return string(g.Kind) + sep + g.Subject + sep + g.Owner + sep + g.Detail + sep + g.Fragment
	}
}
