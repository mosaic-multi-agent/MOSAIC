package app_test

// promote_replace_test.go covers promote with Overwrite replacing an existing agent with the
// same key in the target layer: the new file takes the replaced agent's numeric id, sits under
// the chosen category, the old file is removed when the category differs, so the layer holds
// exactly one agent with that key, and no other layer is touched.

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
)

// replaceCase is one replacement: the category the existing agent lives in and the category
// the promoted file is written to.
type replaceCase struct {
	name     string
	existing string
	chosen   string
}

var replaceCases = []replaceCase{
	{"same category", "Audit", "Audit"},
	{"other category", "Audit", "Build"},
	{"subagent to utility", "Audit", app.PromoteCategoryUtility},
	{"utility to subagent", app.PromoteCategoryUtility, "Audit"},
}

// TestPromote_Overwrite_ReplacesInPlace_KeepsNumericIDAndSingleAgent verifies that replacing an
// existing key leaves exactly one agent with it in the layer, in the chosen category, with the
// replaced agent's numeric id (not max+1), that only that layer changes and that the source
// file is untouched.
func TestPromote_Overwrite_ReplacesInPlace_KeepsNumericIDAndSingleAgent(t *testing.T) {
	for _, tc := range replaceCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange: the highest id of the stack (400) is not the replaced agent's id (301).
			ls := newPlLayerSet(t, "top", "low")
			ls.addAgent(t, "top", "Other", "unrelated-agent", "400")
			ls.addAgentIn(t, "low", tc.existing, plAgentKey, "301")
			svc := app.New(ls.deps(t, newPlInteraction("")))
			src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
			srcBefore := readBytes(t, src)
			before := ls.snapshotLayers(t)

			// Act
			result, err := svc.Promote(context.Background(), replaceRequest(src, tc.chosen, true))

			// Assert
			if err != nil {
				t.Fatalf("Promote: %v", err)
			}
			want := ls.agentPath("low", tc.chosen, plAgentKey)
			if got := keyFiles(t, ls.roots["low"], plAgentKey); !reflect.DeepEqual(got, []string{want}) {
				t.Errorf("layer holds %v for the key, want exactly [%s]", got, want)
			}
			if result.DestinationPath != want {
				t.Errorf("DestinationPath = %q, want %q", result.DestinationPath, want)
			}
			if result.NumericID != "301" {
				t.Errorf("result NumericID = %q, want the replaced agent's 301", result.NumericID)
			}
			if id, _ := parseFrontmatterFieldSvc(t, want, "id"); id != "301" {
				t.Errorf("written id = %q, want the replaced agent's 301", id)
			}
			if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"low"}) {
				t.Errorf("changed layers = %v, want only [low]", got)
			}
			if !bytes.Equal(srcBefore, readBytes(t, src)) {
				t.Error("the source file was modified")
			}
		})
	}
}

// TestPromote_Overwrite_KeyResolvedFromTopLayer_ReplacesThereAndKeepsShadowedCopy verifies that
// when the stack resolves the key from the top layer, that agent is the one replaced (with its
// id) and the lower layer's shadowed copy is left byte-identical.
func TestPromote_Overwrite_KeyResolvedFromTopLayer_ReplacesThereAndKeepsShadowedCopy(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "top", "Audit", plAgentKey, "301")
	ls.addAgent(t, "low", "Audit", plAgentKey, "250")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)

	// Act
	result, err := svc.Promote(context.Background(), replaceRequest(src, "Build", true))

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	want := ls.agentPath("top", "Build", plAgentKey)
	if got := keyFiles(t, ls.roots["top"], plAgentKey); !reflect.DeepEqual(got, []string{want}) {
		t.Errorf("top layer holds %v for the key, want exactly [%s]", got, want)
	}
	if result.NumericID != "301" {
		t.Errorf("NumericID = %q, want the replaced top-layer agent's 301", result.NumericID)
	}
	if got := ls.changedLayers(t, before); !reflect.DeepEqual(got, []string{"top"}) {
		t.Errorf("changed layers = %v, want only [top]", got)
	}
}

// TestPromote_Overwrite_DryRun_ReportsReplacedIDAndChangesNothing verifies a dry run reports
// the numeric id and destination a real replacement would use and writes or removes nothing.
func TestPromote_Overwrite_DryRun_ReportsReplacedIDAndChangesNothing(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "top", "low")
	ls.addAgent(t, "top", "Other", "unrelated-agent", "400")
	ls.addAgent(t, "low", "Audit", plAgentKey, "301")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	before := ls.snapshotLayers(t)
	req := replaceRequest(src, "Build", true)
	req.DryRun = true

	// Act
	result, err := svc.Promote(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if result.NumericID != "301" {
		t.Errorf("NumericID = %q, want the replaced agent's 301", result.NumericID)
	}
	if want := ls.agentPath("low", "Build", plAgentKey); result.DestinationPath != want {
		t.Errorf("DestinationPath = %q, want %q", result.DestinationPath, want)
	}
	if got := ls.changedLayers(t, before); len(got) != 0 {
		t.Errorf("a dry run changed layers %v", got)
	}
}

// TestPromote_Overwrite_NoExistingAgent_AssignsNextID verifies Overwrite on a key that does not
// exist behaves like a new promote: into a user layer the id is the larger of 1000 and one
// greater than the largest in the stack.
func TestPromote_Overwrite_NoExistingAgent_AssignsNextID(t *testing.T) {
	// Arrange
	ls := newPlLayerSet(t, "only")
	ls.addAgent(t, "only", "Other", "unrelated-agent", "12")
	svc := app.New(ls.deps(t, newPlInteraction("")))
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")

	// Act
	result, err := svc.Promote(context.Background(), replaceRequest(src, "Audit", true))

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if result.NumericID != "1000" {
		t.Errorf("NumericID = %q, want 1000 (floor for a non-default layer)", result.NumericID)
	}
	want := filepath.Join(ls.roots["only"], "Subagents", "Audit", plAgentKey+".md")
	if got := keyFiles(t, ls.roots["only"], plAgentKey); !reflect.DeepEqual(got, []string{want}) {
		t.Errorf("layer holds %v for the key, want exactly [%s]", got, want)
	}
}
