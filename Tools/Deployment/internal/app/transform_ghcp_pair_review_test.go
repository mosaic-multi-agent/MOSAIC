package app_test

// transform_ghcp_pair_review_test.go specifies the plan the user reviews before a real transform
// between the two GHCP targets: it lists every removal and replacement the run carries out, and
// declining it writes, removes and backs up nothing.

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trPlanPaths returns the sorted paths of removals or replacements.
func trPlanPaths[T domain.Removal | domain.Replacement](list []T) []string {
	var out []string
	for _, e := range list {
		switch v := any(e).(type) {
		case domain.Removal:
			out = append(out, v.Path)
		case domain.Replacement:
			out = append(out, v.Path)
		}
	}
	sort.Strings(out)
	return out
}

// trChangePaths returns the sorted paths of a result change list.
func trChangePaths(list []app.TransformFileChange) []string {
	var out []string
	for _, c := range list {
		out = append(out, c.Path)
	}
	sort.Strings(out)
	return out
}

// trSamePaths reports whether two sorted path lists are equal.
func trSamePaths(a, b []string) bool { return strings.Join(a, "\n") == strings.Join(b, "\n") }

func TestTransformGhcpPair_ReviewPlanListsEveryRemovalAndReplacement(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		hooks := r.hooksDir(r.src)
		wantRemovals := []string{hooks + "/" + trPairSourceOnlyFile}
		wantReplacements := []string{hooks + "/logger.sh", trPairRegistrationFile}
		if c.agentRel == trPairLegacyAgent {
			wantRemovals = append(wantRemovals, trPairLegacyAgent)
		}
		if c.agentRel == trPairInPlaceAgent || c.beside {
			wantReplacements = append(wantReplacements, trPairInPlaceAgent)
		}
		sort.Strings(wantRemovals)
		sort.Strings(wantReplacements)
		req := r.request()
		req.AutoConfirmPlan = false

		res := r.mustRun(req)

		if len(r.spy.reviews) != 1 {
			t.Fatalf("review shown %d times, want once", len(r.spy.reviews))
		}
		plan := r.spy.reviews[0]
		if got := trPlanPaths(plan.Removals); !trSamePaths(got, wantRemovals) {
			t.Errorf("review plan removals = %v, want %v", got, wantRemovals)
		}
		if got := trPlanPaths(plan.Replacements); !trSamePaths(got, wantReplacements) {
			t.Errorf("review plan replacements = %v, want %v", got, wantReplacements)
		}
		if got := trChangePaths(res.Removals); !trSamePaths(got, wantRemovals) {
			t.Errorf("executed removals = %v, want the reviewed %v", got, wantRemovals)
		}
		if got := trChangePaths(res.Replacements); !trSamePaths(got, wantReplacements) {
			t.Errorf("executed replacements = %v, want the reviewed %v", got, wantReplacements)
		}
	})
}

func TestTransformGhcpPair_ReviewPlanGivesEveryRemovalAndReplacementAReason(t *testing.T) {
	r := trPairExecRun(t, hCLI, hVSCode, trPairExecCases[2])
	req := r.request()
	req.AutoConfirmPlan = false

	r.mustRun(req)

	plan := r.spy.reviews[0]
	if len(plan.Removals) == 0 || len(plan.Replacements) == 0 {
		t.Fatalf("review plan removals %d / replacements %d, want both non-empty", len(plan.Removals), len(plan.Replacements))
	}
	for _, rm := range plan.Removals {
		if rm.Reason == "" || strings.ContainsAny(rm.Reason, "\r\n") {
			t.Errorf("removal %s reason %q, want one non-empty line", rm.Path, rm.Reason)
		}
	}
	for _, rp := range plan.Replacements {
		if rp.Reason == "" || strings.ContainsAny(rp.Reason, "\r\n") {
			t.Errorf("replacement %s reason %q, want one non-empty line", rp.Path, rp.Reason)
		}
	}
}

func TestTransformGhcpPair_ReviewPlanDropsTheManifestEntryOfARemovedAgentOnly(t *testing.T) {
	r := trPairExecRun(t, hVSCode, hCLI, trPairExecCases[1])
	req := r.request()
	req.AutoConfirmPlan = false

	r.mustRun(req)

	removals := map[string]domain.Removal{}
	for _, rm := range r.spy.reviews[0].Removals {
		removals[rm.Path] = rm
	}
	agent, ok := removals[trPairLegacyAgent]
	if !ok {
		t.Fatalf("review plan removals %v, want the legacy agent %s listed", trPlanPaths(r.spy.reviews[0].Removals), trPairLegacyAgent)
	}
	if agent.Ref.Kind != domain.ArtifactAgent || !agent.DropEntry {
		t.Errorf("agent removal kind %s DropEntry %v, want an agent that drops its manifest entry", agent.Ref.Kind, agent.DropEntry)
	}
	hookFile, ok := removals[r.hooksDir(r.src)+"/"+trPairSourceOnlyFile]
	if !ok {
		t.Fatalf("review plan removals %v, want the source-only hook file listed", trPlanPaths(r.spy.reviews[0].Removals))
	}
	if hookFile.Ref.Kind != domain.ArtifactHook || hookFile.DropEntry {
		t.Errorf("hook file removal kind %s DropEntry %v, want a hook that keeps its manifest entry", hookFile.Ref.Kind, hookFile.DropEntry)
	}
}

func TestTransformGhcpPair_DecliningTheReviewChangesNothingAndBacksNothingUp(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		script := interactiontest.NewBuilder().AnswerReview(false).Build()
		r.spy.Stub = script
		before := r.snapshot()
		req := r.request()
		req.AutoConfirmPlan = false

		_, err := r.run(req)

		if !errors.Is(err, app.ErrPlanNotConfirmed) {
			t.Fatalf("error = %v, want ErrPlanNotConfirmed", err)
		}
		if len(r.spy.reviews) != 1 || len(r.spy.reviews[0].Removals)+len(r.spy.reviews[0].Replacements) == 0 {
			t.Fatalf("reviews = %+v, want one review listing removals and replacements", r.spy.reviews)
		}
		if diff := trSameTree(before, r.snapshot()); diff != "" {
			t.Errorf("declined run changed the workspace (backups and manifest included): %s", diff)
		}
	})
}
