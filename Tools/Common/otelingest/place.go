package otelingest

import (
	"errors"

	"mosaic-common/logappend"
	"mosaic-common/otelingest/diag"
)

// placeCalls places every call it can, in file order, and keeps the rest pending. A call whose record_id was
// already placed is not written again. A final ingest sends pending calls of its scope to the unattributed stream.
func placeCalls(req Request, st *ingestState, r *resolver, w *writer, calls []*call, res *Result) error {
	timedOut := 0
	pending := []*call{}
	for _, c := range calls {
		if _, done := st.Placements[c.RecordID]; done {
			continue
		}
		if !r.keys.tied(c.ConversationID, c.ChatSessionID, c.ParentChatSessionID) {
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
	res.Diagnostics = append(res.Diagnostics, placementDiagnostics(res.Finalized)...)
	if timedOut > 0 {
		res.Diagnostics = append(res.Diagnostics, diag.Diagnostic{
			Kind:    diag.KindAppendTimedOut,
			Message: "lines were not written because the append lock wait limit was reached; a later ingest writes them",
			Counts:  map[string]int{"lines": timedOut},
		})
	}
	return nil
}

// placementDiagnostics reports the pending calls a final ingest placed on the unattributed stream.
func placementDiagnostics(finalized int) []diag.Diagnostic {
	var out []diag.Diagnostic
	if finalized > 0 {
		out = append(out, diag.Diagnostic{
			Kind:    diag.KindPendingFinalized,
			Message: "pending spans were placed on the unattributed stream",
			Counts:  map[string]int{"records": finalized},
		})
	}
	return out
}
