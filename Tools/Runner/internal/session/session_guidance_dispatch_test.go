package session_test

// Tests of which dispatches carry the standing guidance: every engine-routed
// dispatch (schedule-defect and mechanical re-dispatches included), and none of
// the consultant-routed, commit-setup or infrastructure dispatches.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// requireAllCarry asserts every request of every named agent carries want, and
// that at least one request exists per agent.
func requireAllCarry(t *testing.T, g *guidanceRig, want domain.Guidance, agents ...string) {
	t.Helper()
	for _, agent := range agents {
		reqs := g.requestsFor(agent)
		if len(reqs) == 0 {
			t.Errorf("want %s dispatched, but it was not", agent)
		}
		for i, req := range reqs {
			if !carriesGuidance(req, want) {
				t.Errorf("want dispatch %d of %s to carry the guidance, got task=%q constraints=%q",
					i+1, agent, req.TaskDescription, req.Constraints)
			}
		}
	}
}

func TestSession_GuidanceDispatch_EveryEngineRoutedDispatchCarriesSavedGuidance(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			g := newLinearGuidanceRig(t)
			g.files.preload(g.runFolder, savedGuidanceValue)
			g.queueLinear()
			cfg := g.config("linear", mode)
			cfg.PreConsultation = true

			got, err := g.ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunCompleted)
			requireAllCarry(t, g, savedGuidanceValue, "agent-a", "agent-b")
		})
	}
}

func TestSession_GuidanceDispatch_FieldsGoToTheirOwnRequestField(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueLinear()

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	req := g.firstRequestOf(t, "agent-a")
	if !carriesGuidance(req, savedGuidanceValue) {
		t.Fatalf("want guidance carried, got task=%q constraints=%q", req.TaskDescription, req.Constraints)
	}
	if strings.Contains(req.TaskDescription, savedConstraints) || strings.Contains(req.Constraints, savedTask) {
		t.Errorf("want each field appended to its own request field, got task=%q constraints=%q", req.TaskDescription, req.Constraints)
	}
}

func TestSession_GuidanceDispatch_ScheduleDefectRedispatchCarriesGuidance(t *testing.T) {
	for _, mode := range sdAutoModes {
		t.Run(string(mode), func(t *testing.T) {
			g := newGuidanceRig(t, "audit-plan-block-orch.md", "planner", "auditor", "merger")
			g.files.preload(g.runFolder, savedGuidanceValue)
			g.writeRunFile(t, sdSourceFile, okSchedule("A", "B"))
			g.queueDefectivePlanner(1)
			g.queueProducer("planner", "fixed", runFileWrite(sdSourceFile, okSchedule("A", "B")))
			g.queueOKTimes("auditor", 2)
			g.queueOK("merger")
			cfg := g.config(sdWorkflow, mode)
			cfg.PreConsultation = true

			_, err := g.ses.Start(context.Background(), cfg)

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if n := len(g.requestsFor("planner")); n != 2 {
				t.Fatalf("want the planner dispatched twice (original and fix), got %d", n)
			}
			requireFixRequest(t, g.adapter.Invocations(), 1, sdSourceFile, duplicateIDParts()...)
			requireAllCarry(t, g, savedGuidanceValue, "planner", "auditor", "merger")
		})
	}
}

func TestSession_GuidanceDispatch_MechanicalRedispatchesCarryGuidance(t *testing.T) {
	cases := map[string]func(g *guidanceRig){
		"partially done": func(g *guidanceRig) {
			g.adapter.Queue("agent-a",
				statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half done"),
				statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))
		},
		"tool unavailable retry": func(g *guidanceRig) {
			g.adapter.Queue("agent-a",
				statusEntry(domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "tool down"),
				statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))
		},
	}
	for name, script := range cases {
		for _, mode := range autoModes {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				g := newLinearGuidanceRig(t)
				g.files.preload(g.runFolder, savedGuidanceValue)
				script(g)
				g.queueOK("agent-b")
				cfg := g.config("linear", mode)
				cfg.PreConsultation = true

				got, err := g.ses.Start(context.Background(), cfg)

				requireRunStatus(t, got, err, domain.RunCompleted)
				if n := len(g.requestsFor("agent-a")); n != 2 {
					t.Fatalf("want agent-a dispatched twice, got %d", n)
				}
				requireAllCarry(t, g, savedGuidanceValue, "agent-a", "agent-b")
			})
		}
	}
}

func TestSession_GuidanceDispatch_ConsultantRoutedDispatchCarriesNoGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.adapter.Queue("agent-a",
		statusEntry(domain.StatusBLOCKED, domain.ErrorNone, "needs a decision"),
		statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done on retry"))
	g.queueOK("agent-b")
	g.consult.queueDispatch("agent-a", "consultant-retry-task", 0)

	_, _ = g.ses.Start(context.Background(), g.autoConfig("linear"))

	reqs := g.requestsFor("agent-a")
	if len(reqs) < 2 {
		t.Fatalf("want agent-a dispatched again by the consultant, got %d dispatches", len(reqs))
	}
	if !carriesGuidance(reqs[0], savedGuidanceValue) {
		t.Errorf("want the engine-routed first dispatch to carry guidance, got %q", reqs[0].TaskDescription)
	}
	if reqs[1].TaskDescription != "consultant-retry-task" {
		t.Errorf("want the consultant's task description verbatim, got %q", reqs[1].TaskDescription)
	}
	if !carriesNoGuidance(reqs[1], savedGuidanceValue) {
		t.Errorf("want no guidance on the consultant-routed dispatch, got task=%q constraints=%q", reqs[1].TaskDescription, reqs[1].Constraints)
	}
}

// runProductWithGuidance runs the Product fixture with saved guidance in auto
// mode, with commits enabled when infraAgent is the commit agent and
// checkpoints otherwise.
func runProductWithGuidance(t *testing.T, fixture, infraAgent string) *guidanceRig {
	t.Helper()
	g := newGuidanceRig(t, fixture, "planner", "builder", "checker", infraAgent)
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueOK("planner", runFileWrite("ProductPlan.md", scheduleDoc("Product", "Products", ids("A"))))
	cfg := g.autoConfig("product-block")
	if infraAgent == infraCommitAgent {
		g.queueCommitSetup()
		cfg.Commits = true
		cfg.CommitBranchVariant = domain.CommitBranchMOSAICOwned
	} else {
		cfg.Checkpoints = true
	}
	queueProductIteration(g.blockRig, "A")
	g.queueOKTimes(infraAgent, 1)

	got, err := g.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	return g
}

func requireNoneCarry(t *testing.T, g *guidanceRig, agent string) {
	t.Helper()
	var n int
	for _, inv := range g.adapter.Invocations() {
		if inv.Agent.Identifier != agent {
			continue
		}
		n++
		if !carriesNoGuidance(inv.Request, savedGuidanceValue) {
			t.Errorf("want no guidance on a %s dispatch, got task=%q constraints=%q",
				agent, inv.Request.TaskDescription, inv.Request.Constraints)
		}
	}
	if n == 0 {
		t.Errorf("want %s dispatched, but it was not", agent)
	}
}

func TestSession_GuidanceDispatch_CommitSetupAndCommitTriggerCarryNoGuidance(t *testing.T) {
	g := runProductWithGuidance(t, "infra-contents-commit-interval-orch.md", infraCommitAgent)

	requireNoneCarry(t, g, infraCommitAgent)
	requireAllCarry(t, g, savedGuidanceValue, "planner", "builder", "checker")
}

func TestSession_GuidanceDispatch_CheckpointTriggerCarriesNoGuidance(t *testing.T) {
	g := runProductWithGuidance(t, "infra-contents-checkpoint-orch.md", "checkpoint-manager-git")

	requireNoneCarry(t, g, "checkpoint-manager-git")
	requireAllCarry(t, g, savedGuidanceValue, "planner", "builder", "checker")
}

func TestSession_GuidanceDispatch_NilHolder_StillAppendsSavedGuidance(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.live = nil
	g.rebuild()
	g.files.preload(g.runFolder, savedGuidanceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAllCarry(t, g, savedGuidanceValue, "agent-a", "agent-b")
}

func TestSession_GuidanceDispatch_NilStore_FirstRunStillUsesPreConsultationResult(t *testing.T) {
	g := newLinearGuidanceRig(t)
	g.noStore = true
	g.rebuild()
	g.pc.advice = adviceOf(adviceValue)
	g.queueLinear()

	got, err := g.ses.Start(context.Background(), g.autoConfig("linear"))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAllCarry(t, g, adviceValue, "agent-a", "agent-b")
}
