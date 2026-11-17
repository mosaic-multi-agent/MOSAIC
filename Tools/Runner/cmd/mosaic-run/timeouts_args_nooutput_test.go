package main

// Tests that --no-output-timeout is resolved by the pre-scan into the
// NoOutput value handed to buildAdapter: absent or invalid gives the default,
// 0 disables, a positive value is used as given.

import (
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

func TestTimeoutsFromArgs_NoOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want time.Duration
	}{
		{"absent gives the default", []string{"--harness", "fake"}, domain.DefaultNoOutputTimeout},
		{"space form", []string{"--no-output-timeout", "3m"}, 3 * time.Minute},
		{"equals form", []string{"--no-output-timeout=45s"}, 45 * time.Second},
		{"zero disables", []string{"--no-output-timeout", "0"}, domain.NoOutputDisabled},
		{"zero seconds disables", []string{"--no-output-timeout=0s"}, domain.NoOutputDisabled},
		{"unparseable gives the default", []string{"--no-output-timeout", "soon"}, domain.DefaultNoOutputTimeout},
		{"negative gives the default", []string{"--no-output-timeout", "-5m"}, domain.DefaultNoOutputTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := timeoutsFromArgs(tc.args).NoOutput

			if got != tc.want {
				t.Fatalf("timeoutsFromArgs(%v).NoOutput = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestTimeoutsFromArgs_NoOutputDoesNotChangeTheOtherTimeouts(t *testing.T) {
	got := timeoutsFromArgs([]string{"--timeout", "10m", "--no-output-timeout", "2m", "--hitl-timeout", "6h"})

	want := domain.InvocationTimeouts{NonHITL: 10 * time.Minute, HITL: 6 * time.Hour, NoOutput: 2 * time.Minute}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
