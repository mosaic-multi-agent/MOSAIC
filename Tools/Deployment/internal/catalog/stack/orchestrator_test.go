package stack_test

import (
	"path/filepath"
	"testing"
)

func TestStack_Orchestrator_HigherLayerPairReplacesBothOrchestrators(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestrator(t, high, "9.0.0")
	addTestOrchestratorScript(t, high, "9.0.0")
	addTestOrchestrator(t, low, "1.0.0")
	addTestOrchestratorScript(t, low, "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	if o := c.Orchestrator(); o.Version != "9.0.0" || !underRoot(o.SourcePath, high) {
		t.Errorf("Orchestrator() = version %s path %s, want the high layer's", o.Version, o.SourcePath)
	}
	s, ok := c.OrchestratorScript()
	if !ok || s.Version != "9.0.0" || !underRoot(s.SourcePath, high) {
		t.Errorf("OrchestratorScript() = %+v, %v, want the high layer's", s, ok)
	}
}

func TestStack_Orchestrator_FallsBackToLowerPairWhenHigherLayersSupplyNone(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, middle, bottom := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "middle"), newTestCatalogRoot(t, "bottom")
	addTestSubagent(t, top, "Cat", "worker", "1.0.0")
	addTestOrchestrator(t, bottom, "1.0.0")
	addTestOrchestratorScript(t, bottom, "1.0.0")

	c := newTestStack(t, mosaic, top, middle, bottom)

	if o := c.Orchestrator(); o.Version != "1.0.0" || !underRoot(o.SourcePath, bottom) {
		t.Errorf("Orchestrator() = version %s path %s, want the bottom layer's", o.Version, o.SourcePath)
	}
	if s, ok := c.OrchestratorScript(); !ok || !underRoot(s.SourcePath, bottom) {
		t.Errorf("OrchestratorScript() = %+v, %v, want the bottom layer's", s, ok)
	}
}

func TestStack_Orchestrator_MiddleLayerPairBeatsBottomLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, middle, bottom := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "middle"), newTestCatalogRoot(t, "bottom")
	addTestOrchestrator(t, middle, "5.0.0")
	addTestOrchestratorScript(t, middle, "5.0.0")
	addTestOrchestrator(t, bottom, "1.0.0")
	addTestOrchestratorScript(t, bottom, "1.0.0")

	c := newTestStack(t, mosaic, top, middle, bottom)

	o := c.Orchestrator()
	s, ok := c.OrchestratorScript()
	if !ok || !underRoot(o.SourcePath, middle) || !underRoot(s.SourcePath, middle) {
		t.Errorf("orchestrators came from %s and %s, want both from the middle layer", o.SourcePath, s.SourcePath)
	}
}

func TestStack_Orchestrator_BothAgentKeysResolveFromTheSameLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestrator(t, high, "9.0.0")
	addTestOrchestratorScript(t, high, "9.0.0")
	addTestOrchestrator(t, low, "1.0.0")
	addTestOrchestratorScript(t, low, "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		a, ok := c.Agent(key)
		if !ok || !underRoot(a.SourcePath, high) {
			t.Errorf("Agent(%q) = %+v, %v; want the high layer's", key, a, ok)
		}
	}
}

func TestStack_Orchestrator_NoLayerSuppliesOrchestratorComposesWithNone(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	if o := c.Orchestrator(); o.Key != "" {
		t.Errorf("Orchestrator() = %+v, want the zero agent", o)
	}
	if _, ok := c.OrchestratorScript(); ok {
		t.Error("OrchestratorScript() reported present, want absent")
	}
}

func TestStack_ReadSource_RefusesShadowedOrchestratorPair(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestrator(t, high, "9.0.0")
	addTestOrchestratorScript(t, high, "9.0.0")
	addTestOrchestrator(t, low, "1.0.0")
	addTestOrchestratorScript(t, low, "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	for _, name := range []string{"orchestrator.md", "orchestrator-script.md"} {
		if _, err := c.ReadSource(filepath.Join(high, "Orchestrator", name)); err != nil {
			t.Errorf("ReadSource of the winner's %s: %v", name, err)
		}
		if _, err := c.ReadSource(filepath.Join(low, "Orchestrator", name)); err == nil {
			t.Errorf("ReadSource served the shadowed lower-layer %s", name)
		}
	}
}
