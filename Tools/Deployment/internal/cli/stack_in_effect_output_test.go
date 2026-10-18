package cli_test

// stack_in_effect_output_test.go verifies that every catalog-loading subcommand reports the
// stack in effect, which it obtains from the app service's active-stack method:
//
//   - human output shows a "Catalog stack (<origin>):" block, one line per layer highest first,
//     before the subcommand's own result, for the stack-input and persisted origins
//   - human output is unchanged when the origin is default
//   - JSON output carries the stack in effect as one additive top-level key for all three
//     origins, and every key the document had before is still present with the same value
//   - the whole JSON stdout stays a single document

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

// sieSpy is a spyService whose active stack is set by the test.
type sieSpy struct {
	*spyService
	active stackconfig.ActiveStack
}

func (s sieSpy) ActiveStack() stackconfig.ActiveStack { return s.active }

var sieUserLayer = domain.CatalogLayer{
	Identity: domain.CatalogIdentity{ID: "cat-user", Name: "User Catalog", Version: "2.0.0"},
	Root:     "/catalogs/user",
}

var sieDefaultLayer = domain.CatalogLayer{
	Identity: domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC Default", Version: "1.4.0"},
	Root:     "/mosaic/Catalog",
	Default:  true,
}

func sieStack(origin stackconfig.Origin, layers ...domain.CatalogLayer) stackconfig.ActiveStack {
	return stackconfig.ActiveStack{Layers: layers, Origin: origin}
}

// sieCommand is one catalog-loading subcommand with the arguments that make it succeed against
// a spy, the spy preparation, and the JSON key the stack in effect is expected under.
type sieCommand struct {
	name     string
	args     func(workspace string) []string
	prepare  func(s *spyService, workspace string)
	extraKey string
}

func sieCommands() []sieCommand {
	run := func(s *spyService, ws string) {
		s.deployResp, s.updateResp, s.workflowsResp = successSummary(ws), successSummary(ws), successSummary(ws)
		s.deployAgentsResp, s.deployHooksResp = successSummary(ws), successSummary(ws)
	}
	deployLike := func(sub string) func(string) []string {
		return func(ws string) []string {
			return []string{sub, "--harness", "stub-harness", "--workspace", ws, "--auto-confirm"}
		}
	}
	plain := func(sub string) func(string) []string {
		return func(ws string) []string { return []string{sub, "--harness", "stub-harness", "--workspace", ws} }
	}
	return []sieCommand{
		{"deploy", deployLike("deploy"), run, "StackInEffect"},
		{"update", deployLike("update"), run, "StackInEffect"},
		{"workflows", deployLike("workflows"), run, "StackInEffect"},
		{"agents", plain("agents"), run, "StackInEffect"},
		{"hooks", plain("hooks"), run, "StackInEffect"},
		{"render", func(string) []string {
			return []string{"render", "--harness", "claude-code", "--source", "/agents/agent.md", "--dest", "/out/agent.md"}
		}, func(s *spyService, _ string) {
			s.renderAgentResp = app.RenderAgentResult{DestinationPath: "/out/agent.md", DestinationResolvedBy: "explicit"}
		}, "stackInEffect"},
		{"promote", func(string) []string {
			return []string{"promote", "--file", "/src/agent.md", "--category", "Cat", "--harness", "stub-harness"}
		}, func(s *spyService, _ string) {
			s.promoteResp = app.PromoteResult{}
		}, "stackInEffect"},
		{"transform", func(string) []string {
			return []string{"transform", "--harness", "claude-code", "--target-harness", "opencode", "--path", "/agents/agent.md"}
		}, func(s *spyService, _ string) {
			s.transformResp = app.TransformHarnessResult{SourceHarnessID: "claude-code", TargetHarnessID: "opencode"}
		}, "stackInEffect"},
		{"check-index", func(string) []string { return []string{"check-index"} }, func(s *spyService, _ string) {
			s.checkIndexResp = app.IndexCheckResult{CatalogRoot: "/catalogs/user", Clean: true}
		}, "stackInEffect"},
	}
}

// sieRun runs the subcommand in the given output format against a spy reporting active (a zero
// ActiveStack uses the shared no-op form) and returns the exit code and stdout.
// The caller passes the workspace so that a baseline run and the compared run echo the same path.
func sieRun(t *testing.T, c sieCommand, ws, format string, active *stackconfig.ActiveStack, extra ...string) (int, string) {
	t.Helper()
	return sieRunErr(t, c, ws, format, active, nil, extra...)
}

// sieRunErr is sieRun with a service error returned next to the summary of the deploy-like
// subcommands (a declined plan is ErrPlanNotConfirmed).
func sieRunErr(t *testing.T, c sieCommand, ws, format string, active *stackconfig.ActiveStack, svcErr error, extra ...string) (int, string) {
	t.Helper()
	spy := &spyService{}
	c.prepare(spy, ws)
	switch c.name {
	case "deploy":
		spy.deployErr = svcErr
	case "update":
		spy.updateErr = svcErr
	case "workflows":
		spy.workflowsErr = svcErr
	case "agents":
		spy.deployAgentsErr = svcErr
	case "hooks":
		spy.deployHooksErr = svcErr
	}
	var svc app.Service = spy
	if active != nil {
		svc = sieSpy{spyService: spy, active: *active}
	}
	args := append(c.args(ws), extra...)
	if format == "json" {
		args = append(args, "--output", "json")
	}
	var out bytes.Buffer
	code := cli.Run(context.Background(), args, svc, &out, &bytes.Buffer{})
	return code, out.String()
}

func TestStackInEffect_Human_ShowsLayersAndOriginBeforeTheResult(t *testing.T) {
	origins := []struct {
		origin stackconfig.Origin
		layers []domain.CatalogLayer
	}{
		{stackconfig.OriginStackInput, []domain.CatalogLayer{sieUserLayer, sieDefaultLayer}},
		{stackconfig.OriginPersisted, []domain.CatalogLayer{sieUserLayer, sieDefaultLayer}},
		{stackconfig.OriginStackInput, []domain.CatalogLayer{sieDefaultLayer}},
	}
	for _, c := range sieCommands() {
		for _, o := range origins {
			t.Run(c.name+"/"+string(o.origin)+"/"+strconv.Itoa(len(o.layers))+" layers", func(t *testing.T) {
				ws := t.TempDir()
				_, baseline := sieRun(t, c, ws, "human", nil)
				active := sieStack(o.origin, o.layers...)

				_, out := sieRun(t, c, ws, "human", &active)

				header := "Catalog stack (" + string(o.origin) + "):"
				at := strings.Index(out, header)
				if at < 0 {
					t.Fatalf("output lacks %q:\n%s", header, out)
				}
				last := at
				for i, l := range o.layers {
					line := strconv.Itoa(i+1) + ". " + l.Identity.ID + " " + l.Identity.Name + " " + l.Identity.Version + " " + l.Root
					idx := strings.Index(out, line)
					if idx < last {
						t.Errorf("layer line %q missing or out of order (index %d after %d):\n%s", line, idx, last, out)
						continue
					}
					last = idx
					rest := out[idx+len(line):]
					rest = rest[:strings.Index(rest+"\n", "\n")]
					if got := strings.Contains(rest, "(default)"); got != l.Default {
						t.Errorf("layer %q: default marker present = %v, want %v", l.Identity.ID, got, l.Default)
					}
				}
				if !strings.HasSuffix(out, baseline) || strings.Index(out, baseline) <= last {
					t.Errorf("the result does not follow the stack block unchanged\nwant suffix:\n%s\ngot:\n%s", baseline, out)
				}
			})
		}
	}
}

func TestStackInEffect_Human_DefaultOriginIsUnchanged(t *testing.T) {
	for _, c := range sieCommands() {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			_, baseline := sieRun(t, c, ws, "human", nil)
			active := sieStack(stackconfig.OriginDefault, sieDefaultLayer)

			_, out := sieRun(t, c, ws, "human", &active)

			if out != baseline {
				t.Errorf("output changed for the default origin:\nwant:\n%s\ngot:\n%s", baseline, out)
			}
			if strings.Contains(out, "Catalog stack") {
				t.Errorf("default origin printed a stack block:\n%s", out)
			}
		})
	}
}

func TestStackInEffect_Human_DryRunStillShowsTheBlock(t *testing.T) {
	for _, c := range sieCommands() {
		if c.name == "check-index" { // check-index has no --dry-run flag
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			active := sieStack(stackconfig.OriginPersisted, sieUserLayer, sieDefaultLayer)

			_, out := sieRun(t, c, t.TempDir(), "human", &active, "--dry-run")

			if !strings.Contains(out, "Catalog stack (persisted):") {
				t.Errorf("dry-run output lacks the stack block:\n%s", out)
			}
		})
	}
}

// TestStackInEffect_DeclinedPlan_StillShowsTheBlock verifies that when the service declines the
// plan (ErrPlanNotConfirmed returned with a summary) the human output still carries the stack
// block ahead of the summary, the JSON document still carries the stack key, and the exit code
// is the one the same declined plan gives without a stack block.
func TestStackInEffect_DeclinedPlan_StillShowsTheBlock(t *testing.T) {
	for _, c := range sieCommands() {
		if c.extraKey != "StackInEffect" {
			continue
		}
		active := sieStack(stackconfig.OriginPersisted, sieUserLayer, sieDefaultLayer)
		t.Run(c.name+"/human", func(t *testing.T) {
			ws := t.TempDir()
			plainCode, baseline := sieRunErr(t, c, ws, "human", nil, app.ErrPlanNotConfirmed)

			code, out := sieRunErr(t, c, ws, "human", &active, app.ErrPlanNotConfirmed)

			at := strings.Index(out, "Catalog stack (persisted):")
			if at < 0 {
				t.Fatalf("declined-plan output lacks the stack block:\n%s", out)
			}
			if !strings.Contains(out, "1. cat-user User Catalog 2.0.0 /catalogs/user") ||
				!strings.Contains(out, "2. mosaic-default MOSAIC Default 1.4.0 /mosaic/Catalog") {
				t.Errorf("declined-plan block lacks the layers:\n%s", out)
			}
			// A declined plan normally prints no summary on stdout (the error goes to stderr),
			// so the order check applies only when the plain run printed something.
			if strings.TrimSpace(baseline) != "" &&
				(!strings.HasSuffix(out, baseline) || strings.Index(out, baseline) <= at) {
				t.Errorf("the result does not follow the stack block unchanged\nwant suffix:\n%s\ngot:\n%s", baseline, out)
			}
			if code != plainCode {
				t.Errorf("exit code = %d with the block, %d without: the block must not change it", code, plainCode)
			}
		})
		t.Run(c.name+"/json", func(t *testing.T) {
			ws := t.TempDir()
			plainCode, baseline := sieRunErr(t, c, ws, "json", nil, app.ErrPlanNotConfirmed)
			code, out := sieRunErr(t, c, ws, "json", &active, app.ErrPlanNotConfirmed)

			if strings.TrimSpace(baseline) == "" {
				// A declined plan prints no JSON document: it must stay "one document or none".
				if strings.TrimSpace(out) != "" {
					t.Errorf("declined plan printed output although the plain run printed none:\n%s", out)
				}
			} else if _, ok := decodeSieDoc(t, out)[c.extraKey]; !ok {
				t.Errorf("declined-plan document lacks the %q key:\n%s", c.extraKey, out)
			}
			if code != plainCode {
				t.Errorf("exit code = %d with the key, %d without", code, plainCode)
			}
		})
	}
}

func TestStackInEffect_JSON_AddsOneKeyForEveryOrigin(t *testing.T) {
	origins := []stackconfig.ActiveStack{
		sieStack(stackconfig.OriginStackInput, sieUserLayer, sieDefaultLayer),
		sieStack(stackconfig.OriginPersisted, sieUserLayer, sieDefaultLayer),
		sieStack(stackconfig.OriginDefault, sieDefaultLayer),
	}
	for _, c := range sieCommands() {
		for _, active := range origins {
			t.Run(c.name+"/"+string(active.Origin), func(t *testing.T) {
				ws := t.TempDir()
				_, baseline := sieRun(t, c, ws, "json", nil)
				want := decodeSieDoc(t, baseline)
				delete(want, c.extraKey)

				_, out := sieRun(t, c, ws, "json", &active)

				got := decodeSieDoc(t, out)
				if len(want) == 0 {
					t.Fatalf("baseline document has no keys:\n%s", baseline)
				}
				for k, v := range want {
					if gv, ok := got[k]; !ok || !reflect.DeepEqual(gv, v) {
						t.Errorf("existing key %q changed: got %s, want %s", k, gv, v)
					}
				}
				raw, ok := got[c.extraKey]
				if !ok {
					t.Fatalf("document lacks the %q key:\n%s", c.extraKey, out)
				}
				if len(got) != len(want)+1 {
					t.Errorf("document has %d keys, want the existing %d plus one", len(got), len(want))
				}
				var decoded stackconfig.ActiveStack
				if err := json.Unmarshal(raw, &decoded); err != nil {
					t.Fatalf("decode %q: %v", c.extraKey, err)
				}
				if !reflect.DeepEqual(decoded, active) {
					t.Errorf("%s = %+v, want %+v", c.extraKey, decoded, active)
				}
			})
		}
	}
}

func decodeSieDoc(t *testing.T, out string) map[string]json.RawMessage {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	return doc
}
