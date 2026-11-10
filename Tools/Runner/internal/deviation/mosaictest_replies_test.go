package deviation

// Reply-fixture check for the MosaicTest routing fixtures shipped in the
// Runner's own TestCatalog. The stub orchestrator answers a consultation by
// returning the instruction of one rule of a fixture's MosaicTestRouting.md.
// Every rule must therefore be a reply the Runner accepts: it must pass the
// reply parser, and its row, iteration and group must fit the migrated
// workflow the fixture belongs to (and, where the fixture folder supplies a
// block schedule, name an iteration of that schedule).
//
// One subtest per fixture: a failing subtest names a fixture still written in
// the pre-3.0 reply form or naming a row that the migrated table does not hold.
// The test reads only the Runner's own tree, never Catalog/.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/planstages"
	"mosaic-run/internal/workflow"
)

const mosaicTestWorkflowsRoot = "../../TestCatalog/Workflows/MosaicTest"

// routingRule is one "## Rule:" section of a MosaicTestRouting.md fixture.
type routingRule struct {
	selector    string
	headings    []string
	action      string
	agent       string
	row         string
	iteration   string
	stage       string // the pre-3.0 heading; present only in an old-form fixture
	hasStage    bool
	task        string
	reason      string
	overrides   []string
	constraints *string
}

func TestMosaicTestRoutingFixtures_RepliesAreAcceptedAndFitTheirWorkflow(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(mosaicTestWorkflowsRoot, "Fixtures", "*", "MosaicTestRouting.md"))
	if err != nil {
		t.Fatalf("glob routing fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no MosaicTestRouting.md fixture found")
	}
	sort.Strings(paths)

	for _, path := range paths {
		dir := filepath.Dir(path)
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			admitted := admitMosaicTestWorkflow(t, name)
			schedules := readFixtureSchedules(t, dir, admitted)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read routing fixture: %v", err)
			}
			rules := parseRoutingFixture(string(content))
			for _, rule := range rules {
				checkFixtureRule(t, admitted, schedules, rule)
			}
		})
	}
}

// admitMosaicTestWorkflow parses the migrated workflow strictly and admits it
// in the first mode that accepts it.
func admitMosaicTestWorkflow(t *testing.T, id string) domain.AdmittedWorkflow {
	t.Helper()
	path := filepath.Join(mosaicTestWorkflowsRoot, id+".md")
	region, err := orchfile.GetWorkflow(path, id)
	if err != nil {
		t.Fatalf("read workflow region of %s: %v", id, err)
	}
	table, err := workflow.Parse(region.Content, region.Info)
	if err != nil {
		t.Fatalf("strict parse of workflow %s refused: %v", id, err)
	}
	modes := []domain.ExecutionMode{domain.ExecutionModeAuto, domain.ExecutionModeAutoReview, domain.ExecutionModeOrchestrated}
	var lastErr error
	for _, mode := range modes {
		admitted, err := compat.Admit(table, mode)
		if err == nil {
			return admitted
		}
		lastErr = err
	}
	t.Fatalf("workflow %s is admitted in no execution mode: %v", id, lastErr)
	return domain.AdmittedWorkflow{}
}

// readFixtureSchedules reads the block schedule of every admitted block whose
// declared Source the fixture folder supplies. A block without a supplied
// source has no entry: its schedule is produced during the run.
func readFixtureSchedules(t *testing.T, dir string, admitted domain.AdmittedWorkflow) domain.BlockSchedules {
	t.Helper()
	schedules := domain.BlockSchedules{}
	for _, block := range admitted.Blocks {
		path := filepath.Join(dir, filepath.FromSlash(block.Source))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		schedule, err := planstages.ReadBlockSchedule(planstages.ScheduleRequest{
			Path:       path,
			Source:     block.Source,
			Section:    block.Section,
			Block:      block.Name,
			Grouped:    block.Grouped,
			Approaches: block.Approaches.Approaches(),
		})
		if err != nil {
			t.Errorf("fixture schedule of block %s (%s): %v", block.Name, block.Source, err)
			continue
		}
		schedules[block.Name] = schedule
	}
	return schedules
}

// checkFixtureRule checks one rule as the stub would return it.
func checkFixtureRule(t *testing.T, admitted domain.AdmittedWorkflow, schedules domain.BlockSchedules, rule routingRule) {
	t.Helper()
	label := "rule " + rule.selector
	for _, h := range rule.headings {
		switch h {
		case "Action", "Agent", "Row", "Iteration", "Stage", "TaskDescription", "Reason", "Overrides":
		default:
			t.Errorf("%s: unrecognised heading %q", label, h)
		}
	}
	reply := buildStubReply(rule)
	extracted, err := ExtractJSONObject(reply)
	if err != nil {
		t.Errorf("%s: reply is not extractable JSON: %v", label, err)
		return
	}
	resp, err := unmarshalRoutingResponseLenient(extracted)
	if err != nil {
		t.Errorf("%s: reply parser refused the reply: %v", label, err)
		return
	}
	switch rule.action {
	case "stop":
		if resp.Action != "stop" || resp.Reason != rule.reason {
			t.Errorf("%s: stop reply did not round-trip its reason", label)
		}
	case "dispatch":
		checkDispatchRule(t, admitted, schedules, rule, resp)
	default:
		t.Errorf("%s: action %q is neither dispatch nor stop", label, rule.action)
	}
}

// checkDispatchRule validates a dispatch reply against the migrated table with
// the Runner's own consultant path (row, agent, iteration and group).
func checkDispatchRule(t *testing.T, admitted domain.AdmittedWorkflow, schedules domain.BlockSchedules, rule routingRule, resp wireRoutingResponse) {
	t.Helper()
	label := "rule " + rule.selector
	if rule.row == "" {
		t.Errorf("%s: dispatch rule names no row", label)
		return
	}
	effective := schedules
	if resp.Row >= 1 && resp.Row.Index() < len(admitted.Table.Rows) {
		effective = withSyntheticSchedule(schedules, admitted.Table.Rows[resp.Row.Index()], resp.Iteration)
	}
	consultant := &OrchestratorConsultant{Table: admitted.Table}
	instr, err := consultant.dispatchFromReply(domain.ConsultationRequest{Schedules: effective}, resp)
	if err != nil {
		t.Errorf("%s: reply does not fit the workflow: %v", label, err)
		return
	}
	if want := int(resp.Row) - 1; instr.RowIndex != want {
		t.Errorf("%s: dispatch resolved to row index %d, want %d", label, instr.RowIndex, want)
	}
	_, inBlock := admitted.Table.Rows[instr.RowIndex].Block()
	if inBlock && instr.Iteration == "" {
		t.Errorf("%s: row %d is in a block but the reply resolved to no iteration", label, resp.Row)
	}
	if !inBlock && instr.Iteration != "" {
		t.Errorf("%s: row %d is outside every block but the reply names iteration %q", label, resp.Row, instr.Iteration)
	}
	if instr.TaskDescription != rule.task {
		t.Errorf("%s: task description did not round-trip byte for byte", label)
	}
}

// withSyntheticSchedule gives a block row whose schedule the fixture folder
// does not supply a one-iteration schedule holding the reply's own ID, so the
// Runner's validation still checks that a block row names an iteration and
// the row's group while the ID, produced during the run, is taken as given.
func withSyntheticSchedule(schedules domain.BlockSchedules, row domain.RoutingRow, iteration string) domain.BlockSchedules {
	block, inBlock := row.Block()
	if !inBlock || iteration == "" {
		return schedules
	}
	if _, supplied := schedules[block]; supplied {
		return schedules
	}
	id := iteration
	if group := string(row.PhaseParsed.Group); group != "" {
		id = strings.TrimSuffix(iteration, "."+group)
	}
	out := domain.BlockSchedules{}
	for b, s := range schedules {
		out[b] = s
	}
	out[block] = domain.BlockSchedule{
		Block:      block,
		Iterations: []domain.ScheduleIteration{{ID: domain.IterationID(id)}},
	}
	return out
}

// buildStubReply renders the JSON object the stub orchestrator returns for the
// rule. A rule still carrying the pre-3.0 "### Stage" heading is rendered with
// the "stage" member the old stub returned, so the reply parser refuses it.
func buildStubReply(rule routingRule) []byte {
	reply := map[string]any{"action": rule.action}
	if rule.action == "stop" {
		reply["reason"] = rule.reason
		return mustMarshal(reply)
	}
	reply["agent"] = rule.agent
	reply["task_description"] = rule.task
	if rule.row != "" {
		reply["row"] = numberOrText(rule.row)
	}
	if rule.iteration != "" {
		reply["iteration"] = rule.iteration
	}
	if rule.hasStage {
		reply["stage"] = numberOrText(rule.stage)
	}
	for _, line := range rule.overrides {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "input_artifacts", "output_artifacts":
			reply[strings.TrimSpace(key)] = splitList(value)
		case "hitl":
			reply["hitl_override"] = value == "true"
		}
	}
	if rule.constraints != nil {
		reply["constraints"] = *rule.constraints
	}
	return mustMarshal(reply)
}

func numberOrText(s string) any {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return s
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// parseRoutingFixture reads the rules of a MosaicTestRouting.md fixture:
// "## Rule: {selector}" sections holding "### {Heading}" fields, whose values
// are one line or a tilde-fenced block.
func parseRoutingFixture(content string) []routingRule {
	var rules []routingRule
	idx := -1
	heading := ""
	inFence := false
	var fence []string

	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if inFence {
			if line == "~~~" {
				inFence = false
				setFenceValue(&rules[idx], heading, strings.Join(fence, "\n"))
				fence = nil
				continue
			}
			fence = append(fence, line)
			continue
		}
		switch {
		case strings.HasPrefix(line, "## Rule: "):
			rules = append(rules, routingRule{selector: strings.TrimSpace(strings.TrimPrefix(line, "## Rule: "))})
			idx, heading = len(rules)-1, ""
		case strings.HasPrefix(line, "## "), strings.HasPrefix(line, "# "):
			idx, heading = -1, ""
		case idx >= 0 && strings.HasPrefix(line, "### "):
			heading = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			rules[idx].headings = append(rules[idx].headings, heading)
			if heading == "Stage" {
				rules[idx].hasStage = true
			}
		case idx >= 0 && line == "~~~":
			inFence = true
		case idx >= 0 && heading != "" && strings.TrimSpace(line) != "":
			setLineValue(&rules[idx], heading, strings.TrimSpace(line))
		}
	}
	return rules
}

func setLineValue(r *routingRule, heading, value string) {
	switch heading {
	case "Action":
		r.action = value
	case "Agent":
		r.agent = value
	case "Row":
		r.row = value
	case "Iteration":
		r.iteration = value
	case "Stage":
		r.stage = value
	case "Overrides":
		if value != "none" {
			r.overrides = append(r.overrides, value)
		}
	}
}

func setFenceValue(r *routingRule, heading, value string) {
	switch heading {
	case "TaskDescription":
		r.task = value
	case "Reason":
		r.reason = value
	case "Overrides":
		r.constraints = &value
	}
}
