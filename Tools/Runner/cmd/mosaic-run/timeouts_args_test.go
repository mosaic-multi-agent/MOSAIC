package main

// Tests that the CLI timeout flags are resolved into the invocation timeouts
// handed to buildAdapter.

import (
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

func TestTimeoutsFromArgs(t *testing.T) {
	def := domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: domain.DefaultHITLTimeout}

	tests := []struct {
		name string
		args []string
		want domain.InvocationTimeouts
	}{
		{"both absent use defaults", []string{"--harness", "fake"}, def},
		{"hitl timeout space form sets HITL only",
			[]string{"--hitl-timeout", "2h"},
			domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: 2 * time.Hour}},
		{"hitl timeout equals form sets HITL only",
			[]string{"--hitl-timeout=90m"},
			domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: 90 * time.Minute}},
		{"timeout sets NonHITL only",
			[]string{"--timeout", "10m"},
			domain.InvocationTimeouts{NonHITL: 10 * time.Minute, HITL: domain.DefaultHITLTimeout}},
		{"both flags set independently",
			[]string{"--timeout=10m", "--hitl-timeout", "6h"},
			domain.InvocationTimeouts{NonHITL: 10 * time.Minute, HITL: 6 * time.Hour}},
		{"invalid hitl timeout falls back to default",
			[]string{"--hitl-timeout", "soon"},
			def},
		{"non-positive hitl timeout falls back to default",
			[]string{"--hitl-timeout", "-5m"},
			def},
		{"zero hitl timeout falls back to default",
			[]string{"--hitl-timeout", "0s"},
			def},
		{"invalid timeout falls back to default",
			[]string{"--timeout", "soon", "--hitl-timeout", "2h"},
			domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: 2 * time.Hour}},
		{"non-positive timeout falls back to default",
			[]string{"--timeout", "-1m"},
			def},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := timeoutsFromArgs(tc.args)

			if got != tc.want {
				t.Fatalf("timeoutsFromArgs(%v) = %+v, want %+v", tc.args, got, tc.want)
			}
		})
	}
}
