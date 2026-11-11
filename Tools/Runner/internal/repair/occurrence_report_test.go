package repair_test

// An Occurrence hands out the artifact_damage content of its repair
// consultations: at most MaxConsultations, the first from the damage that opened
// it, later ones from the latest rejected verdict, with the evidence paths the
// orchestrator may use.

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/repair"
)

func TestOccurrence_FirstReport_CarriesTheDamageProblemsAndTheLastGoodCopy(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")

	report, ok := occ.NextReport()

	if !ok {
		t.Fatal("want a first report")
	}
	if want := problemTexts(r.damage.Problems); !slices.Equal(problemTexts(report.Problems), want) {
		t.Errorf("want the problems of the damaging read %v, got %v", want, problemTexts(report.Problems))
	}
	if len(report.Problems) == 0 {
		t.Error("a report must never carry an empty problem list")
	}
	if report.LastGoodCopy != r.lastGood {
		t.Errorf("want the last-good copy %q, got %q", r.lastGood, report.LastGoodCopy)
	}
}

func TestOccurrence_WithoutLastGoodCopy_ReportNamesNone(t *testing.T) {
	r := newOccurrenceRun(t)

	report, ok := r.occurrence("", "", "").NextReport()

	if !ok {
		t.Fatal("want a report without a last-good copy")
	}
	if report.LastGoodCopy != "" {
		t.Errorf("want no last-good path, got %q", report.LastGoodCopy)
	}
}

func TestOccurrence_AllowsTwoRepairConsultationsAndCountsThem(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")

	if got := occ.Attempt(); got != 0 {
		t.Fatalf("want 0 attempts before the first report, got %d", got)
	}
	_, first := occ.NextReport()
	afterFirst := occ.Attempt()
	_, second := occ.NextReport()
	afterSecond := occ.Attempt()
	_, third := occ.NextReport()

	if !first || !second {
		t.Fatalf("want two reports (MaxConsultations=%d), got first=%v second=%v", repair.MaxConsultations, first, second)
	}
	if afterFirst != 1 || afterSecond != 2 {
		t.Errorf("want attempts 1 then 2, got %d then %d", afterFirst, afterSecond)
	}
	if third {
		t.Error("want no third report")
	}
	if got := occ.Attempt(); got != 2 {
		t.Errorf("a refused report must not count: want 2 attempts, got %d", got)
	}
}

func TestOccurrence_SecondReport_CarriesTheProblemsOfTheRejectedVerdict(t *testing.T) {
	r := newOccurrenceRun(t)
	occ := r.occurrence(r.lastGood, "", "")
	first, _ := occ.NextReport()
	r.writeArtifact(t, withRowCell(t, r.repaired, 40, 7, "rewritten by the orchestrator"))

	verdict := occ.Judge()
	second, ok := occ.NextReport()

	requireRejectedAt(t, verdict, "ExecutionLog", -1)
	if !ok {
		t.Fatal("want a second report after a rejection")
	}
	if want := problemTexts(verdict.Problems); !slices.Equal(problemTexts(second.Problems), want) {
		t.Errorf("want the verdict's problems %v in the fresh report, got %v", want, problemTexts(second.Problems))
	}
	if slices.Equal(problemTexts(second.Problems), problemTexts(first.Problems)) {
		t.Error("the second report must be fresh, not a copy of the first")
	}
}

func TestOccurrence_DispatchLogInTheReport(t *testing.T) {
	otherRunsLog := writeFile(t, "20261001T000000Z-aaaa-dispatch.log", "{}\n")
	startupLog := writeFile(t, "startup-20261109T195225Z-dispatch.log", "{}\n")
	reportedRunLog := writeFile(t, occurrenceRunID+"-dispatch.log", "{}\n")
	missingRunLog := filepath.Join(t.TempDir(), occurrenceRunID+"-dispatch.log")
	cases := []struct {
		name     string
		reported string // what the logger reports
		onDisk   bool   // the run's log exists at the derived location
		want     string // expected path; "" with onDisk means the derived location
	}{
		{"the logger's own run log is reported", reportedRunLog, false, reportedRunLog},
		{"a startup fallback file is not the run's log", startupLog, false, ""},
		{"another run's log is not the run's log", otherRunsLog, false, ""},
		{"an empty path with no log on disk names none", "", false, ""},
		{"an own-run path whose file is missing names none", missingRunLog, false, ""},
		{"an empty path with the run's log on disk names that log", "", true, ""},
		{"a startup fallback with the run's log on disk names the run's log", startupLog, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := newOccurrenceRun(t)
			want := tc.want
			if tc.onDisk {
				run.writeRunLog(t)
				want = run.runLogPath()
			}

			report, ok := run.occurrence(run.lastGood, "", tc.reported).NextReport()

			if !ok {
				t.Fatal("want a report")
			}
			if report.DispatchLog != want {
				t.Errorf("want dispatch log %q, got %q", want, report.DispatchLog)
			}
		})
	}
}

func TestOccurrence_DispatchLogWithoutUsablePath_IsNotUsedToJudge(t *testing.T) {
	r := newOccurrenceRun(t)
	// A startup file whose content contradicts the repaired artifact: a step
	// response far above every row. Judged against it, the repair would fail.
	contradicting := writeFile(t, "startup-20261109T195225Z-dispatch.log",
		versionLine(t)+"\n"+responseLine(t, "test-writer-tdd#200", domain.StatusSUCCESS, domain.ErrorNone)+"\n")
	r.writeArtifact(t, r.repaired)
	if _, err := os.Stat(contradicting); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	verdict := r.occurrence(r.lastGood, "", contradicting).Judge()

	requireAccepted(t, verdict)
}
