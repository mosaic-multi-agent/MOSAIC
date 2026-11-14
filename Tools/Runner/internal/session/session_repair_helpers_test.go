package session_test

// Fixtures for the mid-run artifact repair tests: a routing consultant that
// edits the run's Orchestration.md while it answers (like an orchestrator that
// damages or repairs the file), an orchestrated linear run on the real artifact
// file store, and the assertions over the outcome and the evidence copies.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// strayText is a line the orchestrator wrongly leaves between the Execution Log
// table and its closing tag: a damage the strict reader reports.
const strayText = "the orchestrator left this line between the table and its closing tag"

// docEdit rewrites the text of Orchestration.md.
type docEdit func(t *testing.T, doc string) string

func damageLog(t *testing.T, doc string) string {
	t.Helper()
	if !strings.Contains(doc, "</ExecutionLog>") {
		t.Fatal("test setup: the artifact has no Execution Log closing tag")
	}
	return strings.Replace(doc, "</ExecutionLog>", strayText+"\n</ExecutionLog>", 1)
}

func fixLog(t *testing.T, doc string) string {
	t.Helper()
	if !strings.Contains(doc, strayText+"\n") {
		t.Fatal("test setup: the artifact does not carry the stray line to fix")
	}
	return strings.Replace(doc, strayText+"\n", "", 1)
}

// replaceOnce swaps old for new, which must be present.
func replaceOnce(t *testing.T, doc, old, repl string) string {
	t.Helper()
	if !strings.Contains(doc, old) {
		t.Fatalf("test setup: %q not found in the artifact", old)
	}
	return strings.Replace(doc, old, repl, 1)
}

// fixLogAndRewriteRow repairs the damage but also changes the summary of the
// first recorded row, which a repair must never do.
func fixLogAndRewriteRow(t *testing.T, doc string) string {
	t.Helper()
	return replaceOnce(t, fixLog(t, doc), "agent-a finished", "agent-a rewritten")
}

// fixLogAndRestoreRow repairs the damage and puts the first row's summary back.
func fixLogAndRestoreRow(t *testing.T, doc string) string {
	t.Helper()
	return replaceOnce(t, doc, "agent-a rewritten", "agent-a finished")
}

var globalSequenceLine = regexp.MustCompile(`(?m)^global_sequence: \d+$`)

// fixLogAndSetSequence repairs the damage and sets global_sequence to n.
func fixLogAndSetSequence(n int) docEdit {
	return func(t *testing.T, doc string) string {
		t.Helper()
		doc = fixLog(t, doc)
		if !globalSequenceLine.MatchString(doc) {
			t.Fatal("test setup: the artifact has no global_sequence line")
		}
		return globalSequenceLine.ReplaceAllString(doc, "global_sequence: "+strconv.Itoa(n))
	}
}

// damageAndDropLastGood damages the Execution Log and deletes the last-good copy.
func damageAndDropLastGood(evidenceDir string) docEdit {
	return func(t *testing.T, doc string) string {
		t.Helper()
		if err := os.Remove(filepath.Join(evidenceDir, "Orchestration.last-good.md")); err != nil {
			t.Fatalf("test setup: %v", err)
		}
		return damageLog(t, doc)
	}
}

// repairStep scripts one routing consultation of the editing consultant.
type repairStep struct {
	edit   docEdit // rewrites the artifact while the consultation runs; nil leaves it
	reply  domain.RoutingInstruction
	err    error
	force  bool   // raise the force-stop request, then wait for the kill
	arm    bool   // confirm a graceful stop while the consultation runs
	during func() // extra action while the consultation runs
}

// editingConsultant answers its steps in order and records, per call, the
// request, the artifact text it left behind and the evidence copies that
// existed when the call began.
type editingConsultant struct {
	t           *testing.T
	path        string
	evidenceDir string
	win         *forceWindow
	stop        *stopFlag
	steps       []repairStep

	mu     sync.Mutex
	reqs   []domain.ConsultationRequest
	left   []string   // artifact text after each call's edit
	copies [][]string // names of the damaged copies on disk when each call began
}

func (c *editingConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.mu.Lock()
	n := len(c.reqs)
	c.reqs = append(c.reqs, req)
	c.copies = append(c.copies, c.damagedCopyNames())
	c.mu.Unlock()
	if n >= len(c.steps) {
		return domain.RoutingInstruction{}, &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "no scripted step"}
	}
	step := c.steps[n]
	if step.edit != nil {
		data, err := os.ReadFile(c.path)
		if err != nil {
			c.t.Fatalf("editing consultant: %v", err)
		}
		if err := os.WriteFile(c.path, []byte(step.edit(c.t, string(data))), 0o600); err != nil {
			c.t.Fatalf("editing consultant: %v", err)
		}
	}
	data, _ := os.ReadFile(c.path)
	c.mu.Lock()
	c.left = append(c.left, string(data))
	c.mu.Unlock()
	if step.during != nil {
		step.during()
	}
	if step.arm {
		c.stop.arm()
	}
	if step.force {
		return domain.RoutingInstruction{}, blockUntilKilled(ctx, c.win, nil, false)
	}
	return step.reply, step.err
}

func (c *editingConsultant) damagedCopyNames() []string {
	matches, _ := filepath.Glob(filepath.Join(c.evidenceDir, "Orchestration.damaged-*.md"))
	for i, m := range matches {
		matches[i] = filepath.Base(m)
	}
	return matches
}

func (c *editingConsultant) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reqs)
}

func (c *editingConsultant) request(i int) domain.ConsultationRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i >= len(c.reqs) {
		c.t.Fatalf("want at least %d routing consultations, got %d", i+1, len(c.reqs))
	}
	return c.reqs[i]
}

func (c *editingConsultant) leftAfter(i int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i >= len(c.left) {
		c.t.Fatalf("want at least %d routing consultations, got %d", i+1, len(c.left))
	}
	return c.left[i]
}

func (c *editingConsultant) copiesAtCall(i int) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.copies[i])
}

// ---- replies ----

func dispatchReply(agent string, row int) domain.RoutingInstruction {
	return domain.RoutingInstruction{Dispatch: &domain.DispatchInstruction{Agent: agent, RowIndex: row, TaskDescription: "task for " + agent}}
}

func stopReply(reason string) domain.RoutingInstruction {
	return domain.RoutingInstruction{Stop: &domain.StopInstruction{Reason: reason}}
}

func transportFailure() error {
	return &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "orchestrator timed out"}
}

// firstStep is the consultation before any damage: it dispatches agent-a.
func firstStep() repairStep { return repairStep{reply: dispatchReply("agent-a", 0)} }

// ---- observer ----

// repairTrace is the consultation trace that also receives repair verdicts.
type repairTrace struct {
	consultTrace
	vmu      sync.Mutex
	outcomes []domain.RepairOutcome
}

const evRepairJudged = "repair-judged"

func (r *repairTrace) RepairJudged(_ context.Context, o domain.RepairOutcome) {
	r.vmu.Lock()
	r.outcomes = append(r.outcomes, o)
	r.vmu.Unlock()
	r.add(evRepairJudged)
}

func (r *repairTrace) judged() []domain.RepairOutcome {
	r.vmu.Lock()
	defer r.vmu.Unlock()
	return slices.Clone(r.outcomes)
}

// pathLogger is a dispatch logger whose Path is whatever the test sets.
type pathLogger struct {
	domain.NopDispatchLogger
	path string
}

func (p pathLogger) Path() string { return p.path }

// ---- the rig ----

// repairRig is an orchestrated linear run on the real artifact file store whose
// routing consultant is an editingConsultant.
type repairRig struct {
	run     seededRun
	f       *harness.MockAdapter
	cons    *editingConsultant
	manual  *scriptedRoutingConsultant
	user    *choiceUser
	trace   *repairTrace
	log     *sessionRecordingLogger
	win     *forceWindow
	stop    *stopFlag
	flag    *requestFlag
	cfg     domain.RunConfig
	deps    session.Deps
	evidDir string
}

// newRepairRig builds the rig; steps are the routing consultations in order.
// tune may adjust the session dependencies and the config before the session
// is built.
func newRepairRig(t *testing.T, steps []repairStep, tune func(*repairRig)) *repairRig {
	t.Helper()
	run := emptyFileStoreRun(t)
	evidDir, ok := artifact.EvidenceDir(run.artifactPath)
	if !ok {
		t.Fatal("test setup: the run's artifact path has no evidence location")
	}
	r := &repairRig{
		run: run, f: harness.NewMockAdapter(), manual: &scriptedRoutingConsultant{}, evidDir: evidDir,
		user: &choiceUser{option: domain.ForceStopNextStop}, trace: &repairTrace{}, log: &sessionRecordingLogger{},
		win: &forceWindow{}, stop: &stopFlag{}, flag: &requestFlag{}, cfg: run.newRunConfig(),
	}
	r.cfg.RunSettings.Mode = domain.ExecutionModeOrchestrated
	r.cons = &editingConsultant{t: t, path: run.artifactPath, evidenceDir: evidDir, win: r.win, stop: r.stop, steps: steps}
	r.f.Queue("agent-a", successEntry("agent-a finished"))
	r.f.Queue("agent-b", successEntry("agent-b finished"))
	r.deps = session.Deps{
		Harness: r.f, Store: artifact.NewFileStore(run.artifactPath), Routing: r.cons, Manual: r.manual,
		Clock: fixedClock{t: epoch}, Interact: r.user, Consultations: r.trace, Debug: r.log,
		ForceStop: r.win, StopRequested: r.stop.requested, TakeManualDispatchRequest: r.flag.take,
	}
	if tune != nil {
		tune(r)
	}
	return r
}

// start runs the session to its end.
func (r *repairRig) start(t *testing.T) domain.RunOutcome {
	t.Helper()
	got, err := session.New(r.deps).Start(context.Background(), r.cfg)
	if err != nil {
		t.Fatalf("want a nil error (outcome carries the ending), got %v", err)
	}
	return got
}

// ---- assertions ----

// unrepairedOf returns the unrepaired-damage cause of the outcome.
func unrepairedOf(t *testing.T, out domain.RunOutcome) *domain.UnrepairedDamageError {
	t.Helper()
	var unrepaired *domain.UnrepairedDamageError
	if !errors.As(out.Cause, &unrepaired) {
		t.Fatalf("want a cause matching *domain.UnrepairedDamageError, got %v (status %q, message %q)", out.Cause, out.Status, out.Message)
	}
	return unrepaired
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// requireDamageReport asserts req is a repair consultation and returns its report.
func requireDamageReport(t *testing.T, req domain.ConsultationRequest) domain.ArtifactDamageReport {
	t.Helper()
	if req.ArtifactDamage == nil {
		t.Fatal("want a repair consultation carrying artifact_damage, got none")
	}
	if len(req.ArtifactDamage.Problems) == 0 {
		t.Error("a damage report must list at least one problem")
	}
	return *req.ArtifactDamage
}

// requireNoDamageReport asserts req carries no artifact_damage.
func requireNoDamageReport(t *testing.T, req domain.ConsultationRequest, which string) {
	t.Helper()
	if req.ArtifactDamage != nil {
		t.Errorf("%s: want artifact_damage null, got %+v", which, *req.ArtifactDamage)
	}
}

// invokedInstances lists the agent instance ids the harness was invoked with.
func invokedInstances(f *harness.MockAdapter) []string {
	var ids []string
	for _, inv := range f.Invocations() {
		ids = append(ids, inv.Request.AgentInstanceID)
	}
	return ids
}

// triggerFields is the part of a request that says why the orchestrator is asked.
type triggerFields struct {
	status, message, reason, user any
	deviation                     *domain.DeviationInfo
}

func triggerOf(req domain.ConsultationRequest) triggerFields {
	return triggerFields{status: req.LastStatus, message: req.LastStatusMessage, reason: req.LastErrorReason, user: req.UserMessage, deviation: req.Deviation}
}
