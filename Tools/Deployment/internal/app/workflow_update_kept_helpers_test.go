package app_test

// workflow_update_kept_helpers_test.go holds the builders of the Update workflows tests that
// run over a workspace whose orchestrator-role files hold custom workflows: workflows the
// catalog stack does not supply. The workspace comes from a real deploy of the workflowpair
// catalog; custom blocks are then planted into the workflow region of the deployed files and
// the manifest hashes are re-recorded, so each file is still the file the manifest describes.
// The interaction records the workflow checklist question and answers it with a chosen set.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	wukCustomHint = "not in catalog; deployed only"

	wukIDX = "custom-x"
	wukIDW = "custom-w"
	wukIDZ = "custom-z"
)

// Deployed custom blocks. X has a ragged routing table and spaces after its closing tag, so a
// rewritten block cannot equal the deployed one by accident.
const (
	wukBlockX = "<Workflow type=\"managed\" name=\"custom-x\" version=\"1.0\">\n## Custom X\n\n" +
		"| A | B | C |\n|---|---|---|\n| 1 | 2 |\n| 1 | 2 | 3 | 4 |\nWUK-X-BODY\n</Workflow>  \n"
	wukBlockWOrch   = "<Workflow type=\"managed\" name=\"custom-w\" version=\"1.0\">\n## Custom W\nWUK-W-ORCH-BODY\n</Workflow>\n"
	wukBlockWScript = "<Workflow type=\"managed\" name=\"custom-w\" version=\"2.0\">\n## Custom W\nWUK-W-SCRIPT-BODY\n</Workflow>\n"
	wukBlockZ       = "<Workflow type=\"managed\" name=\"custom-z\" version=\"1.0\">\n## Custom Z\n\n" +
		"| A | B |\n|---|---|\n| 1 |\nWUK-Z-BODY\n</Workflow>   \n"
)

// wukChecklist is an Interaction that records the workflow checklist question, answers it with
// a chosen selection, and keeps every plan it is asked to review.
type wukChecklist struct {
	*interactiontest.Stub
	answer []string
	asked  []domain.ChoiceQuestion
	plans  []domain.Plan
}

func newWukChecklist(answer ...string) *wukChecklist {
	return &wukChecklist{Stub: interactiontest.NewBuilder().Build(), answer: answer}
}

func (c *wukChecklist) SelectMany(ctx context.Context, q domain.ChoiceQuestion) (domain.MultiChoiceAnswer, error) {
	if q.ID != domain.QWorkflows {
		return c.Stub.SelectMany(ctx, q)
	}
	c.asked = append(c.asked, q)
	return domain.MultiChoiceAnswer{Status: domain.Answered, OptionIDs: c.answer}, nil
}

func (c *wukChecklist) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	c.plans = append(c.plans, p)
	return c.Stub.Review(ctx, p)
}

// wukPlant inserts the blocks at the end of the workflow region of one orchestrator-role file
// and re-records the file's manifest hash.
func wukPlant(t *testing.T, ws, key string, blocks ...string) {
	t.Helper()
	rel := wpsRel(t, hClaude, key)
	doc := string(gamRead(t, ws, rel))
	at := strings.Index(doc, wpsRegionClose)
	if at < 1 || doc[at-1] != '\n' {
		t.Fatalf("%s: workflow region close tag is not at the start of a line", key)
	}
	doc = doc[:at] + strings.Join(blocks, "") + doc[at:]
	gamWrite(t, ws, rel, doc)
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key {
			m.Entries[i].ContentHash = manifest.Hash([]byte(doc))
		}
	}
	gamSaveManifest(t, ws, m)
}

// wukWorkspace deploys the parity workflow to a fresh workspace and plants the custom blocks:
// inOrch into the orchestrator, inScript into the orchestrator-script.
func wukWorkspace(t *testing.T, inOrch, inScript []string) string {
	t.Helper()
	ws := wpsDeploy(t, hClaude, []string{wpsFlowParity})
	if len(inOrch) > 0 {
		wukPlant(t, ws, wpsOrch, inOrch...)
	}
	if len(inScript) > 0 {
		wukPlant(t, ws, wpsScript, inScript...)
	}
	return ws
}

// wukRun runs Update workflows with no preset selection, so the run asks the checklist, and the
// checklist is answered with the capture's answer.
func wukRun(t *testing.T, ws string, c *wukChecklist) domain.RunSummary {
	t.Helper()
	svc := wpsService(t, func(d *app.Deps) { d.Interaction = c })
	sum, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: hClaude, WorkspacePath: ws, AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	if err != nil {
		t.Fatalf("UpdateWorkflows: %v", err)
	}
	return sum
}

// wukIDsIn returns the workflow ids, in file order, of a deployed orchestrator-role file.
func wukIDsIn(t *testing.T, ws, key string) []string {
	t.Helper()
	blocks := keptBlocksIn(t, key, gamRead(t, ws, wpsRel(t, hClaude, key)))
	return keptIDsOf(blocks)
}

// wukOption returns the checklist option with the id.
func wukOption(q domain.ChoiceQuestion, id string) (domain.Option, bool) {
	for _, o := range q.Options {
		if o.ID == id {
			return o, true
		}
	}
	return domain.Option{}, false
}

// wukRemovalNotices returns the run-summary items that report a removed workflow id.
func wukRemovalNotices(sum domain.RunSummary, id string) []domain.TodoItem {
	var out []domain.TodoItem
	for _, item := range sum.Todos {
		if item.Kind == domain.GapWorkflowRemoved && (item.Subject == id || strings.Contains(item.Detail, id)) {
			out = append(out, item)
		}
	}
	return out
}

// wukMentions lists every place of the planned items and the summary that names the id in a
// version comparison: a stale delta of a planned item or a summary item other than the kept and
// removed notices.
func wukMentions(sum domain.RunSummary, c *wukChecklist, id string) []string {
	var out []string
	for _, p := range c.plans {
		for _, item := range p.Items {
			if len(item.Stale) > 0 && strings.Contains(fmt.Sprintf("%+v", item.Stale), id) {
				out = append(out, fmt.Sprintf("plan item %s: %+v", item.Ref.Key, item.Stale))
			}
		}
	}
	for _, item := range sum.Todos {
		if item.Kind != domain.GapWorkflowKept && item.Kind != domain.GapWorkflowRemoved &&
			strings.Contains(item.Subject+item.Detail, id) {
			out = append(out, fmt.Sprintf("summary item %s: %s", item.Subject, item.Detail))
		}
	}
	return out
}
