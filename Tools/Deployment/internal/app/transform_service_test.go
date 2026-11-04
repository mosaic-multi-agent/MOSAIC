package app_test

// transform_service_test.go covers the run-level questions of Service.TransformHarness that do
// not depend on the workspace content: the target-harness question is asked only when
// TargetHarnessID is empty, a cancelled answer returns an error and writes nothing, the same
// source and target harness is rejected, and an unresolvable harness is a whole-run error.
// The whole-deployment behaviour is covered by the transform_*_test.go files that drive the
// service over a temporary workspace.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// ---------------------------------------------------------------------------
// Transform test fixtures
// ---------------------------------------------------------------------------

// transformSourceHarness is the harness the input files were deployed for.
var transformSourceHarness = domain.HarnessRef{
	ID:          "source-harness",
	DisplayName: "Source Harness",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// transformTargetHarness is the harness to transform files into.
var transformTargetHarness = domain.HarnessRef{
	ID:          "target-harness",
	DisplayName: "Target Harness",
	Tier:        domain.TierBuiltin,
	Usable:      true,
}

// newTransformSourceModule returns a stubHarnessModule for the source harness with a
// model key that distinguishes it from the target harness.
func newTransformSourceModule() *stubHarnessModule {
	return &stubHarnessModule{
		ref: transformSourceHarness,
		descriptor: domain.HarnessDescriptor{
			ID:          "source-harness",
			DisplayName: "Source Harness",
			Frontmatter: domain.FrontmatterSpec{
				ModelKey: "src_model",
			},
			Extensions: map[domain.ArtifactKind]string{
				domain.ArtifactAgent: ".src.md",
			},
		},
	}
}

// newTransformTargetModule returns a stubHarnessModule for the target harness. Its model
// key ("tgt_model") is unknown to the source harness, yielding HarnessMatchNo for source
// files that carry it. Paths.Agents is configured so ResolveTargetPath can compute a
// non-empty destination.
func newTransformTargetModule() *stubHarnessModule {
	return &stubHarnessModule{
		ref: transformTargetHarness,
		descriptor: domain.HarnessDescriptor{
			ID:          "target-harness",
			DisplayName: "Target Harness",
			Frontmatter: domain.FrontmatterSpec{
				ModelKey: "tgt_model",
			},
			Paths: domain.PathSpec{
				Agents: domain.ScopedPaths{
					Supported: true,
					Project:   "agents",
				},
			},
			Extensions: map[domain.ArtifactKind]string{
				domain.ArtifactAgent: ".tgt.md",
			},
		},
	}
}

// newTransformRegistry returns a stubRegistry backed by both transform harness modules.
func newTransformRegistry() *stubRegistry {
	srcMod := newTransformSourceModule()
	tgtMod := newTransformTargetModule()
	return &stubRegistry{
		list: []domain.HarnessRef{transformSourceHarness, transformTargetHarness},
		modules: map[string]domain.HarnessModule{
			"source-harness": srcMod,
			"target-harness": tgtMod,
		},
	}
}

// newTransformDeps returns an app.Deps wired for transform tests — same as newBaseDeps
// but with a two-harness registry instead of the minimal single-harness one.
func newTransformDeps(t *testing.T, stub *interactiontest.Stub) (app.Deps, string) {
	t.Helper()
	deps, workspace := newBaseDeps(t, stub)
	deps.Registry = newTransformRegistry()
	return deps, workspace
}

// matchingAgentBytes returns the raw content of an agent file that belongs to the source
// harness. The file carries transform_version (eligibility signal 1), canonical boundary
// tags (eligibility signal 2), and the source harness's model key in frontmatter (positive
// identification evidence for the source harness field classifier).
func matchingAgentBytes() []byte {
	return []byte("---\n" +
		"transform_version: \"2.1.0\"\n" +
		"injections_version: \"3.0.0\"\n" +
		"src_model: claude-3-sonnet\n" +
		"---\n" +
		"<Identity type=\"core\">\n" +
		"You are the Source Harness Agent.\n" +
		"</Identity>\n")
}

// writeTransformFile writes content to a file named name inside dir and returns its path.
func writeTransformFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("writeTransformFile mkdir: %v", err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("writeTransformFile: %v", err)
	}
	return p
}

// newPreAnsweredTransformRequest returns a TransformHarnessRequest with all interactive
// fields pre-answered, so no questions are asked through the Interaction port during the
// service call.
func newPreAnsweredTransformRequest(workspace string) app.TransformHarnessRequest {
	return app.TransformHarnessRequest{
		SourceHarnessID: transformSourceHarness.ID,
		TargetHarnessID: transformTargetHarness.ID,
		Workspace:       workspace,
		TargetModel:     "target-model-x",
		Overwrite:       false,
		DryRun:          false,
	}
}

// ---------------------------------------------------------------------------
// Interactive question routing
// ---------------------------------------------------------------------------

// TestTransformHarness_EmptyTargetHarness_AsksQTransformTargetHarness verifies that when
// TargetHarnessID is empty, the service asks QTransformTargetHarness through the
// Interaction port with an option for the target harness.
func TestTransformHarness_EmptyTargetHarness_AsksQTransformTargetHarness(t *testing.T) {
	// Arrange: pre-answer QTransformTargetHarness with the target harness ID.
	stub := interactiontest.NewBuilder().
		AnswerSelectOne(domain.QTransformTargetHarness, "", transformTargetHarness.ID).
		Build()
	deps, _ := newTransformDeps(t, stub)
	svc := app.New(deps)

	dir := t.TempDir()
	agentPath := writeTransformFile(t, dir, "my-agent.src.md", matchingAgentBytes())

	req := newPreAnsweredTransformRequest(agentPath)
	req.TargetHarnessID = "" // unset → must trigger question

	// Act
	_, _ = svc.TransformHarness(context.Background(), req)

	// Assert: QTransformTargetHarness was asked.
	if !containsQuestion(stub.QuestionOrder(), domain.QTransformTargetHarness) {
		t.Errorf("QTransformTargetHarness was not asked; question order: %v; "+
			"want QTransformTargetHarness asked when TargetHarnessID is empty", stub.QuestionOrder())
	}
}

// TestTransformHarness_PreAnsweredTargetHarness_SkipsQTransformTargetHarness verifies that
// when TargetHarnessID is non-empty, QTransformTargetHarness is NOT asked.
//
// NOT RED-VERIFIED: this test asserts a purely negative condition ("question was not asked").
// Before I6.3, the stub returns errTransformNotImplemented and never calls the Interaction
// port — so no questions are ever asked, making "not asked" trivially true. The test passes
// in the RED phase for the wrong reason. The implementer must verify manually during I6.3 that:
//   - TestTransformHarness_EmptyTargetHarness_AsksQTransformTargetHarness correctly fails in RED
//     (that test does fail correctly today)
//   - This test remains GREEN after I6.3 because a pre-answered TargetHarnessID skips the question
func TestTransformHarness_PreAnsweredTargetHarness_SkipsQTransformTargetHarness(t *testing.T) {
	// Arrange: no answer wired for QTransformTargetHarness — the stub must not be asked.
	stub := interactiontest.NewBuilder().Build()
	deps, _ := newTransformDeps(t, stub)
	svc := app.New(deps)

	dir := t.TempDir()
	agentPath := writeTransformFile(t, dir, "my-agent.src.md", matchingAgentBytes())

	req := newPreAnsweredTransformRequest(agentPath)
	// TargetHarnessID is already set by newPreAnsweredTransformRequest.

	// Act
	_, _ = svc.TransformHarness(context.Background(), req)

	// Assert: QTransformTargetHarness was not asked.
	if containsQuestion(stub.QuestionOrder(), domain.QTransformTargetHarness) {
		t.Errorf("QTransformTargetHarness was asked despite TargetHarnessID being pre-answered; "+
			"question order: %v", stub.QuestionOrder())
	}
}

// TestTransformHarness_CancelledTargetHarnessQuestion_ReturnsError verifies that
// cancellation of QTransformTargetHarness causes TransformHarness to return a non-nil
// error and write no files.
//
// NOT RED-VERIFIED (partially): The assertion "err != nil" and "zero counts" pass today for
// the wrong reason — the stub returns errTransformNotImplemented (err != nil, zero result).
// The "QTransformTargetHarness was asked" assertion below DOES fail correctly today (stub
// never calls Interaction), providing genuine RED-phase protection for the question-routing
// path. Once I6.3 is delivered and the stub is replaced, ALL three assertions must hold:
//   - QTransformTargetHarness IS asked (service calls Interaction with empty TargetHarnessID)
//   - err != nil (cancellation causes whole-run abort)
//   - zero file counts (aborted run produces no per-file outcomes)
func TestTransformHarness_CancelledTargetHarnessQuestion_ReturnsError(t *testing.T) {
	// Arrange: interaction stub configured to cancel QTransformTargetHarness.
	stub := interactiontest.NewBuilder().
		AnswerCancelledChoice(domain.QTransformTargetHarness, "").
		Build()
	deps, _ := newTransformDeps(t, stub)
	svc := app.New(deps)

	dir := t.TempDir()
	agentPath := writeTransformFile(t, dir, "my-agent.src.md", matchingAgentBytes())

	req := newPreAnsweredTransformRequest(agentPath)
	req.TargetHarnessID = "" // unset to trigger the question

	// Act
	result, err := svc.TransformHarness(context.Background(), req)

	// Assert: QTransformTargetHarness must have been asked before it could be cancelled.
	// This assertion fails correctly today (stub implementation never calls Interaction port).
	// Once I6.3 is delivered, the service will ask QTransformTargetHarness and the stub will
	// return Cancelled, satisfying this assertion.
	if !containsQuestion(stub.QuestionOrder(), domain.QTransformTargetHarness) {
		t.Error("QTransformTargetHarness was not asked; the service must invoke the Interaction port " +
			"when TargetHarnessID is empty — cancellation can only occur after the question is asked")
	}

	// Assert: error returned.
	if err == nil {
		t.Error("TransformHarness returned nil error after QTransformTargetHarness cancelled; want non-nil error")
	}

	// Assert: no file outcomes when the whole run is aborted.
	if result.Transformed > 0 || result.Failed > 0 {
		t.Errorf("result has outcomes (transformed=%d failed=%d) after cancellation; want zero counts",
			result.Transformed, result.Failed)
	}
}

// TestTransformHarness_SameSourceAndTargetHarness_ReturnsError verifies that requesting a
// transform where source and target harness are the same returns a whole-run error wrapping
// ErrTransformSameHarness, without calling any interactive questions.
func TestTransformHarness_SameSourceAndTargetHarness_ReturnsError(t *testing.T) {
	// Arrange
	stub := interactiontest.NewBuilder().Build()
	deps, _ := newTransformDeps(t, stub)
	svc := app.New(deps)

	dir := t.TempDir()
	agentPath := writeTransformFile(t, dir, "my-agent.src.md", matchingAgentBytes())

	req := newPreAnsweredTransformRequest(agentPath)
	req.TargetHarnessID = req.SourceHarnessID // same → must be rejected

	// Act
	_, err := svc.TransformHarness(context.Background(), req)

	// Assert: error wrapping ErrTransformSameHarness.
	if err == nil {
		t.Fatal("TransformHarness returned nil error when source == target harness; want error wrapping ErrTransformSameHarness")
	}
	if !errors.Is(err, app.ErrTransformSameHarness) {
		t.Errorf("error = %v; want errors.Is(err, ErrTransformSameHarness) = true", err)
	}

	// No interactive questions should have been asked.
	if len(stub.QuestionOrder()) != 0 {
		t.Errorf("questions asked: %v; want none when same-harness is detected up front", stub.QuestionOrder())
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// containsQuestion reports whether questions contains id.
func containsQuestion(questions []domain.QuestionID, id domain.QuestionID) bool {
	for _, q := range questions {
		if q == id {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Whole-run error cases
// ---------------------------------------------------------------------------

// TestTransformHarness_UnresolvableTargetHarness_ReturnsWholeRunError verifies that when the
// target harness ID cannot be resolved by the registry (harness not found), TransformHarness
// returns a whole-run error identifying the unresolvable harness. No file outcomes are
// produced because the run cannot proceed without a target harness module.
func TestTransformHarness_UnresolvableTargetHarness_ReturnsWholeRunError(t *testing.T) {
	// Arrange: registry has source harness but not the target harness.
	stub := interactiontest.NewBuilder().Build()
	deps, _ := newTransformDeps(t, stub)
	deps.Registry = &stubRegistry{
		list: []domain.HarnessRef{transformSourceHarness},
		modules: map[string]domain.HarnessModule{
			"source-harness": newTransformSourceModule(),
			// "target-harness" deliberately absent → Resolve("target-harness") errors.
		},
	}
	svc := app.New(deps)

	dir := t.TempDir()
	agentPath := writeTransformFile(t, dir, "my-agent.src.md", matchingAgentBytes())
	req := newPreAnsweredTransformRequest(agentPath)
	// TargetHarnessID is "target-harness" — not in the registry above.

	// Act
	_, err := svc.TransformHarness(context.Background(), req)

	// Assert: whole-run error must be returned.
	if err == nil {
		t.Fatal("TransformHarness returned nil error for unresolvable target harness; want a whole-run error")
	}
	// The error must mention the unresolvable harness ID so the user can identify the problem.
	// (errTransformNotImplemented does NOT mention "target-harness", so this assertion correctly
	// fails in the RED phase before I6.1 replaces the stub.)
	if !strings.Contains(err.Error(), "target-harness") {
		t.Errorf("error = %q; want it to identify the unresolvable harness ID 'target-harness'", err.Error())
	}
}

// TestTransformHarness_UnresolvableSourceHarness_ReturnsWholeRunError verifies that when the
// source harness ID cannot be resolved by the registry, TransformHarness returns a whole-run
// error identifying the unresolvable harness.
func TestTransformHarness_UnresolvableSourceHarness_ReturnsWholeRunError(t *testing.T) {
	// Arrange: registry has target harness but not the source harness.
	stub := interactiontest.NewBuilder().Build()
	deps, _ := newTransformDeps(t, stub)
	deps.Registry = &stubRegistry{
		list: []domain.HarnessRef{transformTargetHarness},
		modules: map[string]domain.HarnessModule{
			"target-harness": newTransformTargetModule(),
			// "source-harness" deliberately absent → Resolve("source-harness") errors.
		},
	}
	svc := app.New(deps)

	dir := t.TempDir()
	agentPath := writeTransformFile(t, dir, "my-agent.src.md", matchingAgentBytes())
	req := newPreAnsweredTransformRequest(agentPath)
	// SourceHarnessID is "source-harness" — not in the registry above.

	// Act
	_, err := svc.TransformHarness(context.Background(), req)

	// Assert: whole-run error must be returned.
	if err == nil {
		t.Fatal("TransformHarness returned nil error for unresolvable source harness; want a whole-run error")
	}
	// The error must mention the unresolvable harness ID.
	// (errTransformNotImplemented does NOT mention "source-harness", so this assertion correctly
	// fails in the RED phase before I6.1 replaces the stub.)
	if !strings.Contains(err.Error(), "source-harness") {
		t.Errorf("error = %q; want it to identify the unresolvable harness ID 'source-harness'", err.Error())
	}
}
