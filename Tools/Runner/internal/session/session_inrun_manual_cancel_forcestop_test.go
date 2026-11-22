package session_test

// The manual dialogue opened after a force stop is not an in-run request: Esc
// there keeps the resumable stop and the dialogue shows no cancel hint.

import (
	"context"
	"strings"
	"testing"
)

func TestSession_ManualDialogueAfterForceStop_Esc_KeepsTheResumableStopWithoutTheHint(t *testing.T) {
	for _, withRouting := range []bool{true, false} {
		name := "manual resolver only"
		if withRouting {
			name = "routing consultant wired"
		}
		t.Run(name, func(t *testing.T) {
			user := newManualUser(t)
			rows := cancelRowRecordingDetail(user)
			ses, cfg := postKillRig(t, withRouting, user)

			got, err := ses.Start(context.Background(), cfg)

			requireAbandonedStop(t, got, err)
			if len(rows.details) != 1 || strings.Contains(rows.details[0], manualCancelHint) {
				t.Errorf("want one dialogue without the Esc hint, got %q", rows.details)
			}
		})
	}
}
