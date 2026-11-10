package repair

import (
	"strconv"
	"strings"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
)

// stepID is a parsed "{name}#{seq}" agent instance id.
type stepID struct {
	name string
	seq  int
}

// parseStepID recognises a step id: exactly one '#', a non-empty name without
// whitespace and a decimal suffix >= 1. Consultation ids carry two '#'.
func parseStepID(id string) (stepID, bool) {
	name, suffix, found := strings.Cut(id, "#")
	if !found || strings.Contains(suffix, "#") || name == "" || strings.ContainsAny(name, " \t\r\n") {
		return stepID{}, false
	}
	seq, err := strconv.Atoi(suffix)
	if err != nil || seq < 1 || strconv.Itoa(seq) != suffix {
		return stepID{}, false
	}
	return stepID{name: name, seq: seq}, true
}

// stepResponse is a logged response of a step.
type stepResponse struct {
	id     stepID
	status domain.StatusCode
	code   domain.ErrorCode
	raw    string
}

// checkDispatchLog judges the repaired state against the step responses of the
// dispatch log. It is lenient by design: it never filters out non-advancing
// responses, it only accepts whatever any logged attempt of the step admits.
// Skipped for an empty path or a log that cannot be read.
func checkDispatchLog(path string, state domain.ArtifactState) []domain.ArtifactDamageProblem {
	if path == "" {
		return nil
	}
	entries, err := dispatchlog.ReadEntries(path)
	if err != nil {
		return nil
	}
	var responses []stepResponse
	highestStep, highestResponse := 0, 0
	for _, e := range entries {
		id, ok := parseStepID(e.AgentInstanceID)
		if !ok {
			continue
		}
		highestStep = max(highestStep, id.seq)
		if e.Type == "response" {
			responses = append(responses, stepResponse{id: id, status: e.StatusCode, code: e.ErrorCode, raw: e.AgentInstanceID})
			highestResponse = max(highestResponse, id.seq)
		}
	}

	if len(responses) == 0 {
		return nil // the log carries no step outcome to judge against
	}
	var problems []domain.ArtifactDamageProblem
	if lastLoggedSeq(state) < highestResponse {
		problems = append(problems, domain.ArtifactDamageProblem{
			Section: sectionExecutionLog,
			Problem: "the last Seq " + strconv.Itoa(lastLoggedSeq(state)) +
				" is below Seq " + strconv.Itoa(highestResponse) + " of a response in the dispatch log",
		})
	}
	return append(problems, checkCurrentState(state.CurrentState, responses, highestStep)...)
}

// lastLoggedSeq is the highest Seq of the repaired Execution Log, 0 when empty.
func lastLoggedSeq(state domain.ArtifactState) int {
	last := 0
	for _, row := range state.ExecutionLog {
		last = max(last, row.Seq)
	}
	return last
}

// checkCurrentState checks last_agent, last_status and error_code against the
// step responses, as far as the log reaches last_agent.
func checkCurrentState(cur domain.CurrentState, responses []stepResponse, highestStep int) []domain.ArtifactDamageProblem {
	if cur.LastAgent == "" {
		return nil
	}
	last, ok := parseStepID(cur.LastAgent)
	if !ok || last.seq > highestStep {
		return nil
	}
	hasOwn := false
	pairLogged := false
	for _, r := range responses {
		if r.raw == cur.LastAgent {
			hasOwn = true
		}
		if r.id.name == last.name && r.id.seq <= last.seq && r.status == cur.LastStatus && r.code == cur.ErrorCode {
			pairLogged = true
		}
	}
	if !hasOwn {
		return []domain.ArtifactDamageProblem{{
			Section: "frontmatter",
			Problem: "last_agent " + cur.LastAgent + " has no response in the dispatch log",
		}}
	}
	if !pairLogged {
		return []domain.ArtifactDamageProblem{{
			Section: "frontmatter",
			Problem: "last_status " + string(cur.LastStatus) + " and error_code " + string(cur.ErrorCode) +
				" match no logged response of " + last.name,
		}}
	}
	return nil
}
