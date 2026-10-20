package stack_test

import (
	"errors"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/stack/stackerr"
)

// addTestAgentWithNumericID writes an agent of the given role under relDir declaring numeric id.
func addTestAgentWithNumericID(t *testing.T, root, relDir, key, role, numericID string) {
	t.Helper()
	fm := "---\nid: \"" + numericID + "\"\nversion: \"1.0.0\"\nname: " + key + "\ndescription: Agent.\nrole: " + role + "\n"
	if role == "subagent" {
		fm += "required_skills: []\n"
	}
	mustWrite(t, root, filepath.Join(relDir, key+".md"), fm+"---\nBody.\n")
}

func addTestWorkerWithID(t *testing.T, root, key, numericID string) {
	t.Helper()
	addTestAgentWithNumericID(t, root, filepath.Join("Subagents", "Cat"), key, "subagent", numericID)
}

func addTestStandaloneWithID(t *testing.T, root, key, numericID string) {
	t.Helper()
	addTestAgentWithNumericID(t, root, "StandaloneAgents", key, "standalone", numericID)
}

func asCollision(t *testing.T, err error) *stackerr.NumericIDCollisionError {
	t.Helper()
	var ce *stackerr.NumericIDCollisionError
	if !errors.As(err, &ce) {
		t.Fatalf("New error = %v, want *stackerr.NumericIDCollisionError", err)
	}
	if !errors.Is(err, stackerr.ErrNumericIDCollision) {
		t.Error("errors.Is(err, ErrNumericIDCollision) = false")
	}
	return ce
}

func TestNew_WorkersWithSameNumericIDInDifferentLayers_FailsNamingIdKeysAndCatalogs(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkerWithID(t, high, "alpha", "77")
	addTestWorkerWithID(t, low, "beta", "77")

	ce := asCollision(t, composeRoots(mosaic, high, low))

	if ce.NumericID != "77" || ce.Keys != [2]string{"alpha", "beta"} {
		t.Errorf("collision = id %q keys %v, want 77 [alpha beta]", ce.NumericID, ce.Keys)
	}
	if ce.Catalogs[0].ID != "high" || ce.Catalogs[1].ID != "low" {
		t.Errorf("Catalogs = %v, want [high low] matching Keys", ce.Catalogs)
	}
}

func TestNew_StandaloneCollidesWithWorkerInOtherLayer_Fails(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestStandaloneWithID(t, high, "solo", "55")
	addTestWorkerWithID(t, low, "worker", "55")

	ce := asCollision(t, composeRoots(mosaic, high, low))

	if ce.Keys != [2]string{"solo", "worker"} {
		t.Errorf("Keys = %v, want [solo worker]", ce.Keys)
	}
	if ce.Catalogs[0].ID != "high" || ce.Catalogs[1].ID != "low" {
		t.Errorf("Catalogs = %v, want [high low]", ce.Catalogs)
	}
}

func TestNew_TwoStandalonesWithSameNumericIDInDifferentLayers_Fails(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestStandaloneWithID(t, high, "one", "56")
	addTestStandaloneWithID(t, low, "two", "56")

	asCollision(t, composeRoots(mosaic, high, low))
}

func TestNew_ScriptOrchestratorCollidesWithWorker_Fails(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestrator(t, high, "1.0.0")
	addTestAgentWithNumericID(t, high, "Orchestrator", "orchestrator-script", "orchestrator", "88")
	addTestWorkerWithID(t, low, "worker", "88")

	ce := asCollision(t, composeRoots(mosaic, high, low))

	if ce.Keys != [2]string{"orchestrator-script", "worker"} {
		t.Errorf("Keys = %v, want [orchestrator-script worker]", ce.Keys)
	}
	if ce.Catalogs[0].ID != "high" || ce.Catalogs[1].ID != "low" {
		t.Errorf("Catalogs = %v, want [high low]", ce.Catalogs)
	}
}

func TestNew_CollisionWithinOneLayer_FailsAsCompositionError(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "only")
	addTestStandaloneWithID(t, root, "solo", "66")
	addTestWorkerWithID(t, root, "worker", "66")

	err := composeRoots(mosaic, root)

	ce := asCollision(t, err)
	if ce.Catalogs[0].ID != "only" || ce.Catalogs[1].ID != "only" {
		t.Errorf("Catalogs = %v, want both naming the layer", ce.Catalogs)
	}
	var le *stackerr.LayerError
	if errors.As(err, &le) {
		t.Errorf("collision surfaced as a layer error (%v), want one composition error", le)
	}
	assertReportedOnce(t, err)
}

// assertReportedOnce fails when err is a joined error carrying more than one error.
func assertReportedOnce(t *testing.T, err error) {
	t.Helper()
	if joined, ok := err.(interface{ Unwrap() []error }); ok && len(joined.Unwrap()) != 1 {
		t.Errorf("error carries %d joined errors, want the collision reported once: %v", len(joined.Unwrap()), err)
	}
}

func TestNew_WorkerCollisionWithinOneLayer_ReportedOnceAsCollision(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "only")
	addTestWorkerWithID(t, root, "a", "12")
	addTestWorkerWithID(t, root, "b", "12")

	err := composeRoots(mosaic, root)
	ce := asCollision(t, err)

	if ce.Keys != [2]string{"a", "b"} {
		t.Errorf("Keys = %v, want [a b]", ce.Keys)
	}
	assertReportedOnce(t, err)
}

func TestNew_SameKeyShadowWithSameNumericID_NotACollision(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkerWithID(t, high, "shared", "21")
	addTestWorkerWithID(t, low, "shared", "21")
	addTestStandaloneWithID(t, high, "solo", "22")
	addTestStandaloneWithID(t, low, "solo", "22")

	if err := composeRoots(mosaic, high, low); err != nil {
		t.Fatalf("same-key shadows must not collide: %v", err)
	}
}

func TestNew_ShadowedAgentIDCollidingWithOtherWinner_NotACollision(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkerWithID(t, high, "x", "31")
	addTestWorkerWithID(t, high, "y", "30")
	addTestWorkerWithID(t, low, "x", "30") // shadowed by high/x, so its id 30 is not in the resolved set

	if err := composeRoots(mosaic, high, low); err != nil {
		t.Fatalf("a shadowed agent's id must not count: %v", err)
	}
}

func TestNew_MultipleCollisions_ReportsSmallestIDAsStringWithTwoSmallestKeys(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkerWithID(t, high, "d", "3")
	addTestWorkerWithID(t, low, "e", "3")
	addTestWorkerWithID(t, high, "z", "10")
	addTestWorkerWithID(t, high, "m", "10")
	addTestWorkerWithID(t, low, "k", "10")

	ce := asCollision(t, composeRoots(mosaic, high, low))

	if ce.NumericID != "10" {
		t.Errorf("NumericID = %q, want 10 (smallest as a string)", ce.NumericID)
	}
	if ce.Keys != [2]string{"k", "m"} {
		t.Errorf("Keys = %v, want the two smallest keys [k m]", ce.Keys)
	}
	if ce.Catalogs[0].ID != "low" || ce.Catalogs[1].ID != "high" {
		t.Errorf("Catalogs = %v, want [low high] (supplier of k, supplier of m)", ce.Catalogs)
	}
}

func TestNew_AgentsWithoutNumericIDOrDistinctIDs_Accepted(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestStandalone(t, high, "plain-a", "1.0.0")
	addTestStandalone(t, low, "plain-b", "1.0.0")
	addTestWorkerWithID(t, high, "w1", "1")
	addTestWorkerWithID(t, low, "w2", "2")

	if err := composeRoots(mosaic, high, low); err != nil {
		t.Fatalf("no collision expected: %v", err)
	}
}

func TestNew_NumericCollisionAndIncompletePair_ReportsPairFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestrator(t, high, "1.0.0")
	addTestWorkerWithID(t, high, "a", "9")
	addTestWorkerWithID(t, low, "b", "9")

	err := composeRoots(mosaic, high, low)

	if !errors.Is(err, stackerr.ErrOrchestratorPairIncomplete) {
		t.Fatalf("New error = %v, want the pair error before the numeric-id check", err)
	}
}
