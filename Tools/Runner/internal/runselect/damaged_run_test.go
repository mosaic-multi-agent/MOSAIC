package runselect_test

// A damaged run stays selectable. Its identity says it is damaged, carries what
// the intact frontmatter recorded, and the announcement tells the user the
// artifact is repaired when the run starts.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/runscan"
	"mosaic-run/internal/runselect"
)

const damagedRunID = "20260701T120000Z-a3f9"

func damagedCandidate() runscan.RunCandidate {
	c := candidate(damagedRunID, filepath.Join(testWorkDir, domain.RunScopedFolder(damagedRunID)))
	c.Damaged = true
	c.RunnerSettingsRecorded = true
	return c
}

// damagedFrontmatterRun is a damaged run whose frontmatter could not be read and
// that has no usable last-good copy, so the scan lists it as unresumable with
// the folder-derived facts only.
const damagedFrontmatterDetail = "Orchestration.md is damaged: frontmatter problem at line 4"

func damagedFrontmatterRun() runscan.UnresumableRun {
	return runscan.UnresumableRun{
		RunInfo: runscan.RunInfo{
			RunID:      damagedRunID,
			FolderPath: filepath.Join(testWorkDir, domain.RunScopedFolder(damagedRunID)),
			Damaged:    true,
		},
		Reason: runscan.ReasonDamagedFrontmatter,
		Detail: damagedFrontmatterDetail,
	}
}

func resolveNamed(t *testing.T, c runscan.RunCandidate) runselect.Identity {
	t.Helper()
	mint, _ := countingMinter()
	dec, err := runselect.Resolve(runselect.Request{
		Scan:      runscan.ScanResult{Candidates: []runscan.RunCandidate{c}},
		WorkDir:   testWorkDir,
		RunIDFlag: damagedRunID,
	}, mint)
	if err != nil {
		t.Fatalf("Resolve error = %v, want the damaged run resolved", err)
	}
	if dec.Resolved == nil {
		t.Fatal("Resolve returned no resolved identity")
	}
	return *dec.Resolved
}

func TestResolve_NamedDamagedRun_IdentityIsMarkedDamaged(t *testing.T) {
	id := resolveNamed(t, damagedCandidate())

	if !id.Damaged {
		t.Error("Identity.Damaged = false, want true for a run whose artifact is damaged")
	}
	if id.IsNewRun {
		t.Error("Identity.IsNewRun = true, want a resume")
	}
}

func TestResolve_NamedDamagedRun_CarriesWhatTheFrontmatterRecorded(t *testing.T) {
	c := damagedCandidate()
	c.CommitSetupPending = true

	id := resolveNamed(t, c)

	if id.Workflow != "test-workflow" {
		t.Errorf("Identity.Workflow = %q, want the recorded test-workflow", id.Workflow)
	}
	if id.Position == nil || id.Position.LastAgent != "impl#1" {
		t.Errorf("Identity.Position = %+v, want the recorded position (last agent impl#1)", id.Position)
	}
	if id.NeedsRunnerAdoption {
		t.Error("Identity.NeedsRunnerAdoption = true, want false: the frontmatter records runner settings")
	}
	if !id.CommitSetupPending {
		t.Error("Identity.CommitSetupPending = false, want true from the frontmatter")
	}
}

func TestResolve_NamedDamagedRunWithDamagedFrontmatterAndNoLastGoodCopy_IsAUsageErrorNamingTheDamage(t *testing.T) {
	mint, _ := countingMinter()

	dec, err := runselect.Resolve(runselect.Request{
		Scan:      runscan.ScanResult{Unresumable: []runscan.UnresumableRun{damagedFrontmatterRun()}},
		WorkDir:   testWorkDir,
		RunIDFlag: damagedRunID,
	}, mint)

	if !errors.Is(err, runselect.ErrUsage) {
		t.Fatalf("Resolve error = %v, want a usage error", err)
	}
	if dec.Resolved != nil {
		t.Errorf("Resolve resolved %+v, want no identity for an unresumable run", dec.Resolved)
	}
	if !strings.Contains(err.Error(), damagedRunID) || !strings.Contains(err.Error(), damagedFrontmatterDetail) {
		t.Errorf("error %q does not name the run and the damage", err)
	}
}

func TestResolve_Question_ShowsADamagedFrontmatterRunUnresumableWithItsDetail(t *testing.T) {
	mint, _ := countingMinter()
	dec, err := runselect.Resolve(runselect.Request{
		Scan:    runscan.ScanResult{Unresumable: []runscan.UnresumableRun{damagedFrontmatterRun()}},
		WorkDir: testWorkDir,
	}, mint)
	if err != nil || dec.Question == nil {
		t.Fatalf("Resolve = %+v, %v, want a question", dec, err)
	}
	for _, c := range dec.Question.Choices {
		if c.ID != damagedRunID {
			continue
		}
		if c.Kind != runselect.ChoiceUnresumable || c.Selectable {
			t.Errorf("choice = kind %v selectable %v, want a non-selectable unresumable choice", c.Kind, c.Selectable)
		}
		if c.Reason != runscan.ReasonDamagedFrontmatter || c.Detail != damagedFrontmatterDetail {
			t.Errorf("choice reason/detail = %q/%q, want the damaged-frontmatter reason and the damage detail", c.Reason, c.Detail)
		}
		if !c.Run.Damaged {
			t.Error("Choice.Run.Damaged = false, want the damage mark carried")
		}
		return
	}
	t.Fatalf("the damaged run is missing from the choices: %+v", dec.Question.Choices)
}

func TestAnswer_DamagedFrontmatterChoiceWithoutLastGoodCopy_IsRefusedNamingTheDamage(t *testing.T) {
	mint, _ := countingMinter()
	run := damagedFrontmatterRun()
	q := runselect.Question{Choices: []runselect.Choice{
		{ID: runselect.NewRunChoiceID, Kind: runselect.ChoiceNewRun, Selectable: true},
		{ID: run.RunID, Kind: runselect.ChoiceUnresumable, Run: run.RunInfo, Reason: run.Reason, Detail: run.Detail},
	}}

	_, err := runselect.Answer(q, damagedRunID, mint)

	if err == nil || !strings.Contains(err.Error(), damagedFrontmatterDetail) {
		t.Errorf("Answer error = %v, want a refusal naming the damage", err)
	}
}

func TestResolve_NamedUndamagedRun_IsNotMarkedDamaged(t *testing.T) {
	c := damagedCandidate()
	c.Damaged = false

	id := resolveNamed(t, c)

	if id.Damaged {
		t.Error("Identity.Damaged = true for a run that is not damaged")
	}
}

func TestResolve_NewRun_IsNeverMarkedDamaged(t *testing.T) {
	mint, _ := countingMinter()
	dec, err := runselect.Resolve(runselect.Request{
		Scan:    runscan.ScanResult{Candidates: []runscan.RunCandidate{damagedCandidate()}},
		WorkDir: testWorkDir,
		NewRun:  true,
	}, mint)
	if err != nil || dec.Resolved == nil {
		t.Fatalf("Resolve = %+v, %v, want a resolved new run", dec, err)
	}
	if dec.Resolved.Damaged {
		t.Error("Identity.Damaged = true for a new run")
	}
}

func TestResolve_Question_OffersTheDamagedRunSelectableAndMarked(t *testing.T) {
	mint, _ := countingMinter()
	dec, err := runselect.Resolve(runselect.Request{
		Scan:    runscan.ScanResult{Candidates: []runscan.RunCandidate{damagedCandidate()}},
		WorkDir: testWorkDir,
	}, mint)
	if err != nil || dec.Question == nil {
		t.Fatalf("Resolve = %+v, %v, want a question", dec, err)
	}
	for _, c := range dec.Question.Choices {
		if c.ID != damagedRunID {
			continue
		}
		if c.Kind != runselect.ChoiceResume || !c.Selectable {
			t.Errorf("choice = kind %v selectable %v, want a selectable resume choice", c.Kind, c.Selectable)
		}
		if !c.Run.Damaged {
			t.Error("Choice.Run.Damaged = false, want the damage mark carried to the list")
		}
		return
	}
	t.Fatalf("the damaged run is missing from the choices: %+v", dec.Question.Choices)
}

func TestAnswer_DamagedChoice_IdentityIsMarkedDamagedWithTheRecordedWorkflow(t *testing.T) {
	mint, _ := countingMinter()
	q := runselect.Question{Choices: []runselect.Choice{
		{ID: runselect.NewRunChoiceID, Kind: runselect.ChoiceNewRun, Selectable: true},
		{ID: damagedRunID, Kind: runselect.ChoiceResume, Selectable: true, Run: damagedCandidate().RunInfo},
	}}

	id, err := runselect.Answer(q, damagedRunID, mint)

	if err != nil {
		t.Fatalf("Answer error = %v, want the damaged run chosen", err)
	}
	if !id.Damaged {
		t.Error("Identity.Damaged = false, want true")
	}
	if id.Workflow != "test-workflow" {
		t.Errorf("Identity.Workflow = %q, want the recorded test-workflow", id.Workflow)
	}
}

func TestAnnounce_DamagedRun_SaysTheArtifactIsDamagedAndRepairedAtStart(t *testing.T) {
	id := runselect.Identity{RunID: damagedRunID, Damaged: true, Position: &runselect.Position{
		Phase: "EXECUTION", LastAgent: "impl#1",
	}}

	text := runselect.Announce(id)

	if !strings.Contains(text, damagedRunID) {
		t.Errorf("announcement %q does not name the run", text)
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "damaged") || !strings.Contains(lower, "repair") {
		t.Errorf("announcement %q does not say the artifact is damaged and is repaired when the run starts", text)
	}
	if !strings.Contains(text, "impl#1") {
		t.Errorf("announcement %q lost the recorded position", text)
	}
}

func TestAnnounce_DamagedRunWithoutPosition_StillSaysTheArtifactIsDamaged(t *testing.T) {
	text := runselect.Announce(runselect.Identity{RunID: damagedRunID, Damaged: true})

	if !strings.Contains(strings.ToLower(text), "damaged") {
		t.Errorf("announcement %q does not say the artifact is damaged", text)
	}
}

func TestAnnounce_UndamagedRun_SaysNothingOfDamage(t *testing.T) {
	text := runselect.Announce(runselect.Identity{RunID: damagedRunID, Position: &runselect.Position{
		Phase: "EXECUTION", LastAgent: "impl#1",
	}})

	if strings.Contains(strings.ToLower(text), "damaged") {
		t.Errorf("announcement %q mentions damage for a run that is not damaged", text)
	}
}
