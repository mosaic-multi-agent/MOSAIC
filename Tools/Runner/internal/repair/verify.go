// Package repair holds the decisions about an Orchestration.md the orchestrator
// repaired: whether the repair is accepted and why not. It restores nothing.
package repair

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// VerifyInput is what Verify judges.
type VerifyInput struct {
	Store        domain.ArtifactStore // store of the repaired artifact
	LastGoodCopy string               // "" = no last-good check
	DispatchLog  string               // usable run dispatch log; "" = no log check
}

// Verdict is the judgement on a repaired artifact.
type Verdict struct {
	Accepted bool
	State    domain.ArtifactState           // the strictly read repaired state; set only when Accepted
	Problems []domain.ArtifactDamageProblem // why not accepted; nil when Accepted, never empty otherwise
}

const sectionExecutionLog = "ExecutionLog"

// artifactPather is implemented by stores that know their artifact file; it
// lets a rejection name the line of a changed row.
type artifactPather interface {
	ArtifactPath() string
}

// Verify judges the artifact the orchestrator repaired. It reads the artifact
// once through in.Store.Read and accepts the repair only when the strict read
// passes, every Execution Log row of the last-good copy is kept unchanged and
// the last Seq, last_agent, last_status and error_code are consistent with the
// dispatch log. A last-good copy or dispatch log that cannot be used skips its
// check instead of failing it.
func Verify(in VerifyInput) Verdict {
	state, err := in.Store.Read(context.Background())
	if err != nil {
		return Verdict{Problems: readProblems(err)}
	}
	problems := checkLastGoodRows(in, state)
	problems = append(problems, checkDispatchLog(in.DispatchLog, state)...)
	if len(problems) > 0 {
		return Verdict{Problems: problems}
	}
	return Verdict{Accepted: true, State: state}
}

// readProblems turns a failed strict read into damage problems.
func readProblems(err error) []domain.ArtifactDamageProblem {
	var damage *domain.ArtifactDamageError
	if errors.As(err, &damage) && len(damage.Problems) > 0 {
		return damage.Problems
	}
	return []domain.ArtifactDamageProblem{{Problem: err.Error()}}
}

// checkLastGoodRows reports every Execution Log row of the last-good copy that
// is missing from, or changed in, the repaired state.
func checkLastGoodRows(in VerifyInput, repaired domain.ArtifactState) []domain.ArtifactDamageProblem {
	if in.LastGoodCopy == "" {
		return nil
	}
	data, err := os.ReadFile(in.LastGoodCopy)
	if err != nil {
		return nil
	}
	lastGood, err := artifact.ParseAt(in.LastGoodCopy, data)
	if err != nil {
		return nil
	}
	bySeq := make(map[int]domain.ExecutionLogEntry, len(repaired.ExecutionLog))
	for _, row := range repaired.ExecutionLog {
		bySeq[row.Seq] = row
	}
	lines := repairedRowLines(in.Store)
	var problems []domain.ArtifactDamageProblem
	for _, want := range lastGood.ExecutionLog {
		got, ok := bySeq[want.Seq]
		switch {
		case !ok:
			problems = append(problems, domain.ArtifactDamageProblem{
				Section: sectionExecutionLog,
				Problem: "row " + strconv.Itoa(want.Seq) + " of the last-good copy is missing",
			})
		case !sameRow(want, got):
			problems = append(problems, domain.ArtifactDamageProblem{
				Section: sectionExecutionLog,
				Line:    lines[got.Agent],
				Problem: "row " + strconv.Itoa(want.Seq) + " differs from the last-good copy",
			})
		}
	}
	return problems
}

// sameRow reports whether every parsed column of two rows is equal.
func sameRow(a, b domain.ExecutionLogEntry) bool {
	return a.Seq == b.Seq && a.Agent == b.Agent && a.Phase == b.Phase && a.Stage == b.Stage &&
		a.WorkflowRow == b.WorkflowRow && a.Status == b.Status && a.Timestamp.Equal(b.Timestamp) &&
		a.Summary == b.Summary && a.ErrorCode == b.ErrorCode && a.Inputs == b.Inputs &&
		a.Checkpoint == b.Checkpoint
}

// repairedRowLines maps an Execution Log row's Agent cell to its 1-based line in
// the repaired artifact file. Empty when the store does not expose its file.
func repairedRowLines(store domain.ArtifactStore) map[string]int {
	lines := map[string]int{}
	p, ok := store.(artifactPather)
	if !ok {
		return lines
	}
	data, err := os.ReadFile(p.ArtifactPath())
	if err != nil {
		return lines
	}
	for i, line := range strings.Split(string(data), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 3 || strings.TrimSpace(cells[0]) != "" {
			continue
		}
		seq := strings.TrimSpace(cells[1])
		agent := strings.TrimSpace(cells[2])
		if _, err := strconv.Atoi(seq); err == nil && strings.HasSuffix(agent, "#"+seq) {
			lines[agent] = i + 1
		}
	}
	return lines
}
