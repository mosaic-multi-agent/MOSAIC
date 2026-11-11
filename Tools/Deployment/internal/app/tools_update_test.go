package app_test

// tools_update_test.go verifies what an update does with the tools field of a deployed agent:
// a change of the agent's catalog tools or of the tool mappings reaches the written agent while
// tools the user added stay, and the plan (review and dry run) names the built-in tools the
// update adds and removes as reasons of the item. The plan names exactly what the write does.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	reasonAddedPrefix   = "built-in tools added: "
	reasonRemovedPrefix = "built-in tools removed: "
)

// reasonWithPrefix returns the reason of item that starts with prefix, "" when none does.
func reasonWithPrefix(item domain.PlanItem, prefix string) string {
	for _, r := range item.Reasons {
		if strings.HasPrefix(r, prefix) {
			return r
		}
	}
	return ""
}

func requireToolsRunItem(t *testing.T, run *tuRun) domain.PlanItem {
	t.Helper()
	if run.err != nil {
		t.Fatalf("DeployAgents: %v", run.err)
	}
	item, ok := run.item()
	if !ok {
		t.Fatalf("reviewed plan has no item for %s", tuAgentPath)
	}
	if item.Action != domain.ActionUpdate {
		t.Fatalf("Action = %q (reasons %q), want %q", item.Action, item.Reasons, domain.ActionUpdate)
	}
	return item
}

func TestUpdateTools_CatalogToolsChange_PlanNamesAddedAndRemovedBuiltIns(t *testing.T) {
	// The deployed agent holds read and bash plus the user's own my-tool; the catalog now
	// produces read and edit.
	run := runToolsUpdate(t, tuDeployed("0.9.0", "[read, bash, my-tool]", ""), nil)

	item := requireToolsRunItem(t, run)

	if got := reasonWithPrefix(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
		t.Errorf("added reason = %q, want %q; reasons: %q", got, reasonAddedPrefix+"edit", item.Reasons)
	}
	if got := reasonWithPrefix(item, reasonRemovedPrefix); got != reasonRemovedPrefix+"bash" {
		t.Errorf("removed reason = %q, want %q; reasons: %q", got, reasonRemovedPrefix+"bash", item.Reasons)
	}
	for _, r := range item.Reasons {
		if strings.Contains(r, "my-tool") {
			t.Errorf("reason %q names a custom tool; only built-in changes are reported", r)
		}
	}
}

func TestUpdateTools_CatalogToolsChange_WrittenAgentFollowsCatalogAndKeepsCustomTool(t *testing.T) {
	run := runToolsUpdate(t, tuDeployed("0.9.0", "[read, bash, my-tool]", ""), nil)

	requireToolsRunItem(t, run)

	line := toolsLine(run.written(t))
	for _, want := range []string{"read", "edit", "my-tool"} {
		if !strings.Contains(line, want) {
			t.Errorf("written tools line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "bash") {
		t.Errorf("written tools line %q still holds the removed built-in bash", line)
	}
}

func TestUpdateTools_MappingsVersionChange_ReachesWrittenAgent(t *testing.T) {
	// The agent is otherwise current (same version); only the tool-mappings stamp differs and the
	// deployed field no longer matches the catalog's tools.
	run := runToolsUpdate(t, tuDeployed("1.0.0", "[read, bash, my-tool]", "oldhash"), nil)

	item := requireToolsRunItem(t, run)

	if !strings.Contains(strings.Join(item.Reasons, "; "), "tool_mappings_version") {
		t.Errorf("reasons %q do not name the tool_mappings_version change", item.Reasons)
	}
	if got := reasonWithPrefix(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
		t.Errorf("added reason = %q, want %q; reasons: %q", got, reasonAddedPrefix+"edit", item.Reasons)
	}
	if got := reasonWithPrefix(item, reasonRemovedPrefix); got != reasonRemovedPrefix+"bash" {
		t.Errorf("removed reason = %q, want %q; reasons: %q", got, reasonRemovedPrefix+"bash", item.Reasons)
	}
	line := toolsLine(run.written(t))
	if !strings.Contains(line, "edit") || strings.Contains(line, "bash") || !strings.Contains(line, "my-tool") {
		t.Errorf("written tools line %q, want edit added, bash removed, my-tool kept", line)
	}
}

func TestUpdateTools_NoBuiltInChange_PlanHasNoToolsReasons(t *testing.T) {
	run := runToolsUpdate(t, tuDeployed("0.9.0", "[read, edit, my-tool]", ""), nil)

	item := requireToolsRunItem(t, run)

	if r := reasonWithPrefix(item, reasonAddedPrefix); r != "" {
		t.Errorf("unexpected reason %q: no built-in tool is added", r)
	}
	if r := reasonWithPrefix(item, reasonRemovedPrefix); r != "" {
		t.Errorf("unexpected reason %q: no built-in tool is removed", r)
	}
}

func TestUpdateTools_RunProducedCustomTool_WrittenAndNotReportedAsBuiltInChange(t *testing.T) {
	// The catalog lists gen_mcp and the run answers it with the custom server name. The name is
	// added to the deployed field, and is never listed among built-in changes.
	source := strings.Replace(tuCatalogSource, "tools: [gen_read, gen_edit]", "tools: [gen_read, gen_edit, gen_mcp]", 1)
	setup := tuSetup{registry: tuRegistry(), source: source}
	run := runToolsUpdateWith(t, setup, tuDeployed("0.9.0", "[read, edit]", ""), func(r *app.DeployAgentsRequest) {
		r.CustomTools = map[string]string{"gen_mcp": "my-server"}
	})

	item := requireToolsRunItem(t, run)

	if r := reasonWithPrefix(item, reasonAddedPrefix); r != "" {
		t.Errorf("unexpected reason %q: a custom tool is not a built-in change", r)
	}
	if line := toolsLine(run.written(t)); !strings.Contains(line, "my-server") {
		t.Errorf("written tools line %q lacks the run-produced custom tool my-server", line)
	}
}

func TestUpdateTools_Permission_PlanAndWrittenAgentAgree(t *testing.T) {
	// Deployed: read allowed, edit denied, bash allowed, plus the user's srv_* denial. The catalog
	// grants read and edit. Edit becomes granted, bash stops being granted, srv_* stays denied.
	deployed := tuDeployed("0.9.0", "\n  read: allow\n  edit: deny\n  bash: allow\n  srv_*: deny", "")
	setup := tuSetup{registry: tuPermRegistry(), source: tuCatalogSource}
	run := runToolsUpdateWith(t, setup, deployed, nil)

	item := requireToolsRunItem(t, run)

	if got := reasonWithPrefix(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
		t.Errorf("added reason = %q, want %q; reasons: %q", got, reasonAddedPrefix+"edit", item.Reasons)
	}
	if got := reasonWithPrefix(item, reasonRemovedPrefix); got != reasonRemovedPrefix+"bash" {
		t.Errorf("removed reason = %q, want %q; reasons: %q", got, reasonRemovedPrefix+"bash", item.Reasons)
	}
	want := "tools:\n  read: allow\n  edit: allow\n  bash: deny\n  srv_*: deny"
	if got := toolsBlock(run.written(t)); got != want {
		t.Errorf("written tools block = %q, want %q", got, want)
	}
}

func TestUpdateTools_DryRun_PlanCarriesToolsReasonsAndNothingIsWritten(t *testing.T) {
	deployed := tuDeployed("0.9.0", "[read, bash, my-tool]", "")
	run := runToolsUpdate(t, deployed, func(r *app.DeployAgentsRequest) { r.DryRun = true })

	if run.err != nil {
		t.Fatalf("DeployAgents: %v", run.err)
	}
	if run.summary.DryRunPlan == nil {
		t.Fatal("summary carries no dry-run plan")
	}
	var item domain.PlanItem
	for _, it := range run.summary.DryRunPlan.Items {
		if it.TargetPath == tuAgentPath {
			item = it
		}
	}
	if got := reasonWithPrefix(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
		t.Errorf("dry-run added reason = %q, want %q", got, reasonAddedPrefix+"edit")
	}
	if got := reasonWithPrefix(item, reasonRemovedPrefix); got != reasonRemovedPrefix+"bash" {
		t.Errorf("dry-run removed reason = %q, want %q", got, reasonRemovedPrefix+"bash")
	}
	if run.written(t) != deployed {
		t.Error("a dry run changed the deployed agent")
	}
}
