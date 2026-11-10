package app_test

// rebuild_model_test.go covers model handling of the deployed-agent rebuild: a resolved model
// is written under the target's model key; an unresolved model omits the key entirely and
// yields the same no-model gap deploy produces. The source file's own model never carries over.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestRebuild_ResolvedModelWrittenUnderTargetModelKey(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, p, false))

			if got, ok := rbScalar(t, out, "model"); !ok || got != rbModelID {
				t.Errorf("model: want %q, got %q (present=%v)", rbModelID, got, ok)
			}
			if hasGap(res.Gaps, domain.GapNoModel) {
				t.Errorf("a resolved model must not produce a no-model gap: %+v", res.Gaps)
			}
		})
	}
}

func TestRebuild_UnresolvedModelOmitsModelKey(t *testing.T) {
	unresolved := withModelSelection(domain.ModelSelection{Origin: domain.OriginUnresolved})
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false, unresolved))

			if rbHasKey(t, out, "model") {
				t.Errorf("an unresolved model must omit the model key:\n%s", out)
			}
			if strings.Contains(out, "\nmodel:") {
				t.Errorf("no model line of any kind may remain in the frontmatter:\n%s", out)
			}
		})
	}
}

func TestRebuild_UnresolvedModelYieldsOneNoModelGap(t *testing.T) {
	unresolved := withModelSelection(domain.ModelSelection{Origin: domain.OriginUnresolved})
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, _ := rbRebuild(t, rbInput(t, p, false, unresolved))

			if n := countGaps(res.Gaps, domain.GapNoModel); n != 1 {
				t.Errorf("want exactly one no-model gap, got %d: %+v", n, res.Gaps)
			}
		})
	}
}

func TestRebuild_UnresolvedModelDoesNotKeepSourceModel(t *testing.T) {
	// The Claude Code fixture carries claude-sonnet-4-5 and the OpenCode one a provider-qualified
	// form; neither belongs to the target.
	unresolved := withModelSelection(domain.ModelSelection{Origin: domain.OriginUnresolved})
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false, unresolved))

			if strings.Contains(out, "claude-sonnet-4-5") {
				t.Errorf("the source model must not be carried into the target:\n%s", out)
			}
		})
	}
}

func countGaps(gaps []domain.Gap, kind domain.GapKind) int {
	n := 0
	for _, g := range gaps {
		if g.Kind == kind {
			n++
		}
	}
	return n
}

func hasGap(gaps []domain.Gap, kind domain.GapKind) bool { return countGaps(gaps, kind) > 0 }
