package otelingest

import (
	"errors"

	"mosaic-common/logappend"
	"mosaic-common/otelingest/diag"
)

// placeCalls places every call it can, in file order, and keeps the rest pending. A call whose record_id was
// already placed is not written again. A final ingest sends pending calls of its scope to the unattributed stream.
func placeCalls(req Request, st *ingestState, r *resolver, w *writer, calls []*call, res *Result) error {
	untiedCalls := 0
	timedOut := 0
	pending := []*call{}
	for _, c := range calls {
		if _, done := st.Placements[c.RecordID]; done {
			continue
		}
		if !r.keys.tied(c.ConversationID, c.ChatSessionID, c.ParentChatSessionID) {
			if c.Dialect == dialectCLI && c.ConversationID != "" {
				untiedCalls++
			}
			continue
		}
		dest, ok := r.place(c)
		finalized := false
		if !ok && req.Final && inScope(req.Scope, c) {
			dest, ok, finalized = destUnattributed, true, true
		}
		if !ok {
			pending = append(pending, c)
			continue
		}
		if err := w.write(c, dest); err != nil {
			if !errors.Is(err, logappend.ErrLockTimeout) {
				return err
			}
			timedOut++
			pending = append(pending, c)
			continue
		}
		st.Placements[c.RecordID] = dest
		res.RecordsWritten++
		if finalized {
			res.Finalized++
		}
	}
	st.Pending = pending
	res.Pending = len(pending)
	res.Diagnostics = append(res.Diagnostics, placementDiagnostics(r.keys, untiedCalls, res.Finalized)...)
	if timedOut > 0 {
		res.Diagnostics = append(res.Diagnostics, diag.Diagnostic{
			Kind:    diag.KindAppendTimedOut,
			Message: "lines were not written because the append lock wait limit was reached; a later ingest writes them",
			Counts:  map[string]int{"lines": timedOut},
		})
	}
	return nil
}

// placementDiagnostics reports usage the ingest could not tie to the run and pending calls it finalised.
func placementDiagnostics(keys *runKeys, untiedCalls, finalized int) []diag.Diagnostic {
	var out []diag.Diagnostic
	if untiedCalls > 0 {
		out = append(out, diag.Diagnostic{
			Kind:    diag.KindConversationNotTied,
			Message: "usage of a GHCP CLI conversation that no logged session id ties to the run was not ingested",
			Counts:  map[string]int{"usage_spans": untiedCalls},
			IDs:     keys.sessionIDs(),
		})
	}
	if finalized > 0 {
		out = append(out, diag.Diagnostic{
			Kind:    diag.KindPendingFinalized,
			Message: "pending spans were placed on the unattributed stream",
			Counts:  map[string]int{"records": finalized},
		})
	}
	return out
}
