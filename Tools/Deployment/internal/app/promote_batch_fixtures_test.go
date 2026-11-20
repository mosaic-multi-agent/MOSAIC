package app_test

// promote_batch_fixtures_test.go holds the fixtures shared by the PromoteBatch tests: a harness
// that declares a model key and maps tools, harness agent files for an agents folder, catalog
// layers seeded with a tier and categories, and a scripted interaction that records every
// question together with the number of files the layers held when it was asked. Nothing here
// is a test.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

const (
	batchHarnessID = "tool-resolution-harness" // maps "bash"; any other tool name is unmapped
	batchSeedID    = "900"                     // numeric id of the largest agent the layers hold
)

// batchHarnessModule is the tool-resolution harness with a model key, optionally one whose
// tool information cannot be recovered.
type batchHarnessModule struct {
	*toolResolutionHarnessModule
	unrecoverable bool
	modelKey      string // descriptor model key; empty = "model"
	agentExt      string // descriptor agent extension; empty = none declared
}

func (m batchHarnessModule) Descriptor() *domain.HarnessDescriptor {
	d := m.toolResolutionHarnessModule.Descriptor()
	d.Frontmatter.ModelKey = "model"
	if m.modelKey != "" {
		d.Frontmatter.ModelKey = m.modelKey
	}
	d.ToolInfoUnrecoverable = m.unrecoverable
	if m.agentExt != "" {
		d.Extensions = map[domain.ArtifactKind]string{domain.ArtifactAgent: m.agentExt}
	}
	return d
}

func withBatchHarness(deps app.Deps, unrecoverable bool) app.Deps {
	return withBatchHarnessKey(deps, unrecoverable, "")
}

// withBatchHarnessKey is withBatchHarness with a descriptor model key other than "model".
func withBatchHarnessKey(deps app.Deps, unrecoverable bool, modelKey string) app.Deps {
	return withBatchHarnessModule(deps, batchHarnessModule{
		toolResolutionHarnessModule: &toolResolutionHarnessModule{ref: toolResolutionHarness},
		unrecoverable:               unrecoverable, modelKey: modelKey})
}

// withBatchHarnessModule installs mod as the batch harness.
func withBatchHarnessModule(deps app.Deps, mod batchHarnessModule) app.Deps {
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{toolResolutionHarness},
		modules: map[string]domain.HarnessModule{batchHarnessID: mod},
	}
	return deps
}

// batchSrc describes one harness agent file of an agents folder.
type batchSrc struct {
	File  string   // file name in the folder
	Model string   // value of the model key; empty = no model
	Tools []string // harness tool names; nil = no tools key
	Desc  string   // description; empty = a default
	Full  bool     // carries recommended_tier, tier_rationale and required_skills
	Key   string   // frontmatter key that holds Model; empty = "model"
	Extra string   // raw extra frontmatter lines (each ending in a newline)
}

func (b batchSrc) bytes() []byte {
	desc := b.Desc
	if desc == "" {
		desc = "A harness agent for batch promote tests"
	}
	s := "---\ntransform_version: \"2.1.0\"\ninjections_version: \"3.0.0\"\nbundle_version: \"1.0.0\"\n" +
		"protocol_version: \"1.9\"\nid: \"77\"\nversion: \"1.5.0\"\nname: " + batchKey(b.File) + "\n" +
		"description: " + desc + "\nrole: subagent\n"
	if b.Model != "" {
		key := b.Key
		if key == "" {
			key = "model"
		}
		s += key + ": " + b.Model + "\n"
	}
	s += b.Extra
	if b.Tools != nil {
		s += "tools: [" + strings.Join(b.Tools, ", ") + "]\n"
	}
	if b.Full {
		s += "recommended_tier: MEDIUM\ntier_rationale: moderate capability\nrequired_skills: [lean-tdd]\n"
	}
	return []byte(s + "---\n<Identity type=\"core\">\nYou are a batch promote test agent.\n</Identity>\n")
}

// batchKey is the agent key of an agent file name.
func batchKey(file string) string {
	return strings.TrimSuffix(strings.TrimSuffix(file, ".md"), ".agent")
}

// writeBatchFolder writes the agent files into a new folder and returns it.
func writeBatchFolder(t *testing.T, srcs ...batchSrc) string {
	t.Helper()
	dir := t.TempDir()
	for _, s := range srcs {
		writeSysFile(t, filepath.Join(dir, s.File), string(s.bytes()))
	}
	return dir
}

// writeBatchFile writes raw content as a file of dir.
func writeBatchFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	writeSysFile(t, filepath.Join(dir, name), string(content))
}

// newBatchLayers creates user layers (highest first) that carry the HIGH tier and the Audit and
// Build categories. The largest numeric id in the layers is batchSeedID.
func newBatchLayers(t *testing.T, ids ...string) *plLayerSet {
	t.Helper()
	ls := newPlLayerSet(t, ids...)
	seed := "---\nid: \"" + batchSeedID + "\"\nversion: \"1.0.0\"\nname: tier-seed\n" +
		"description: Seed.\nrole: subagent\nrecommended_tier: HIGH\nrequired_skills: []\n---\nBody.\n"
	writeSysFile(t, filepath.Join(ls.roots[ids[0]], "Subagents", "Audit", "tier-seed.md"), seed)
	for _, id := range ids {
		for _, c := range []string{"Audit", "Build"} {
			mkdirBatch(t, filepath.Join(ls.roots[id], "Subagents", c))
		}
	}
	return ls
}

func mkdirBatch(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
}

// batchDeps installs the stack of ls and the batch harness.
func (ls *plLayerSet) batchDeps(t *testing.T, ix domain.Interaction) app.Deps {
	t.Helper()
	return withBatchHarness(ls.deps(t, ix), false)
}

// registerDefault makes the MOSAIC default catalog (id plmosaicdefault) addressable in ls.roots,
// with an Audit category, and returns its root. It does not change the stack.
func (ls *plLayerSet) registerDefault(t *testing.T) string {
	t.Helper()
	defRoot := filepath.Join(ls.mosaicRoot, "Catalog")
	ls.roots["plmosaicdefault"] = defRoot
	mkdirBatch(t, filepath.Join(defRoot, "Subagents", "Audit"))
	return defRoot
}

// withDefaultLayer adds the MOSAIC default catalog as the lowest layer of the stack and
// returns deps over the user layers plus that layer. The default layer's id is plmosaicdefault.
func (ls *plLayerSet) withDefaultLayer(t *testing.T, ix domain.Interaction) app.Deps {
	t.Helper()
	defRoot := ls.registerDefault(t)
	layers := []stack.Layer{}
	for _, id := range ls.order {
		layers = append(layers, stack.Layer{Root: ls.roots[id]})
	}
	layers = append(layers, stack.Layer{Root: defRoot, Default: true})
	st, err := stack.New(ls.mosaicRoot, layers)
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	deps.Interaction = ix
	deps.Catalog = st
	deps.MosaicRoot = ls.mosaicRoot
	return withBatchHarness(deps, false)
}

// layerLabel is the label PromoteBatch gives a user layer created by newPlLayerSet.
func layerLabel(id string) string { return "Fixture " + id + " (" + id + ")" }

const defaultLayerLabel = "MOSAIC default catalog (plmosaicdefault)"

// batchAsk is one recorded question.
type batchAsk struct {
	ID      domain.QuestionID
	Subject string
	Choice  domain.ChoiceQuestion
	Text    domain.TextQuestion
	Files   int // files the watched roots held when the question was asked
}

type askKey struct {
	id      domain.QuestionID
	subject string
}

// batchSpy is a scripted domain.Interaction. An answer is looked up by (ID, subject), then by
// ID, then in the per-ID sequence (the last element repeats); an unscripted question is
// skipped. Every question is recorded.
type batchSpy struct {
	*interactiontest.Stub
	t        *testing.T
	watch    []string
	asks     []batchAsk
	many     domain.MultiChoiceAnswer
	manyErr  error
	one      map[askKey]domain.ChoiceAnswer
	oneByID  map[domain.QuestionID]domain.ChoiceAnswer
	oneSeq   map[domain.QuestionID][]domain.ChoiceAnswer
	text     map[askKey]domain.TextAnswer
	textByID map[domain.QuestionID]domain.TextAnswer
	err      map[askKey]error
	after    func(domain.QuestionID, string) // called after each question is answered
}

func newBatchSpy(t *testing.T, watch ...string) *batchSpy {
	return &batchSpy{
		Stub: interactiontest.NewBuilder().Build(), t: t, watch: watch,
		one:      map[askKey]domain.ChoiceAnswer{},
		oneByID:  map[domain.QuestionID]domain.ChoiceAnswer{},
		oneSeq:   map[domain.QuestionID][]domain.ChoiceAnswer{},
		text:     map[askKey]domain.TextAnswer{},
		textByID: map[domain.QuestionID]domain.TextAnswer{},
		err:      map[askKey]error{},
	}
}

// batchSpyFor watches every layer root of ls and selects the given files.
func batchSpyFor(t *testing.T, ls *plLayerSet, files ...string) *batchSpy {
	t.Helper()
	var roots []string
	for _, r := range ls.roots {
		roots = append(roots, r)
	}
	return newBatchSpy(t, roots...).selecting(files...)
}

func (s *batchSpy) selecting(files ...string) *batchSpy {
	s.many = domain.MultiChoiceAnswer{Status: domain.Answered, OptionIDs: files}
	return s
}

func (s *batchSpy) fileCount() int {
	n := 0
	for _, r := range s.watch {
		n += len(snapshotTree(s.t, r))
	}
	return n
}

func (s *batchSpy) record(id domain.QuestionID, subject string) batchAsk {
	a := batchAsk{ID: id, Subject: subject, Files: s.fileCount()}
	s.asks = append(s.asks, a)
	return a
}

func (s *batchSpy) SelectMany(_ context.Context, q domain.ChoiceQuestion) (domain.MultiChoiceAnswer, error) {
	s.record(q.ID, q.Subject)
	s.asks[len(s.asks)-1].Choice = q
	defer s.afterAsk(q.ID, q.Subject)
	return s.many, s.manyErr
}

func (s *batchSpy) SelectOne(ctx context.Context, q domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	s.record(q.ID, q.Subject)
	s.asks[len(s.asks)-1].Choice = q
	defer s.afterAsk(q.ID, q.Subject)
	if err := ctx.Err(); err != nil {
		return domain.ChoiceAnswer{}, err
	}
	k := askKey{q.ID, q.Subject}
	if err, ok := s.err[k]; ok {
		return domain.ChoiceAnswer{}, err
	}
	if a, ok := s.one[k]; ok {
		return a, nil
	}
	if a, ok := s.oneByID[q.ID]; ok {
		return a, nil
	}
	if seq := s.oneSeq[q.ID]; len(seq) > 0 {
		n := 0
		for _, a := range s.asks {
			if a.ID == q.ID {
				n++
			}
		}
		return seq[min(n, len(seq))-1], nil
	}
	return domain.ChoiceAnswer{Status: domain.SkippedOne}, nil
}

func (s *batchSpy) AskText(ctx context.Context, q domain.TextQuestion) (domain.TextAnswer, error) {
	s.record(q.ID, q.Subject)
	s.asks[len(s.asks)-1].Text = q
	defer s.afterAsk(q.ID, q.Subject)
	if err := ctx.Err(); err != nil {
		return domain.TextAnswer{}, err
	}
	k := askKey{q.ID, q.Subject}
	if err, ok := s.err[k]; ok {
		return domain.TextAnswer{}, err
	}
	if a, ok := s.text[k]; ok {
		return a, nil
	}
	if a, ok := s.textByID[q.ID]; ok {
		return a, nil
	}
	return domain.TextAnswer{Status: domain.SkippedOne}, nil
}

func (s *batchSpy) afterAsk(id domain.QuestionID, subject string) {
	if s.after != nil {
		s.after(id, subject)
	}
}

// asked returns the recorded questions with the given ID, in ask order.
func (s *batchSpy) asked(id domain.QuestionID) []batchAsk {
	var out []batchAsk
	for _, a := range s.asks {
		if a.ID == id {
			out = append(out, a)
		}
	}
	return out
}

func (s *batchSpy) askedSubject(id domain.QuestionID, subject string) int {
	n := 0
	for _, a := range s.asked(id) {
		if a.Subject == subject {
			n++
		}
	}
	return n
}

func chosen(optionID string) domain.ChoiceAnswer {
	return domain.ChoiceAnswer{Status: domain.Answered, OptionID: optionID}
}

func chosenCustom(text string) domain.ChoiceAnswer {
	return domain.ChoiceAnswer{Status: domain.Answered, Custom: text}
}

func status(s domain.AnswerStatus) domain.ChoiceAnswer { return domain.ChoiceAnswer{Status: s} }

func typed(text string) domain.TextAnswer {
	return domain.TextAnswer{Status: domain.Answered, Text: text}
}

func textStatus(s domain.AnswerStatus) domain.TextAnswer { return domain.TextAnswer{Status: s} }

// runBatch runs PromoteBatch for the batch harness over dir.
func runBatch(t *testing.T, deps app.Deps, dir string) (app.PromoteBatchResult, error) {
	t.Helper()
	return app.New(deps).PromoteBatch(context.Background(), app.PromoteBatchRequest{HarnessID: batchHarnessID, AgentsDir: dir})
}

// mustRunBatch runs PromoteBatch and fails the test on a returned error.
func mustRunBatch(t *testing.T, deps app.Deps, dir string) app.PromoteBatchResult {
	t.Helper()
	res, err := runBatch(t, deps, dir)
	if err != nil {
		t.Fatalf("PromoteBatch: %v", err)
	}
	return res
}

// agentResult returns the result entry of the agent with the given file name.
func agentResult(t *testing.T, res app.PromoteBatchResult, file string) app.PromoteBatchAgentResult {
	t.Helper()
	for _, a := range res.Agents {
		if filepath.Base(a.SourcePath) == file {
			return a
		}
	}
	t.Fatalf("result has no entry for %q: %+v", file, res.Agents)
	return app.PromoteBatchAgentResult{}
}

// requirePromoted fails unless the entry is promoted, and returns it.
func requirePromoted(t *testing.T, res app.PromoteBatchResult, file string) app.PromoteBatchAgentResult {
	t.Helper()
	a := agentResult(t, res, file)
	if a.Status != app.PromoteBatchPromoted {
		t.Fatalf("%s status = %q (reason %q, err %v), want promoted", file, a.Status, a.Reason, a.Err)
	}
	return a
}
