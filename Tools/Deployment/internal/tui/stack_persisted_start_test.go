package tui

// stack_persisted_start_test.go verifies which stack the stack screen starts from, and which
// origin confirming it unchanged passes to the reload capability:
//
//   - the write modes start from the persisted stack of the confirmed path itself
//   - promote starts from the nearest ancestor workspace of the confirmed file, transform from
//     the confirmed directory or its nearest ancestor workspace
//   - a stack given at launch wins; without a persisted stack the launch stack, or the default
//     layer alone, is the start
//
// Unresolvable persisted layers are covered in stack_persisted_invalid_test.go.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// requireInView fails unless every want is in the current view.
func requireInView(t *testing.T, f *stackFlow, wants ...string) {
	t.Helper()
	view := f.m.View()
	for _, w := range wants {
		if !strings.Contains(view, w) {
			t.Errorf("the view does not contain %q:\n%s", w, view)
		}
	}
}

// requireNotInView fails if any unwanted text is in the current view.
func requireNotInView(t *testing.T, f *stackFlow, unwanted ...string) {
	t.Helper()
	view := f.m.View()
	for _, w := range unwanted {
		if strings.Contains(view, w) {
			t.Errorf("the view contains %q, which must not be there:\n%s", w, view)
		}
	}
}

func TestPersistedStart_WriteModes_StackScreenOpensWithThePersistedStack(t *testing.T) {
	for _, mode := range writeModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			first, second := t.TempDir(), t.TempDir()
			ws := newWorkspaceDir(t, store, storedOf(true, first, second))
			f := newPersistedFlow(t, mode, store, ws, persistFlowOpts{})

			f.toStack(t)

			view := f.m.View()
			i, j := strings.Index(view, first), strings.Index(view, second)
			if i < 0 || j < 0 || i > j {
				t.Errorf("persisted layers are not listed highest first (first at %d, second at %d):\n%s", i, j, view)
			}
			requireInView(t, f, "[x] kept")
		})
	}
}

func TestPersistedStart_ConfirmedUnchanged_UsesThePersistedStackWithPersistedOrigin(t *testing.T) {
	user := t.TempDir()
	cases := []struct {
		name   string
		stored func() []string
		deflt  bool
	}{
		{"user layer with the default layer", func() []string { return []string{user} }, true},
		{"user layer without the default layer", func() []string { return []string{user} }, false},
		{"default layer only", func() []string { return nil }, true},
	}
	for _, tc := range cases {
		for _, mode := range writeModes {
			t.Run(tc.name+"/"+string(mode), func(t *testing.T) {
				store := newPersistStore()
				ws := newWorkspaceDir(t, store, storedOf(tc.deflt, tc.stored()...))
				f := newPersistedFlow(t, mode, store, ws, persistFlowOpts{})
				f.toStack(t)

				cmd := f.confirmStack(t)

				call := requireOneReload(t, f)
				want := stackconfig.Description{UserLayers: tc.stored(), IncludeDefault: tc.deflt}
				if !sameDescription(call.d, want) {
					t.Errorf("rebuilt from %+v, want the persisted stack %+v", call.d, want)
				}
				if call.origin != stackconfig.OriginPersisted {
					t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginPersisted)
				}
				runStarted(cmd)
				if got, _ := store.get(ws); len(got.Layers) != len(storedOf(tc.deflt, tc.stored()...).Layers) {
					t.Errorf("persisted stack changed by an unchanged confirmation: %+v", got)
				}
			})
		}
	}
}

func TestPersistedStart_ChangedOnTheScreen_IsRebuiltAsStackInput(t *testing.T) {
	for _, mode := range writeModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			persisted, added := t.TempDir(), t.TempDir()
			ws := newWorkspaceDir(t, store, storedOf(false, persisted))
			f := newPersistedFlow(t, mode, store, ws, persistFlowOpts{})
			f.toStack(t)
			f.addLayerOnStack(added)

			f.confirmStack(t)

			call := requireOneReload(t, f)
			want := stackconfig.Description{UserLayers: []string{persisted, added}}
			if !sameDescription(call.d, want) {
				t.Errorf("rebuilt from %+v, want the changed stack %+v", call.d, want)
			}
			if call.origin != stackconfig.OriginStackInput {
				t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginStackInput)
			}
		})
	}
}

func TestPersistedStart_Promote_StartsFromTheWorkspaceContainingTheFile(t *testing.T) {
	store := newPersistStore()
	persisted := t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, persisted))
	file := createAgentFile(t, subDir(t, ws, "nested"))
	f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, file, persistFlowOpts{})

	f.toStack(t)
	requireInView(t, f, persisted)
	f.confirmStack(t)

	call := requireOneReload(t, f)
	if !sameDescription(call.d, stackconfig.Description{UserLayers: []string{persisted}}) {
		t.Errorf("rebuilt from %+v, want the workspace's persisted stack", call.d)
	}
	if call.origin != stackconfig.OriginPersisted {
		t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginPersisted)
	}
}

func TestPersistedStart_Promote_FileWithoutWorkspace_StartsFromLaunchOrDefault(t *testing.T) {
	user := t.TempDir()
	cases := []struct {
		name       string
		launch     stackconfig.Description
		given      bool
		wantOrigin stackconfig.Origin
	}{
		{"default layer alone", stackconfig.Description{IncludeDefault: true}, false, stackconfig.OriginDefault},
		{"launch stack", stackconfig.Description{UserLayers: []string{user}}, true, stackconfig.OriginStackInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newPersistStore()
			stray := t.TempDir()
			newWorkspaceDir(t, store, storedOf(false, stray)) // a workspace elsewhere must not apply
			file := createAgentFile(t, t.TempDir())
			f := newPersistedFlow(t, domain.ModePromoteToGeneric, store, file, persistFlowOpts{},
				withLaunch(tc.launch, tc.given))

			f.toStack(t)
			requireNotInView(t, f, stray)
			f.confirmStack(t)

			call := requireOneReload(t, f)
			if !sameDescription(call.d, tc.launch) {
				t.Errorf("rebuilt from %+v, want %+v", call.d, tc.launch)
			}
			if call.origin != tc.wantOrigin {
				t.Errorf("origin = %q, want %q", call.origin, tc.wantOrigin)
			}
		})
	}
}

func TestPersistedStart_Transform_StartsFromTheDirectoryOrFileWorkspace(t *testing.T) {
	store := newPersistStore()
	persisted := t.TempDir()
	ws := newWorkspaceDir(t, store, storedOf(false, persisted))
	inner := subDir(t, ws, "nested")
	paths := map[string]string{
		"the workspace directory":      ws,
		"a directory inside it":        inner,
		"a file inside it":             createAgentFile(t, inner),
		"a file directly in it":        createAgentFile(t, ws),
		"a directory deeper inside it": subDir(t, inner, "deeper"),
	}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			f := newPersistedFlow(t, domain.ModeTransformHarness, store, path, persistFlowOpts{})

			f.toStack(t)
			requireInView(t, f, persisted)
			f.confirmStack(t)

			call := requireOneReload(t, f)
			if call.origin != stackconfig.OriginPersisted {
				t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginPersisted)
			}
		})
	}
}

func TestPersistedStart_Transform_PathWithoutWorkspace_StartsFromDefaultLayer(t *testing.T) {
	store := newPersistStore()
	stray := t.TempDir()
	newWorkspaceDir(t, store, storedOf(false, stray))
	f := newPersistedFlow(t, domain.ModeTransformHarness, store, t.TempDir(), persistFlowOpts{})

	f.toStack(t)
	requireNotInView(t, f, stray)
	f.confirmStack(t)

	call := requireOneReload(t, f)
	if call.origin != stackconfig.OriginDefault {
		t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginDefault)
	}
}

func TestPersistedStart_WriteModes_OnlyTheConfirmedPathItselfCounts(t *testing.T) {
	for _, mode := range writeModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			persisted := t.TempDir()
			ws := newWorkspaceDir(t, store, storedOf(false, persisted))
			inner := subDir(t, ws, "nested")
			f := newPersistedFlow(t, mode, store, inner, persistFlowOpts{})

			f.toStack(t)

			requireNotInView(t, f, persisted)
			f.confirmStack(t)
			if call := requireOneReload(t, f); call.origin != stackconfig.OriginDefault {
				t.Errorf("origin = %q, want %q: a directory inside a workspace is not that workspace", call.origin, stackconfig.OriginDefault)
			}
		})
	}
}

func TestPersistedStart_LaunchStackInput_WinsOverThePersistedStack(t *testing.T) {
	for _, mode := range writeModes {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			persisted, launch := t.TempDir(), t.TempDir()
			ws := newWorkspaceDir(t, store, storedOf(true, persisted))
			launchStack := stackconfig.Description{UserLayers: []string{launch}}
			f := newPersistedFlow(t, mode, store, ws, persistFlowOpts{}, withLaunch(launchStack, true))

			f.toStack(t)
			requireInView(t, f, launch)
			requireNotInView(t, f, persisted)
			f.confirmStack(t)

			call := requireOneReload(t, f)
			if !sameDescription(call.d, launchStack) {
				t.Errorf("rebuilt from %+v, want the launch stack %+v", call.d, launchStack)
			}
			if call.origin != stackconfig.OriginStackInput {
				t.Errorf("origin = %q, want %q", call.origin, stackconfig.OriginStackInput)
			}
		})
	}
}

func TestPersistedStart_LaunchStackInput_WinsInPromoteAndTransform(t *testing.T) {
	for _, mode := range []domain.RunMode{domain.ModePromoteToGeneric, domain.ModeTransformHarness} {
		t.Run(string(mode), func(t *testing.T) {
			store := newPersistStore()
			persisted, launch := t.TempDir(), t.TempDir()
			ws := newWorkspaceDir(t, store, storedOf(true, persisted))
			path := ws
			if mode == domain.ModePromoteToGeneric {
				path = createAgentFile(t, ws)
			}
			launchStack := stackconfig.Description{UserLayers: []string{launch}}
			f := newPersistedFlow(t, mode, store, path, persistFlowOpts{}, withLaunch(launchStack, true))

			f.toStack(t)
			requireInView(t, f, launch)
			requireNotInView(t, f, persisted)
			f.confirmStack(t)

			call := requireOneReload(t, f)
			if !sameDescription(call.d, launchStack) || call.origin != stackconfig.OriginStackInput {
				t.Errorf("rebuilt as %+v with origin %q, want the launch stack with origin %q",
					call.d, call.origin, stackconfig.OriginStackInput)
			}
		})
	}
}

func TestPersistedStart_WorkspaceWithoutPersistedStack_StartsFromLaunchOrDefault(t *testing.T) {
	user := t.TempDir()
	cases := []struct {
		name       string
		launch     stackconfig.Description
		given      bool
		wantOrigin stackconfig.Origin
	}{
		{"default layer alone", stackconfig.Description{IncludeDefault: true}, false, stackconfig.OriginDefault},
		{"launch stack", stackconfig.Description{UserLayers: []string{user}, IncludeDefault: true}, true, stackconfig.OriginStackInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newPersistStore()
			ws := newWorkspaceDir(t, store, stackconfig.Stored{})
			f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{},
				withLaunch(tc.launch, tc.given))

			f.toStack(t)
			f.confirmStack(t)

			call := requireOneReload(t, f)
			if !sameDescription(call.d, tc.launch) {
				t.Errorf("rebuilt from %+v, want %+v", call.d, tc.launch)
			}
			if call.origin != tc.wantOrigin {
				t.Errorf("origin = %q, want %q", call.origin, tc.wantOrigin)
			}
		})
	}
}

func TestPersistedStart_Rows_ShowTheStoredCatalogIdAndTheActiveIdentity(t *testing.T) {
	store := newPersistStore()
	user := t.TempDir()
	ws := newWorkspaceDir(t, store, stackconfig.Stored{Layers: []domain.PersistedCatalogLayer{
		{Path: user, CatalogID: "stored-team-id"},
	}})
	t.Run("stored id only", func(t *testing.T) {
		f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{})

		f.toStack(t)

		requireInView(t, f, "stored-team-id", user)
	})
	t.Run("active identity replaces it", func(t *testing.T) {
		active := stackconfig.ActiveStack{Layers: []domain.CatalogLayer{{
			Root:     user,
			Identity: domain.CatalogIdentity{ID: "stored-team-id", Name: "Team Catalog Name", Version: "7.1.0"},
		}}}
		f := newPersistedFlow(t, domain.ModeUpdateWorkspace, store, ws, persistFlowOpts{active: active})

		f.toStack(t)

		requireInView(t, f, "Team Catalog Name", "7.1.0")
	})
}
