package config_test

// Tests for saving a custom tool answer: CustomToolMapping, SaveToolDestination (under the
// store lock) and the guarantee that a saved entry, loaded and merged, resolves the generic
// to the same harness names and rendered fields as the answer did.

import (
	"errors"
	"reflect"
	"testing"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// answerHarness is a descriptor fixture with the MCP forms of a real harness.
type answerHarness struct {
	id string
	d  *domain.HarnessDescriptor
}

func claudeLikeHarness() answerHarness {
	return answerHarness{"claude-code", &domain.HarnessDescriptor{
		SchemaVersion: "1", ID: "claude-code",
		Tools: domain.ToolSpec{
			Shape:    domain.ShapeList,
			Universe: []domain.HarnessTool{{Name: "Read", Unused: domain.Deny}},
			Mappings: []domain.ToolMapping{mainMapping("file_read", "Read")},
			MCP: domain.MCPNameForms{
				ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}", NormalizeServerName: true,
			},
		},
		Frontmatter: domain.FrontmatterSpec{ToolsKey: "tools"},
	}}
}

func opencodeLikeHarness() answerHarness {
	return answerHarness{"opencode", &domain.HarnessDescriptor{
		SchemaVersion: "1", ID: "opencode",
		Tools: domain.ToolSpec{
			Shape: domain.ShapePermission,
			Universe: []domain.HarnessTool{
				{Name: "read", Unused: domain.Deny}, {Name: "write", Unused: domain.Deny},
			},
			Mappings: []domain.ToolMapping{mainMapping("file_read", "read")},
			MCP: domain.MCPNameForms{
				ServerForm: "{server}_*", ToolForm: "{server}_{tool}", NormalizeServerName: true,
			},
		},
		Frontmatter: domain.FrontmatterSpec{ToolsKey: "permission"},
	}}
}

var answerCases = map[string]domain.CustomToolAnswers{
	"mixed answer": {"pr_comments_read": {
		{Server: "github", Tool: "get_pr"}, {Server: "github", Tool: "list_comments"}, {Server: "jira"},
	}},
	"single bare server": {"pr_comments_read": {{Server: "jira"}}},
}

// resolveCustom resolves the answer on h and returns the ToolCustom result.
func resolveCustom(t *testing.T, h answerHarness, answer domain.CustomToolAnswers) domain.ToolResult {
	t.Helper()
	res, err := descriptor.MapTools(h.d, domain.ToolRequest{
		AgentKey: "agent", Generic: []string{"pr_comments_read"}, CustomNames: answer,
	})
	if err != nil {
		t.Fatalf("MapTools with answer: %v", err)
	}
	if res.Resolutions[0].Outcome != domain.ToolCustom {
		t.Fatalf("answer resolved as %q, want custom", res.Resolutions[0].Outcome)
	}
	return res
}

// resolveAfterSave loads the store, merges its entries for h into a copy of the descriptor and
// resolves the generic without an answer.
func resolveAfterSave(t *testing.T, store config.UserConfigStore, h answerHarness) domain.ToolResult {
	t.Helper()
	cfg := mustLoad(t, store)
	d := *h.d
	d.Tools.Mappings = config.EffectiveToolMappings(h.id, h.d.Tools.Mappings, nil, cfg.ToolDestinations)
	res, err := descriptor.MapTools(&d, domain.ToolRequest{AgentKey: "agent", Generic: []string{"pr_comments_read"}})
	if err != nil {
		t.Fatalf("MapTools after save: %v", err)
	}
	return res
}

func TestSaveToolDestination_SavedAnswer_ResolvesToTheSameNamesAndFieldsAsTheAnswer(t *testing.T) {
	for _, h := range []answerHarness{claudeLikeHarness(), opencodeLikeHarness()} {
		for name, answer := range answerCases {
			t.Run(h.id+"/"+name, func(t *testing.T) {
				store := config.NewUserConfigStore(makeRoot(t))
				answered := resolveCustom(t, h, answer)
				entry, err := config.CustomToolMapping(answered.Resolutions[0])
				if err != nil {
					t.Fatalf("CustomToolMapping: %v", err)
				}
				if err := config.SaveToolDestination(store, h.id, entry); err != nil {
					t.Fatalf("SaveToolDestination: %v", err)
				}

				later := resolveAfterSave(t, store, h)

				got, want := later.Resolutions[0], answered.Resolutions[0]
				if got.Outcome != domain.ToolMapped {
					t.Errorf("later outcome = %q, want mapped", got.Outcome)
				}
				if !reflect.DeepEqual(got.HarnessTools, want.HarnessTools) {
					t.Errorf("HarnessTools = %v, want %v", got.HarnessTools, want.HarnessTools)
				}
				if !reflect.DeepEqual(got.Destinations, want.Destinations) {
					t.Errorf("Destinations = %+v, want %+v", got.Destinations, want.Destinations)
				}
				if !reflect.DeepEqual(later.Fields, answered.Fields) {
					t.Errorf("rendered fields differ:\nlater:    %+v\nanswered: %+v", later.Fields, answered.Fields)
				}
			})
		}
	}
}

func TestSaveToolDestination_SavedAnswer_DoesNotChangeOtherMappingsOfTheHarness(t *testing.T) {
	h := claudeLikeHarness()
	store := config.NewUserConfigStore(makeRoot(t))
	entry, err := config.CustomToolMapping(resolveCustom(t, h, answerCases["mixed answer"]).Resolutions[0])
	if err != nil {
		t.Fatalf("CustomToolMapping: %v", err)
	}
	if err := config.SaveToolDestination(store, h.id, entry); err != nil {
		t.Fatalf("SaveToolDestination: %v", err)
	}
	cfg := mustLoad(t, store)
	d := *h.d
	d.Tools.Mappings = config.EffectiveToolMappings(h.id, h.d.Tools.Mappings, nil, cfg.ToolDestinations)

	res, err := descriptor.MapTools(&d, domain.ToolRequest{AgentKey: "agent", Generic: []string{"file_read"}})

	if err != nil || res.Resolutions[0].Outcome != domain.ToolMapped || res.Resolutions[0].HarnessTools[0] != "Read" {
		t.Errorf("file_read resolution changed after the save: %+v (err %v)", res.Resolutions, err)
	}
}

func TestSaveToolDestination_Save_ChangesTheToolMappingHash(t *testing.T) {
	h := claudeLikeHarness()
	store := config.NewUserConfigStore(makeRoot(t))
	before := config.HashToolDestinations(nil, mustLoad(t, store).ToolDestinations)
	entry, err := config.CustomToolMapping(resolveCustom(t, h, answerCases["mixed answer"]).Resolutions[0])
	if err != nil {
		t.Fatalf("CustomToolMapping: %v", err)
	}

	if err := config.SaveToolDestination(store, h.id, entry); err != nil {
		t.Fatalf("SaveToolDestination: %v", err)
	}

	after := config.HashToolDestinations(nil, mustLoad(t, store).ToolDestinations)
	if before == after {
		t.Error("tool-mapping hash did not change after saving an entry")
	}
}

func TestSaveToolDestination_SaveTwiceForSameGeneric_KeepsOneEntryWithLatestNames(t *testing.T) {
	store := config.NewUserConfigStore(makeRoot(t))
	if err := config.SaveToolDestination(store, "claude-code", mainMapping("g", "a")); err != nil {
		t.Fatalf("first save: %v", err)
	}

	if err := config.SaveToolDestination(store, "claude-code", mainMapping("g", "b")); err != nil {
		t.Fatalf("second save: %v", err)
	}

	cfg := mustLoad(t, store)
	assertMainNames(t, cfg, "claude-code", "g", "b")
	if n := len(cfg.ToolDestinations["claude-code"]); n != 1 {
		t.Errorf("entries = %d, want 1", n)
	}
}

// lockSpyStore records the order of WithLock and PutToolDestination calls.
type lockSpyStore struct {
	config.UserConfigStore
	events   []string
	inLock   bool
	lockErr  error
	putErr   error
	putCalls int
}

func (s *lockSpyStore) WithLock(fn func() error) error {
	s.events = append(s.events, "lock")
	if s.lockErr != nil {
		return s.lockErr
	}
	s.inLock = true
	defer func() { s.inLock = false }()
	return fn()
}

func (s *lockSpyStore) PutToolDestination(string, domain.ToolMapping) error {
	s.putCalls++
	if s.inLock {
		s.events = append(s.events, "put-in-lock")
	} else {
		s.events = append(s.events, "put-outside-lock")
	}
	return s.putErr
}

func TestSaveToolDestination_RunsThePutInsideTheStoreLock(t *testing.T) {
	spy := &lockSpyStore{}

	err := config.SaveToolDestination(spy, "claude-code", mainMapping("g", "a"))

	if err != nil {
		t.Fatalf("SaveToolDestination: %v", err)
	}
	if !reflect.DeepEqual(spy.events, []string{"lock", "put-in-lock"}) {
		t.Errorf("events = %v, want [lock put-in-lock]", spy.events)
	}
}

func TestSaveToolDestination_LockFailure_IsReturnedUnchangedAndNothingIsPut(t *testing.T) {
	lockErr := errors.New("another process holds the lock")
	spy := &lockSpyStore{lockErr: lockErr}

	err := config.SaveToolDestination(spy, "claude-code", mainMapping("g", "a"))

	if !errors.Is(err, lockErr) {
		t.Errorf("err = %v, want the lock error", err)
	}
	if spy.putCalls != 0 {
		t.Errorf("PutToolDestination called %d time(s) without the lock", spy.putCalls)
	}
}

func TestSaveToolDestination_PutFailure_IsReturnedUnchanged(t *testing.T) {
	putErr := errors.New("disk full")
	spy := &lockSpyStore{putErr: putErr}

	err := config.SaveToolDestination(spy, "claude-code", mainMapping("g", "a"))

	if !errors.Is(err, putErr) {
		t.Errorf("err = %v, want the put error", err)
	}
}

func TestCustomToolMapping_CustomResolution_KeepsGenericAndEveryDestinationField(t *testing.T) {
	res := domain.ToolResolution{
		Generic: "g", Outcome: domain.ToolCustom, HarnessTools: []string{"a", "b"},
		Destinations: []domain.ToolDestination{
			{Kind: domain.DestMain, Names: []string{"a", "b"}},
			fieldDest("extra", "scalar", ";", "a", "b"),
		},
	}

	m, err := config.CustomToolMapping(res)

	if err != nil {
		t.Fatalf("CustomToolMapping: %v", err)
	}
	if m.Generic != "g" || !reflect.DeepEqual(m.Destinations, res.Destinations) {
		t.Errorf("mapping = %+v, want generic g with destinations %+v", m, res.Destinations)
	}
}

func TestCustomToolMapping_Result_DoesNotAliasTheResolution(t *testing.T) {
	res := domain.ToolResolution{
		Generic: "g", Outcome: domain.ToolCustom,
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"a"}}},
	}

	m, err := config.CustomToolMapping(res)
	if err != nil {
		t.Fatalf("CustomToolMapping: %v", err)
	}
	m.Destinations[0].Names[0] = "changed"

	if res.Destinations[0].Names[0] != "a" {
		t.Error("changing the mapping changed the resolution's names")
	}
}

func TestCustomToolMapping_NonCustomOutcomeOrNoDestinations_ReturnsError(t *testing.T) {
	dests := []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"a"}}}
	cases := map[string]domain.ToolResolution{
		"mapped":         {Generic: "g", Outcome: domain.ToolMapped, Destinations: dests},
		"skipped":        {Generic: "g", Outcome: domain.ToolSkipped},
		"unmapped":       {Generic: "g", Outcome: domain.ToolUnmapped},
		"no destination": {Generic: "g", Outcome: domain.ToolCustom},
	}
	for name, res := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := config.CustomToolMapping(res); err == nil {
				t.Errorf("CustomToolMapping accepted a %s resolution", name)
			}
		})
	}
	// Control: a custom resolution with a destination, differing from "mapped" only in outcome, is accepted.
	t.Run("control custom with destination", func(t *testing.T) {
		ok := domain.ToolResolution{Generic: "g", Outcome: domain.ToolCustom, Destinations: dests}
		if _, err := config.CustomToolMapping(ok); err != nil {
			t.Errorf("CustomToolMapping rejected a valid custom resolution: %v", err)
		}
	})
}
