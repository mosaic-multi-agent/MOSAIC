package repair_test

// An Occurrence judges the artifact after a repair consultation and builds the
// error that ends a run whose damage was not repaired.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestOccurrence_Judge_RepairedArtifactIsAcceptedWithTheRepairedState(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")
	occ.NextReport()
	r.writeArtifact(t, r.repaired)

	verdict := occ.Judge()

	requireAccepted(t, verdict)
	if len(verdict.State.ExecutionLog) != 83 {
		t.Errorf("want the repaired state with 83 rows, got %d", len(verdict.State.ExecutionLog))
	}
}

func TestOccurrence_Judge_StillDamagedArtifactIsRejected(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")
	occ.NextReport()

	verdict := occ.Judge()

	requireRejectedAt(t, verdict, "ExecutionLog", -1)
}

func TestOccurrence_Judge_ChangedLastGoodRowIsRejected(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")
	occ.NextReport()
	r.writeArtifact(t, withRowCell(t, r.repaired, 40, 7, "rewritten by the orchestrator"))

	verdict := occ.Judge()

	requireRejectedAt(t, verdict, "ExecutionLog", -1)
}

func TestOccurrence_Unrepaired_AfterTwoRejectedConsultations_NamesProblemsAndEveryPath(t *testing.T) {
	r := newOccurrenceRun(t)
	const damagedCopy = "C:\\ws\\RunnerLogs\\run\\Orchestration.damaged-20261109T200000Z.md"
	occ := r.occurrence(r.lastGood, damagedCopy, "")
	occ.NextReport()
	occ.Judge()
	occ.NextReport()
	latest := occ.Judge()

	err := occ.Unrepaired(domain.UnrepairedRejected, nil)

	if err.Reason != domain.UnrepairedRejected {
		t.Errorf("want reason %q, got %q", domain.UnrepairedRejected, err.Reason)
	}
	if err.Consultations != 2 {
		t.Errorf("want 2 consultations, got %d", err.Consultations)
	}
	if err.Artifact != r.artifactPath || err.DamagedCopy != damagedCopy || err.LastGoodCopy != r.lastGood {
		t.Errorf("want the artifact, damaged copy and last-good paths, got %q %q %q", err.Artifact, err.DamagedCopy, err.LastGoodCopy)
	}
	if want := problemTexts(latest.Problems); !slices.Equal(problemTexts(err.Problems), want) {
		t.Errorf("want the latest verdict's problems %v, got %v", want, problemTexts(err.Problems))
	}
	for _, path := range []string{r.artifactPath, damagedCopy, r.lastGood} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("want the message to name %q, got %q", path, err.Error())
		}
	}
}

func TestOccurrence_Unrepaired_BeforeAnyConsultation_CarriesTheOpeningDamage(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence("", "", "")

	err := occ.Unrepaired(domain.UnrepairedNoConsultant, nil)

	if err.Consultations != 0 {
		t.Errorf("want 0 consultations, got %d", err.Consultations)
	}
	if want := problemTexts(r.damage.Problems); !slices.Equal(problemTexts(err.Problems), want) {
		t.Errorf("want the opening damage's problems %v, got %v", want, problemTexts(err.Problems))
	}
	if err.LastGoodCopy != "" || err.DamagedCopy != "" {
		t.Errorf("want no copy paths, got %q %q", err.LastGoodCopy, err.DamagedCopy)
	}
}

func TestOccurrence_Unrepaired_KeepsTheConsultationErrorAsCause(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")
	occ.NextReport()
	cause := errors.New("orchestrator timed out")

	err := occ.Unrepaired(domain.UnrepairedConsultationFailed, cause)

	if !errors.Is(err, cause) {
		t.Errorf("want the consultation error reachable through errors.Is, got %v", err)
	}
	if err.Consultations != 1 {
		t.Errorf("want the one consultation made counted, got %d", err.Consultations)
	}
}
