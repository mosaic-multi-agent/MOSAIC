package main

// tui_launch_test.go verifies how the command-line stack flags become the stack the TUI
// session starts from. The start decides the stack screen's initial state and the origin of
// an unchanged confirmation, so a wrong mapping would silently drop a persisted stack.

import (
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
)

func TestTUILaunchStack_MapsTheStackFlagsToTheStartingStack(t *testing.T) {
	userA := filepath.Join(t.TempDir(), "a")
	userB := filepath.Join(t.TempDir(), "b")
	cases := []struct {
		name      string
		args      []string
		want      stackconfig.Description
		wantGiven bool
	}{
		{"no stack flags", []string{"--tui"}, stackconfig.Description{IncludeDefault: true}, false},
		{"one user catalog", []string{"--catalog-folder", userA},
			stackconfig.Description{UserLayers: []string{userA}}, true},
		{"default-layer flag alone", []string{"--include-default-catalog"},
			stackconfig.Description{IncludeDefault: true}, true},
		{"user catalog with the default-layer flag", []string{"--catalog-folder", userA, "--include-default-catalog"},
			stackconfig.Description{UserLayers: []string{userA}, IncludeDefault: true}, true},
		{"two user catalogs keep the argument order", []string{"--catalog-folder=" + userA, "--catalog-folder=" + userB},
			stackconfig.Description{UserLayers: []string{userA, userB}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, given := tuiLaunchStack(tc.args)

			if !reflect.DeepEqual(got.UserLayers, tc.want.UserLayers) || got.IncludeDefault != tc.want.IncludeDefault {
				t.Errorf("tuiLaunchStack(%v) description = %+v, want %+v", tc.args, got, tc.want)
			}
			if given != tc.wantGiven {
				t.Errorf("tuiLaunchStack(%v) given = %v, want %v", tc.args, given, tc.wantGiven)
			}
		})
	}
}
