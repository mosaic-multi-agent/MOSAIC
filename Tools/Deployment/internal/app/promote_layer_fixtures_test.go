package app_test

// promote_layer_fixtures_test.go holds the fixtures shared by the tests of promote's target
// layer rules: real catalog layers on disk, a real stack installed as the run's catalog, a
// tree snapshot to prove which layers a promote touched, and an interaction wrapper that
// answers and captures the target-layer and category questions. Nothing here is a test.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// plLayerSet is a stack of real layers and the Deps that use it.
type plLayerSet struct {
	mosaicRoot string
	roots      map[string]string // catalog id -> root
	order      []string          // catalog ids, highest first
}

// newPlLayerSet creates a MOSAIC root plus one user layer per id (highest first). Each layer
// carries only an identity until agents are added.
func newPlLayerSet(t *testing.T, ids ...string) *plLayerSet {
	t.Helper()
	ls := &plLayerSet{
		mosaicRoot: newSysMosaicRoot(t, sysLayer{id: "plmosaicdefault"}),
		roots:      map[string]string{},
		order:      ids,
	}
	for _, id := range ids {
		ls.roots[id] = newSysUserLayer(t, sysLayer{id: id})
	}
	return ls
}

// addAgent writes a worker agent with the given numeric id into layer id.
func (ls *plLayerSet) addAgent(t *testing.T, id, category, key, numericID string) {
	t.Helper()
	body := "---\nid: \"" + numericID + "\"\nversion: \"1.0.0\"\nname: " + key + "\n" +
		"description: Layer agent.\nrole: subagent\nrequired_skills: []\n---\nBody.\n"
	writeSysFile(t, filepath.Join(ls.roots[id], "Subagents", category, key+".md"), body)
}

// deps installs the real stack (user layers in order) as the run's catalog.
func (ls *plLayerSet) deps(t *testing.T, ix domain.Interaction) app.Deps {
	t.Helper()
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().Build())
	deps.Interaction = ix
	layers := make([]stack.Layer, 0, len(ls.order))
	for _, id := range ls.order {
		layers = append(layers, stack.Layer{Root: ls.roots[id]})
	}
	st, err := stack.New(ls.mosaicRoot, layers)
	if err != nil {
		t.Fatalf("stack.New: %v", err)
	}
	deps.Catalog = st
	deps.MosaicRoot = ls.mosaicRoot
	return deps
}

// snapshotTree maps every file under root to a content hash, so two snapshots compare equal
// only when no file was added, removed or changed.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		out[p] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %q: %v", root, err)
	}
	return out
}

// snapshotLayers snapshots every layer root by catalog id.
func (ls *plLayerSet) snapshotLayers(t *testing.T) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for id, root := range ls.roots {
		out[id] = snapshotTree(t, root)
	}
	return out
}

// changedLayers returns the sorted ids of layers whose tree differs from before.
func (ls *plLayerSet) changedLayers(t *testing.T, before map[string]map[string]string) []string {
	t.Helper()
	var changed []string
	for id, root := range ls.roots {
		after := snapshotTree(t, root)
		if len(after) != len(before[id]) {
			changed = append(changed, id)
			continue
		}
		for p, h := range after {
			if before[id][p] != h {
				changed = append(changed, id)
				break
			}
		}
	}
	sort.Strings(changed)
	return changed
}

// plInteraction wraps the scripted stub: it answers QPromoteTargetCatalog with layerAnswer
// (skipped when empty), whatever the question subject, and records the options offered for
// QPromoteTargetCatalog and QPromoteCategory.
type plInteraction struct {
	*interactiontest.Stub
	layerAnswer     string
	layerCancelled  bool // answer the layer question with Cancelled instead
	layerAsked      int
	layerOptions    []domain.Option
	categoryOptions []domain.Option
}

func (p *plInteraction) SelectOne(ctx context.Context, q domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	switch q.ID {
	case domain.QPromoteTargetCatalog:
		p.layerAsked++
		p.layerOptions = append([]domain.Option(nil), q.Options...)
		if p.layerCancelled {
			return domain.ChoiceAnswer{Status: domain.Cancelled}, nil
		}
		if p.layerAnswer == "" {
			return domain.ChoiceAnswer{Status: domain.SkippedOne}, nil
		}
		return domain.ChoiceAnswer{Status: domain.Answered, OptionID: p.layerAnswer}, nil
	case domain.QPromoteCategory:
		p.categoryOptions = append([]domain.Option(nil), q.Options...)
	}
	return p.Stub.SelectOne(ctx, q)
}

func newPlInteraction(layerAnswer string) *plInteraction {
	return &plInteraction{Stub: interactiontest.NewBuilder().Build(), layerAnswer: layerAnswer}
}
