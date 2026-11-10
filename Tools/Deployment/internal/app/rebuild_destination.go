package app

// rebuild_destination.go carries what an existing destination file holds into the rebuilt
// agent: the user-owned keys its target format keeps in the carriage container, and the
// notices its decode produced.

import (
	"fmt"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// carryDestination decodes the existing destination file through the target descriptor and
// copies its carriage container into the generic source, so user-owned keys survive an
// overwrite. It returns the gaps the destination's decode reported. An update without
// destination bytes returns nothing: the encoder reports the missing prior bytes itself.
func carryDestination(in RebuildInput, tgtDesc *domain.HarnessDescriptor, generic []byte) ([]byte, []domain.Gap, error) {
	if in.Op != agentformat.OpUpdate || len(in.DestinationRaw) == 0 {
		return generic, nil, nil
	}
	dest := decodeDeployedBytes(in.DestinationRaw, domain.ArtifactAgent, tgtDesc)
	if dest.DecodeErr != nil {
		return nil, nil, dest.DecodeErr
	}
	var rep transform.Report
	mergeDecodeReport(&rep, describedEntries(dest.Report))

	gaps := make([]domain.Gap, len(rep.Gaps))
	for i, g := range rep.Gaps {
		g.Subject, g.Owner = in.AgentKey, in.AgentKey
		gaps[i] = g
	}

	destDoc, err := ParseGenericSource(dest.Canonical)
	if err != nil {
		return generic, gaps, nil
	}
	genDoc, err := ParseGenericSource(generic)
	if err != nil {
		return nil, nil, err
	}
	carried := false
	for _, key := range agentformat.CarriageKeys() {
		if v, ok := destDoc.Frontmatter().Get(key); ok {
			if err := genDoc.Frontmatter().Set(key, v); err != nil {
				return nil, nil, fmt.Errorf("carry destination key %q: %w", key, err)
			}
			carried = true
		}
	}
	if !carried {
		return generic, gaps, nil
	}
	return genDoc.Bytes(), gaps, nil
}

// describedEntries returns a copy of the report in which every entry without a detail takes
// its key and reason as the detail, so a gap built from it says what is wrong with the file.
func describedEntries(dr agentformat.Report) agentformat.Report {
	out := agentformat.Report{Entries: make([]agentformat.ReportEntry, len(dr.Entries))}
	copy(out.Entries, dr.Entries)
	for i, e := range out.Entries {
		if e.Detail == "" && e.Reason != "" {
			out.Entries[i].Detail = e.Key + ": " + e.Reason
		}
	}
	return out
}
