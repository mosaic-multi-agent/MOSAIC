package app_test

// save_offer_helpers_test.go holds the shared builders of the save offer tests: a harness module
// that resolves a custom answer into MCP names like the descriptor-driven modules do (and maps the
// generic from a saved entry afterwards), a scripted confirmation of the offer, and a deploy-family
// run wired with both.

import (
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

// offerHarnessID is the id of the stub harness every deploy-family run targets.
const offerHarnessID = "stub-harness"

// offerNames writes entries in the stub harness's MCP form: mcp__server__tool, or mcp__server__*.
func offerNames(entries []domain.MCPEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		tool := e.Tool
		if tool == "" {
			tool = "*"
		}
		names = append(names, "mcp__"+e.Server+"__"+tool)
	}
	return names
}

// offerListNames are the entries of listFlowAnswer in the stub harness's names.
var offerListNames = []string{"mcp__github__get_pr", "mcp__jira__*"}

// offerMapping is the tool_destinations entry that reproduces the answer for generic.
func offerMapping(generic string, names ...string) domain.ToolMapping {
	return domain.ToolMapping{
		Generic:      generic,
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: names}},
	}
}

// offerModule resolves the listed generics as unmapped unless the user answered, skipped, or a
// saved entry maps them. A custom answer gives one main destination with the entries' names.
type offerModule struct {
	*stubHarnessModule
	unmapped map[string]bool
	saved    map[string][]domain.ToolDestination // generic -> destinations of the saved entry
}

// newOfferModule builds the module. withForms declares MCP name forms in its descriptor.
func newOfferModule(tier domain.ProvisionTier, withForms bool, unmapped []string, saved map[string][]domain.ToolDestination) *offerModule {
	base := newMinimalModule()
	base.ref.Tier = tier
	if withForms {
		base.descriptor.Tools.MCP = domain.MCPNameForms{ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}"}
	}
	set := make(map[string]bool, len(unmapped))
	for _, u := range unmapped {
		set[u] = true
	}
	return &offerModule{stubHarnessModule: base, unmapped: set, saved: saved}
}

func (m *offerModule) Tools(req domain.ToolRequest) (domain.ToolResult, error) {
	out := make([]domain.ToolResolution, len(req.Generic))
	for i, g := range req.Generic {
		out[i] = m.resolve(g, req)
	}
	return domain.ToolResult{Resolutions: out}, nil
}

func (m *offerModule) resolve(g string, req domain.ToolRequest) domain.ToolResolution {
	res := domain.ToolResolution{Generic: g, Outcome: domain.ToolMapped}
	if !m.unmapped[g] {
		return res
	}
	switch {
	case req.SkippedTools[g]:
		res.Outcome = domain.ToolSkipped
	case len(req.CustomNames[g]) > 0:
		names := offerNames(req.CustomNames[g])
		res.Outcome = domain.ToolCustom
		res.HarnessTools = names
		res.Destinations = []domain.ToolDestination{{Kind: domain.DestMain, Names: names}}
	case len(m.saved[g]) > 0:
		res.HarnessTools = slices.Clone(m.saved[g][0].Names)
		res.Destinations = m.saved[g]
	default:
		res.Outcome = domain.ToolUnmapped
	}
	return res
}

// savedFrom returns the saved entries of the stub harness held by the spy config, as an
// offerModule reads them when a registry is built.
func savedFrom(user *spyUserConfig) map[string][]domain.ToolDestination {
	saved := map[string][]domain.ToolDestination{}
	for _, p := range user.puts {
		if p.harnessID == offerHarnessID {
			saved[p.mapping.Generic] = p.mapping.Destinations
		}
	}
	return saved
}

// Scripted answers to the save offer.
func acceptOffer(domain.Question) (domain.ConfirmAnswer, error) {
	return domain.ConfirmAnswer{Status: domain.Answered, Confirm: true}, nil
}

func declineOffer(domain.Question) (domain.ConfirmAnswer, error) {
	return domain.ConfirmAnswer{Status: domain.Answered, Confirm: false}, nil
}

func skipOffer(domain.Question) (domain.ConfirmAnswer, error) {
	return domain.ConfirmAnswer{Status: domain.SkippedOne}, nil
}

func cancelOffer(domain.Question) (domain.ConfirmAnswer, error) {
	return domain.ConfirmAnswer{Status: domain.Cancelled}, nil
}

func failOffer(domain.Question) (domain.ConfirmAnswer, error) {
	return domain.ConfirmAnswer{}, errTrQuestionFailed
}

// offerSpy answers the save offer from a script function and records the offers and the order of
// custom tool questions and offers; every other call goes to the wrapped interaction.
type offerSpy struct {
	domain.Interaction
	answer func(domain.Question) (domain.ConfirmAnswer, error) // nil: accept
	offers []domain.Question
	order  []string // "ask:<generic>" and "offer:<generic>", in call order
}

func (s *offerSpy) AskText(ctx context.Context, q domain.TextQuestion) (domain.TextAnswer, error) {
	if q.ID == domain.QCustomTool {
		s.order = append(s.order, "ask:"+q.Subject)
	}
	return s.Interaction.AskText(ctx, q)
}

func (s *offerSpy) Confirm(ctx context.Context, q domain.Question) (domain.ConfirmAnswer, error) {
	if q.ID != domain.QSaveCustomTool {
		return s.Interaction.Confirm(ctx, q)
	}
	s.offers = append(s.offers, q)
	s.order = append(s.order, "offer:"+q.Subject)
	if s.answer == nil {
		return acceptOffer(q)
	}
	return s.answer(q)
}

// offerSetup varies the wiring of an offer run. The zero value is a builtin harness with MCP name
// forms whose offers are accepted and whose config saves succeed.
type offerSetup struct {
	tier    domain.ProvisionTier // zero: builtin
	noForms bool                 // the descriptor declares no MCP name forms
	answer  func(domain.Question) (domain.ConfirmAnswer, error)
	lockErr error // the config lock fails
	putErr  error // the put fails
}

// offerRun is a deploy-family run wired with an offerModule, a spy user config and an offerSpy.
type offerRun struct {
	*listFlowRun
	offer *offerSpy
	user  *spyUserConfig
}

// newOfferRun is a run in which two agents declare the unmapped generic "bash" and the user
// answers its question with answer.
func newOfferRun(t *testing.T, answer string, o offerSetup) *offerRun {
	t.Helper()
	agents := []domain.Agent{toolAgent("test-runner", "bash"), toolAgent("second-runner", "bash")}
	return newOfferRunFor(t, agents, []string{"bash"}, map[string]string{"bash": answer}, o)
}

// newOfferRunFor wires the given agents: every tool in unmapped is unmapped for the harness and
// answers holds the answer per generic (a tool without one is skipped).
func newOfferRunFor(t *testing.T, agents []domain.Agent, unmapped []string, answers map[string]string, o offerSetup) *offerRun {
	t.Helper()
	r := newListFlowRunWith(t, agents, unmapped, answers)
	user := &spyUserConfig{
		path:    filepath.Join(t.TempDir(), "user-config.yaml"),
		lockErr: o.lockErr, putErr: o.putErr,
	}
	r.deps.UserConfig = user
	tier := o.tier
	if tier == "" {
		tier = domain.TierBuiltin
	}
	cm := r.deps.Registry.(*stubRegistry).modules[offerHarnessID].(*capturingToolsModule)
	cm.base = newOfferModule(tier, !o.noForms, unmapped, nil)
	spy := &offerSpy{Interaction: r.deps.Interaction, answer: o.answer}
	r.deps.Interaction = spy
	return &offerRun{listFlowRun: r, offer: spy, user: user}
}

// notices returns the notices the run sent.
func (r *offerRun) notices() []domain.Notice { return r.spy.Stub.Notices() }

// noticesAt returns the notices of the given level.
func noticesAt(all []domain.Notice, level domain.NoticeLevel) []domain.Notice {
	var out []domain.Notice
	for _, n := range all {
		if n.Level == level {
			out = append(out, n)
		}
	}
	return out
}

// runOutcome is what a deploy-family run reports about the answered tool, independent of the offer.
type runOutcome struct {
	PlanGaps []domain.Gap
	Items    []domain.PlanItem // the planned writes: targets, actions, reasons
	TodoGaps []domain.Gap
	Summary  []domain.TodoItem
	Entries  map[string][]string // agent key -> the distinct answers (neutral text) its tool requests carried
}

// outcome collects the run's outcome after flow returned summary.
func (r *offerRun) outcome(summary domain.RunSummary) runOutcome {
	out := runOutcome{TodoGaps: r.todo.gaps, Summary: summary.Todos, Entries: map[string][]string{}}
	if r.exec.capturedPlan != nil {
		out.PlanGaps = r.exec.capturedPlan.Gaps
		out.Items = r.exec.capturedPlan.Items
	}
	for _, req := range *r.reqs {
		if req.AgentKey == "" || len(req.CustomNames["bash"]) == 0 {
			continue
		}
		text := customtool.Text(req.CustomNames["bash"])
		if !slices.Contains(out.Entries[req.AgentKey], text) {
			out.Entries[req.AgentKey] = append(out.Entries[req.AgentKey], text)
		}
	}
	return out
}

func assertSameOutcome(t *testing.T, got, want runOutcome) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the run's output differs from the run with the offer accepted:\n got: %+v\nwant: %+v", got, want)
	}
}

// noticeMentions reports whether the notice text names every word.
func noticeMentions(n domain.Notice, words ...string) bool {
	text := n.Title + " " + n.Message
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}
