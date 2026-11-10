package app_test

// transform_run_helpers_test.go holds the shared builders of the whole-deployment transform
// service tests: a service wired with real built-in harness modules, the real executor, the
// real manifest store and a real TODO collector; a scripted interaction that also records
// the questions it was asked; workspaces seeded from the tool-local fixtures under
// testdata/transform; and file readers. Nothing here reads the repository Catalog.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/todo"
)

// errTrQuestionFailed is the error a failing question returns.
var errTrQuestionFailed = errors.New("question backend failed")

// trAgentFixtures names the fixtures that are MOSAIC agents, and their deployed models.
// The remaining fixtures (deploy-todo, no-frontmatter, plain-notes) are not agents.
var trAgentFixtures = []string{
	"harness-only", "orchestrator-script", "orchestrator", "standalone", "subagent", "utility",
}

// trNotAgentFixtures are the fixtures that are not MOSAIC agents.
var trNotAgentFixtures = []string{"deploy-todo", "no-frontmatter", "plain-notes"}

// trSpy is a scripted interaction that records the choice questions and review plans it saw.
type trSpy struct {
	*interactiontest.Stub
	choices []domain.ChoiceQuestion
	reviews []domain.Plan
	// failOn makes the SelectOne of this question id return errTrQuestionFailed.
	failOn domain.QuestionID
	// texts are the text questions asked; cancelTextOn makes that question id answer Cancelled.
	texts        []domain.TextQuestion
	cancelTextOn domain.QuestionID
	// atFirstQuestion runs once, when the first model, custom-tool or review question arrives.
	atFirstQuestion func()
}

func (s *trSpy) fireFirst() {
	if s.atFirstQuestion != nil {
		f := s.atFirstQuestion
		s.atFirstQuestion = nil
		f()
	}
}

func (s *trSpy) SelectOne(ctx context.Context, q domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	s.choices = append(s.choices, q)
	if q.ID == domain.QTransformTargetModel || q.ID == domain.QCustomTool {
		s.fireFirst()
	}
	if s.failOn != "" && q.ID == s.failOn {
		return domain.ChoiceAnswer{}, errTrQuestionFailed
	}
	return s.Stub.SelectOne(ctx, q)
}

// AskText records the text question; a question whose id is cancelTextOn is answered with a cancel.
func (s *trSpy) AskText(ctx context.Context, q domain.TextQuestion) (domain.TextAnswer, error) {
	s.texts = append(s.texts, q)
	if q.ID == domain.QCustomTool {
		s.fireFirst()
	}
	if s.cancelTextOn != "" && q.ID == s.cancelTextOn {
		return domain.TextAnswer{Status: domain.Cancelled}, nil
	}
	return s.Stub.AskText(ctx, q)
}

// customToolSubjects returns the subjects of the QCustomTool questions asked, in order.
func (s *trSpy) customToolSubjects() []string {
	var out []string
	for _, q := range s.texts {
		if q.ID == domain.QCustomTool {
			out = append(out, q.Subject)
		}
	}
	return out
}

func (s *trSpy) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	s.reviews = append(s.reviews, p)
	s.fireFirst()
	return s.Stub.Review(ctx, p)
}

// modelQuestions returns the QTransformTargetModel questions asked, in order.
func (s *trSpy) modelQuestions() []domain.ChoiceQuestion {
	var out []domain.ChoiceQuestion
	for _, q := range s.choices {
		if q.ID == domain.QTransformTargetModel {
			out = append(out, q)
		}
	}
	return out
}

// modelSubjects returns the subjects of the model questions asked, in order.
func (s *trSpy) modelSubjects() []string {
	var out []string
	for _, q := range s.modelQuestions() {
		out = append(out, q.Subject)
	}
	return out
}

// trRun is one wired service over a temporary workspace.
type trRun struct {
	t    *testing.T
	ws   string
	deps app.Deps
	svc  app.Service
	spy  *trSpy
	src  string
	tgt  string
}

// trNew wires a service for the harness pair over a fresh workspace. The script drives every
// question; a nil script answers nothing (unscripted questions are skipped, the plan is confirmed).
func trNew(t *testing.T, script *interactiontest.Stub, src, tgt string, opts ...func(*app.Deps)) *trRun {
	t.Helper()
	if script == nil {
		script = interactiontest.NewBuilder().Build()
	}
	spy := &trSpy{Stub: script}
	deps, ws := newBaseDeps(t, script)
	deps.Interaction = spy
	reg := &stubRegistry{modules: map[string]domain.HarnessModule{}}
	for _, id := range []string{hClaude, hOpen, hCLI, hVSCode} {
		m := rbModule(t, id)
		reg.list = append(reg.list, m.Ref())
		reg.modules[id] = m
	}
	deps.Registry = reg
	deps.Manifest = manifest.NewStore()
	deps.Todo = todo.NewCollector()
	deps.Executor = deploy.NewExecutor(deps.Manifest, deps.Logger, deps.Todo)
	for _, o := range opts {
		o(&deps)
	}
	return &trRun{t: t, ws: ws, deps: deps, svc: app.New(deps), spy: spy, src: src, tgt: tgt}
}

// request returns a fully pre-answered request for the run's pair that confirms the plan.
func (r *trRun) request() app.TransformHarnessRequest {
	return app.TransformHarnessRequest{
		SourceHarnessID: r.src,
		TargetHarnessID: r.tgt,
		Workspace:       r.ws,
		AutoConfirmPlan: true,
	}
}

// run calls TransformHarness with the request.
func (r *trRun) run(req app.TransformHarnessRequest) (app.TransformHarnessResult, error) {
	r.t.Helper()
	return r.svc.TransformHarness(context.Background(), req)
}

// mustRun calls TransformHarness and fails the test on an error.
func (r *trRun) mustRun(req app.TransformHarnessRequest) app.TransformHarnessResult {
	r.t.Helper()
	res, err := r.run(req)
	if err != nil {
		r.t.Fatalf("TransformHarness: %v", err)
	}
	return res
}

// module returns the built-in module of a harness id.
func (r *trRun) module(id string) domain.HarnessModule { return rbModule(r.t, id) }

// agentsDir returns the workspace-relative agents folder of a harness.
func (r *trRun) agentsDir(id string) string {
	return r.module(id).Descriptor().Paths.Agents.Project
}

// agentExt returns the agent file extension of a harness.
func (r *trRun) agentExt(id string) string {
	return r.module(id).Descriptor().Extensions[domain.ArtifactAgent]
}

// seed copies the named agent fixtures into the source harness's agents folder.
func (r *trRun) seed(names ...string) {
	r.t.Helper()
	for _, n := range names {
		r.put(r.src, n, trFixture(r.t, n))
	}
}

// put writes content as the deployed file of key in the harness's agents folder.
func (r *trRun) put(harnessID, key, content string) string {
	r.t.Helper()
	rel := r.agentsDir(harnessID) + "/" + key + r.agentExt(harnessID)
	trWrite(r.t, r.ws, rel, content)
	return rel
}

// dest returns the workspace-relative target path of an agent key.
func (r *trRun) dest(key string) string {
	return r.agentsDir(r.tgt) + "/" + key + r.agentExt(r.tgt)
}

// read returns a workspace-relative file, failing the test when it is missing.
func (r *trRun) read(rel string) string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.ws, filepath.FromSlash(rel)))
	if err != nil {
		r.t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// exists reports whether a workspace-relative file exists.
func (r *trRun) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(r.ws, filepath.FromSlash(rel)))
	return err == nil
}

// snapshot returns every file of the workspace.
func (r *trRun) snapshot() map[string]string { return trSnapshot(r.t, r.ws) }

// trFixture reads an agent fixture of testdata/transform/agents.
func trFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "transform", "agents", name+".md"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// trWrite writes content below root, creating folders.
func trWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// trSnapshot maps every file below root (relative, slash form) to its content.
func trSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// trSameTree reports a difference between two snapshots, or "" when they are equal.
func trSameTree(a, b map[string]string) string {
	for k, v := range a {
		w, ok := b[k]
		if !ok {
			return "file removed: " + k
		}
		if v != w {
			return "file changed: " + k
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			return "file added: " + k
		}
	}
	return ""
}

// trFile returns the outcome for an agent key, failing the test when it is absent.
func trFile(t *testing.T, res app.TransformHarnessResult, key string) app.TransformFileOutcome {
	t.Helper()
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactAgent && f.Key == key {
			return f
		}
	}
	t.Fatalf("no agent outcome for key %q; agent keys: %v", key, trAgentKeys(res))
	return app.TransformFileOutcome{}
}

// trAgentKeys lists the keys of the agent outcomes.
func trAgentKeys(res app.TransformHarnessResult) []string {
	var keys []string
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactAgent {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

// trFileBySource returns the agent outcome for a source file base name.
func trFileBySource(t *testing.T, res app.TransformHarnessResult, base string) app.TransformFileOutcome {
	t.Helper()
	for _, f := range res.Files {
		if f.Kind == domain.ArtifactAgent && filepath.Base(filepath.ToSlash(f.SourcePath)) == base {
			return f
		}
	}
	t.Fatalf("no agent outcome for source file %q", base)
	return app.TransformFileOutcome{}
}

// trNotes returns the result notes of one kind.
func trNotes(res app.TransformHarnessResult, kind app.TransformNoteKind) []app.TransformNote {
	var out []app.TransformNote
	for _, n := range res.Notes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// trFrontmatter returns the frontmatter lines of a document.
func trFrontmatter(doc string) []string {
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	var out []string
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			break
		}
		out = append(out, l)
	}
	return out
}

// trModelLine returns the top-level `model:` frontmatter line of a document and whether it exists.
func trModelLine(doc string) (string, bool) {
	for _, l := range trFrontmatter(doc) {
		if strings.HasPrefix(l, "model:") {
			return l, true
		}
	}
	return "", false
}

// trHasKey reports whether any top-level frontmatter line starts with "<key>:".
func trHasKey(doc, key string) bool {
	for _, l := range trFrontmatter(doc) {
		if strings.HasPrefix(l, key+":") {
			return true
		}
	}
	return false
}

// trDuplicates returns names that occur more than once.
func trDuplicates(names []string) []string {
	seen := map[string]int{}
	for _, n := range names {
		seen[n]++
	}
	var dup []string
	for n, c := range seen {
		if c > 1 {
			dup = append(dup, n)
		}
	}
	sort.Strings(dup)
	return dup
}

// trLoadManifest loads the workspace manifest.
func trLoadManifest(t *testing.T, ws string) domain.Manifest {
	t.Helper()
	snap, err := manifest.NewStore().Load(ws)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	return snap.Manifest
}

// trEntryAt returns the manifest entry recorded at a target path.
func trEntryAt(m domain.Manifest, path string) (domain.ManifestEntry, bool) {
	for _, e := range m.Entries {
		if filepath.ToSlash(e.TargetPath) == path {
			return e, true
		}
	}
	return domain.ManifestEntry{}, false
}
