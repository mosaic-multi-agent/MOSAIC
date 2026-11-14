package app_test

// owned_write_basis_helpers_test.go holds the builders shared by the Deploy-owned content
// record tests that run the real flows over the tool-local parity catalog for a Markdown
// harness and for the Codex harness: a spy that wraps the real planner and executor, hand
// edits of a deployed agent in either format, and the check that a manifest record equals
// the record computed from the file on disk (what the probe computes).

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"mosaic-deploy/internal/agentformat"
	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
	"mosaic-deploy/internal/transform"
)

// owHarness describes one harness of the record tests.
type owHarness struct {
	name     string
	id       string
	formatID string // agent format id; empty for Markdown
	model    string
	dir      string   // workspace-relative agents folder
	skip     []string // sample agents the harness does not deploy
	ext      string   // agent file extension
}

// owHarnesses are a Markdown harness and the Codex harness.
var owHarnesses = []owHarness{
	{name: "Markdown", id: hClaude, model: parityModel, dir: ".claude/agents", ext: ".md"},
	{name: "Codex", id: hCodex, formatID: "codex-toml", model: codexAppModel.ModelID, dir: ".codex/agents", ext: ".toml", skip: []string{"orchestrator"}},
}

// owRunner is the sample subagent the hand-edit tests edit.
const owRunner = "parity-runner"

// owAllKeys are the four sample agents a full deploy writes.
var owAllKeys = []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"}

func (h owHarness) rel(key string) string { return h.dir + "/" + key + h.ext }

// keysOf returns the keys the harness deploys among keys.
func (h owHarness) keysOf(keys []string) []string {
	var out []string
	for _, k := range keys {
		skipped := false
		for _, s := range h.skip {
			skipped = skipped || s == k
		}
		if !skipped {
			out = append(out, k)
		}
	}
	return out
}

func (h owHarness) models() map[string]string {
	m := map[string]string{}
	for _, k := range owAllKeys {
		m[k] = h.model
	}
	return m
}

// owExec is one Execute call: the request, the result and the bytes of every agent write the
// plan would make, computed before the executor ran.
type owExec struct {
	req      deploy.ExecRequest
	res      deploy.ExecResult
	contents map[string][]byte
}

// owSpy wraps the real planner and executor of a service and records what passes through.
type owSpy struct {
	inner     plan.Planner
	innerExec deploy.Executor
	inputs    []plan.Input
	plans     []domain.Plan
	execs     []owExec
}

func (s *owSpy) Build(ctx context.Context, in plan.Input) (domain.Plan, error) {
	p, err := s.inner.Build(ctx, in)
	s.inputs = append(s.inputs, in)
	s.plans = append(s.plans, p)
	return p, err
}

func (s *owSpy) Execute(ctx context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	contents := map[string][]byte{}
	for _, item := range req.Plan.Items {
		if item.Ref.Kind != domain.ArtifactAgent || req.Content == nil || !owWrites(item, req) {
			continue
		}
		if b, err := req.Content(item); err == nil {
			contents[item.TargetPath] = b
		}
	}
	res, err := s.innerExec.Execute(ctx, req)
	s.execs = append(s.execs, owExec{req: req, res: res, contents: contents})
	return res, err
}

// owWrites reports whether the executor writes the item under the request's decisions.
func owWrites(item domain.PlanItem, req deploy.ExecRequest) bool {
	switch item.Action {
	case domain.ActionCreate, domain.ActionUpdate:
		return true
	case domain.ActionConflict:
		d := req.Conflicts[item.TargetPath]
		return d == domain.DecisionOverwrite || d == domain.DecisionBackupThenOverwrite
	}
	return false
}

func (s *owSpy) lastPlan(t *testing.T) domain.Plan {
	t.Helper()
	if len(s.plans) == 0 {
		t.Fatal("the planner was never called")
	}
	return s.plans[len(s.plans)-1]
}

func (s *owSpy) lastExec(t *testing.T) owExec {
	t.Helper()
	if len(s.execs) == 0 {
		t.Fatal("the executor was never called")
	}
	return s.execs[len(s.execs)-1]
}

// owSpyDeps is the Deps adjustment that routes the planner and executor through the spy.
func owSpyDeps(s *owSpy) func(*app.Deps) {
	return func(d *app.Deps) {
		s.inner, s.innerExec = d.Planner, d.Executor
		d.Planner, d.Executor = s, s
	}
}

// owService is the parity service with the Codex module registered and the spy in place.
func owService(t *testing.T, s *owSpy, extra ...func(*app.Deps)) app.Service {
	t.Helper()
	return parityServiceWith(t, append([]func(*app.Deps){withCodexTarget(t), owSpyDeps(s)}, extra...)...)
}

// owFlowOpts adjusts one flow run.
type owFlowOpts struct {
	dry      bool
	conflict domain.ConflictDecision
}

func (o owFlowOpts) decision() domain.ConflictDecision {
	if o.conflict != "" {
		return o.conflict
	}
	return domain.DecisionSkip
}

// owFlow is one agent-writing deploy flow and the agents it writes on a workspace holding all
// four sample agents.
type owFlow struct {
	name   string
	writes []string
	run    func(t *testing.T, svc app.Service, h owHarness, ws string, o owFlowOpts) (domain.RunSummary, error)
}

// owFlows returns the four flows that write agents.
func owFlows() []owFlow {
	ctx := context.Background()
	return []owFlow{
		{name: "DeployNew", writes: owAllKeys,
			run: func(t *testing.T, svc app.Service, h owHarness, ws string, o owFlowOpts) (domain.RunSummary, error) {
				return svc.DeployNew(ctx, app.DeployRequest{HarnessID: h.id, WorkspacePath: ws, Scope: domain.ScopeProject,
					WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
					AgentModels: h.models(), AutoConfirmPlan: true, DryRun: o.dry, ConflictDefault: o.decision()})
			}},
		{name: "DeployAgents", writes: []string{owRunner},
			run: func(t *testing.T, svc app.Service, h owHarness, ws string, o owFlowOpts) (domain.RunSummary, error) {
				return svc.DeployAgents(ctx, app.DeployAgentsRequest{HarnessID: h.id, WorkspacePath: ws, Scope: domain.ScopeProject,
					SubagentIDs: []string{owRunner}, UtilityAgentIDs: []string{}, InfrastructureAgentIDs: []string{},
					StandaloneAgentIDs: []string{}, AgentModels: h.models(), AutoConfirmPlan: true, DryRun: o.dry,
					ConflictDefault: o.decision()})
			}},
		{name: "Update", writes: owAllKeys,
			run: func(t *testing.T, svc app.Service, h owHarness, ws string, o owFlowOpts) (domain.RunSummary, error) {
				return svc.Update(ctx, app.UpdateRequest{HarnessID: h.id, WorkspacePath: ws, AgentModels: h.models(),
					AutoConfirmPlan: true, DryRun: o.dry, ConflictDefault: o.decision()})
			}},
		{name: "UpdateWorkflows", writes: []string{"orchestrator", "orchestrator-script"},
			run: func(t *testing.T, svc app.Service, h owHarness, ws string, o owFlowOpts) (domain.RunSummary, error) {
				return svc.UpdateWorkflows(ctx, app.WorkflowUpdateRequest{HarnessID: h.id, WorkspacePath: ws,
					WorkflowIDs: []string{"parity-flow"}, AutoConfirmPlan: true, DryRun: o.dry, ConflictDefault: o.decision()})
			}},
	}
}

// owFlowNamed returns the flow with the given name, so a reordering of owFlows cannot change
// which flow a test drives.
func owFlowNamed(t *testing.T, name string) owFlow {
	t.Helper()
	for _, f := range owFlows() {
		if f.name == name {
			return f
		}
	}
	t.Fatalf("no flow named %q", name)
	return owFlow{}
}

// owCreateFlows are the flows that can create agents in a fresh workspace.
func owCreateFlows(t *testing.T) []owFlow {
	t.Helper()
	return []owFlow{owFlowNamed(t, "DeployNew"), owFlowNamed(t, "DeployAgents")}
}

// owDeploy deploys the sample catalog to a fresh workspace for h.
func owDeploy(t *testing.T, h owHarness) string {
	t.Helper()
	ws := t.TempDir()
	if _, err := owFlowNamed(t, "DeployNew").run(t, owService(t, &owSpy{}), h, ws, owFlowOpts{}); err != nil {
		t.Fatalf("deploy to %s: %v", h.id, err)
	}
	return ws
}

// owUpdate runs Update (with confirmation, conflicts skipped) and returns the spy of the run.
func owUpdate(t *testing.T, h owHarness, ws string, dry bool) (*owSpy, domain.RunSummary) {
	t.Helper()
	spy := &owSpy{}
	sum, err := owFlowNamed(t, "Update").run(t, owService(t, spy), h, ws, owFlowOpts{dry: dry})
	if err != nil {
		t.Fatalf("Update on %s: %v", h.id, err)
	}
	return spy, sum
}

// owProbe returns the deployed state the planner receives for the files of ws, keyed by target
// path, taken from a dry-run Update.
func owProbe(t *testing.T, h owHarness, ws string) map[string]domain.DeployedArtifactState {
	t.Helper()
	spy, _ := owUpdate(t, h, ws, true)
	if len(spy.inputs) == 0 {
		t.Fatal("the planner received no input")
	}
	return spy.inputs[len(spy.inputs)-1].DeployedState
}

// owCanonicalRecord is the record of the canonical (decoded) form of raw agent bytes.
func owCanonicalRecord(t *testing.T, h owHarness, raw []byte) *domain.OwnedContent {
	t.Helper()
	tr, err := agentformat.LookupString(h.formatID)
	if err != nil {
		t.Fatalf("LookupString(%q): %v", h.formatID, err)
	}
	canonical, _, err := tr.Decode(raw, agentformat.ArtifactContext{Kind: domain.ArtifactAgent})
	if err != nil {
		t.Fatalf("decode written %s agent: %v", h.name, err)
	}
	rec, err := transform.OwnedContentOf(domain.ArtifactAgent, canonical)
	if err != nil {
		t.Fatalf("OwnedContentOf(canonical): %v", err)
	}
	return &rec
}

// owAgentEntry returns the manifest entry recorded for the agent key of h in ws.
func owAgentEntry(t *testing.T, h owHarness, ws, key string) domain.ManifestEntry {
	t.Helper()
	for _, e := range gamManifest(t, ws).Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key && filepath.ToSlash(e.TargetPath) == h.rel(key) {
			return e
		}
	}
	t.Fatalf("no manifest entry for agent %s at %s", key, h.rel(key))
	return domain.ManifestEntry{}
}

// owAssertRecordsMatchDisk fails unless, for every key, the manifest record equals both the
// record of the canonical form of the file on disk and the record the probe computes for it.
func owAssertRecordsMatchDisk(t *testing.T, h owHarness, ws string, keys []string) {
	t.Helper()
	owAssertRecords(t, h, ws, keys, true)
}

// owAssertRecords is owAssertRecordsMatchDisk with the probe comparison optional (the probe
// covers the artifacts of the catalog the flow runs over).
func owAssertRecords(t *testing.T, h owHarness, ws string, keys []string, withProbe bool) {
	t.Helper()
	var probed map[string]domain.DeployedArtifactState
	if withProbe {
		probed = owProbe(t, h, ws)
	}
	for _, key := range keys {
		entry := owAgentEntry(t, h, ws, key)
		if entry.Owned == nil {
			t.Errorf("%s: manifest entry has no Owned record", key)
			continue
		}
		want := owCanonicalRecord(t, h, gamRead(t, ws, h.rel(key)))
		if !reflect.DeepEqual(entry.Owned, want) {
			t.Errorf("%s: manifest record = %+v; want the record of the written file's canonical form %+v", key, entry.Owned, want)
		}
		if got := probed[h.rel(key)].Owned; withProbe && !reflect.DeepEqual(entry.Owned, got) {
			t.Errorf("%s: manifest record = %+v; the probe computes %+v from the written file", key, entry.Owned, got)
		}
	}
}

// owAssertChannel fails when an Execute that writes agents carried no AgentOwnedContent.
func owAssertChannel(t *testing.T, s *owSpy) {
	t.Helper()
	wrote := false
	for _, e := range s.execs {
		if len(e.contents) == 0 {
			continue
		}
		wrote = true
		if e.req.AgentOwnedContent == nil {
			t.Error("the executor request writes agents but carries no AgentOwnedContent")
		}
	}
	if !wrote {
		t.Error("no Execute call wrote an agent; the path under test was not exercised")
	}
}

// owPlanItem returns the item of the agent key in the plan.
func owPlanItem(t *testing.T, p domain.Plan, key string) domain.PlanItem {
	t.Helper()
	for _, it := range p.Items {
		if it.Ref.Kind == domain.ArtifactAgent && it.Ref.Key == key {
			return it
		}
	}
	t.Fatalf("plan has no item for agent %s", key)
	return domain.PlanItem{}
}

// ---------------------------------------------------------------------------
// Hand edits of a deployed agent, in either format
// ---------------------------------------------------------------------------

// owEditBody applies edit to the canonical body of the deployed agent (for Codex, the
// developer_instructions text) and writes the file back. It returns the new file bytes.
func owEditBody(t *testing.T, h owHarness, ws, key string, edit func(string) string) []byte {
	t.Helper()
	rel := h.rel(key)
	raw := string(gamRead(t, ws, rel))
	if h.formatID == "" {
		gamWrite(t, ws, rel, edit(raw))
		return gamRead(t, ws, rel)
	}
	return owRewriteToml(t, ws, rel, raw, func(doc map[string]any) {
		doc["developer_instructions"] = edit(doc["developer_instructions"].(string))
	})
}

// owEditFrontmatter sets one frontmatter key of the deployed agent (a Codex file: a top-level
// TOML key) and writes the file back. It returns the new file bytes.
func owEditFrontmatter(t *testing.T, h owHarness, ws, key, field, value string) []byte {
	t.Helper()
	rel := h.rel(key)
	raw := string(gamRead(t, ws, rel))
	if h.formatID != "" {
		return owRewriteToml(t, ws, rel, raw, func(doc map[string]any) { doc[field] = value })
	}
	lines := strings.Split(raw, "\n")
	replaced := false
	for i, l := range lines {
		if i > 0 && strings.HasPrefix(l, field+":") {
			lines[i] = field + ": " + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines[:1], append([]string{field + ": " + value}, lines[1:]...)...)
	}
	gamWrite(t, ws, rel, strings.Join(lines, "\n"))
	return gamRead(t, ws, rel)
}

// owRewriteToml edits the decoded TOML of a Codex file and writes it back with its leading
// stamp comments.
func owRewriteToml(t *testing.T, ws, rel, raw string, mutate func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := toml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("%s is not valid TOML: %v", rel, err)
	}
	mutate(doc)
	body, err := toml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal %s: %v", rel, err)
	}
	var stamps strings.Builder
	for _, line := range strings.SplitAfter(raw, "\n") {
		if !strings.HasPrefix(line, "#") {
			break
		}
		stamps.WriteString(line)
	}
	gamWrite(t, ws, rel, stamps.String()+"\n"+string(body))
	return gamRead(t, ws, rel)
}

// owReplaceOnce replaces the first occurrence of old and fails when old is absent, so a
// catalog change cannot silently turn an edit into a no-op.
func owReplaceOnce(t *testing.T, text, old, repl string) string {
	t.Helper()
	if !strings.Contains(text, old) {
		t.Fatalf("deployed agent does not contain %q", old)
	}
	return strings.Replace(text, old, repl, 1)
}

// The body edits of the five hand-edit scenarios.
func owOutsideEdit(t *testing.T) func(string) string {
	return func(s string) string { return s + "\nOutside edit.\n" }
}

func owIdentityEdit(t *testing.T) func(string) string {
	return func(s string) string {
		return owReplaceOnce(t, s, "Execute test suites", "Execute test suites, edited here")
	}
}

func owProjectEdit(t *testing.T) func(string) string {
	return func(s string) string {
		return owReplaceOnce(t, s, "<CodebaseContext type=\"project\">\n", "<CodebaseContext type=\"project\">\nTeam rules.\n")
	}
}

func owCustomEdit(t *testing.T) func(string) string {
	return func(s string) string { return s + "\n<UserNotes type=\"custom\">\nMy notes.\n</UserNotes>\n" }
}
