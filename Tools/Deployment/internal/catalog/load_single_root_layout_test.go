package catalog_test

// load_single_root_layout_test.go covers the two on-disk layouts a catalogue root can use for
// skills and hooks: the current location (Skills/, Hooks/) and the legacy location
// (Agents/Generic/Skills/, Agents/Generic/Hooks/). The current location is scanned first and
// wins on a key collision. These are regression tests for existing behaviour, guarding the
// single-root loading code against losing the legacy scan or the first-wins rule.

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog"
)

// writeLegacySkill writes a skill under catalogRoot/Agents/Generic/Skills/{key}/SKILL.md whose
// name carries the given label so the loaded entry reveals which location it came from.
func writeLegacySkill(t *testing.T, catalogRoot, key, label string) {
	t.Helper()
	content := "---\nname: " + key + " " + label + "\ndescription: Legacy skill " + key + ".\nversion: \"1.0.0\"\n---\n# Skill\n"
	mustMkdir(t, catalogRoot, "Agents", "Generic", "Skills", key)
	mustWriteFile(t, catalogRoot, filepath.Join("Agents", "Generic", "Skills", key, "SKILL.md"), []byte(content))
}

// writeHookBundleAt writes a hook bundle under catalogRoot/{parentRel}/{bundleID}/ with the
// given description so the loaded entry reveals which location it came from.
func writeHookBundleAt(t *testing.T, catalogRoot, parentRel, bundleID, description string) {
	t.Helper()
	yaml := "schema_version: \"1\"\nid: " + bundleID + "\nversion: 1.0.0\n" +
		"description: " + description + "\nplaceholder: false\n" +
		"variants:\n  test-harness:\n    supported: true\n    files:\n" +
		"      - source: test-hook.sh\n        target: test-hook.sh\n    registration: []\n"
	bundleRel := filepath.Join(parentRel, bundleID)
	mustMkdir(t, catalogRoot, bundleRel, "test-harness")
	mustWriteFile(t, catalogRoot, filepath.Join(bundleRel, "hook.yaml"), []byte(yaml))
	mustWriteFile(t, catalogRoot, filepath.Join(bundleRel, "test-harness", "test-hook.sh"), []byte("#!/bin/sh\n"))
}

func TestLoad_CustomRoot_LegacySkillPath_SkillLoaded(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeLegacySkill(t, custom, "old-skill", "legacy")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cat.Skill("old-skill"); !ok {
		t.Error("skill under the legacy Agents/Generic/Skills path was not loaded")
	}
}

func TestLoad_CustomRoot_LegacyHookPath_HookLoaded(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeHookBundleAt(t, custom, filepath.Join("Agents", "Generic", "Hooks"), "old-hook", "legacy")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := cat.Hook("old-hook"); !ok {
		t.Error("hook bundle under the legacy Agents/Generic/Hooks path was not loaded")
	}
}

func TestLoad_CustomRoot_SkillInBothLayouts_CurrentPathWins(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeSkillToCustomCatalogueRoot(t, custom, "dup-skill") // name "dup-skill custom"
	writeLegacySkill(t, custom, "dup-skill", "legacy")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	s, ok := cat.Skill("dup-skill")
	if !ok || s.Name != "dup-skill custom" {
		t.Errorf("Skill(dup-skill) = %+v (present %v), want the entry from Skills/", s, ok)
	}
	if len(cat.Skills()) != 1 {
		t.Errorf("Skills() has %d entries, want 1", len(cat.Skills()))
	}
}

func TestLoad_CustomRoot_HookInBothLayouts_CurrentPathWins(t *testing.T) {
	mosaicRoot := makeTempMosaicRoot(t)
	custom := makeDistinctCatalogRoot(t)
	writeHookBundleAt(t, custom, "Hooks", "dup-hook", "current")
	writeHookBundleAt(t, custom, filepath.Join("Agents", "Generic", "Hooks"), "dup-hook", "legacy")

	cat, err := catalog.Load(mosaicRoot, custom)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	h, ok := cat.Hook("dup-hook")
	if !ok || h.Description != "current" {
		t.Errorf("Hook(dup-hook) = %+v (present %v), want the entry from Hooks/", h, ok)
	}
	if len(cat.Hooks()) != 1 {
		t.Errorf("Hooks() has %d entries, want 1", len(cat.Hooks()))
	}
}
