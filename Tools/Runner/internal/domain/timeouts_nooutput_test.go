package domain_test

import (
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

func TestInvocationTimeouts_NoOutputDefaultIsTenMinutes(t *testing.T) {
	if domain.DefaultNoOutputTimeout != 10*time.Minute {
		t.Errorf("DefaultNoOutputTimeout = %v, want 10m", domain.DefaultNoOutputTimeout)
	}
}

func TestInvocationTimeouts_NoOutputDisabledIsNegative(t *testing.T) {
	if domain.NoOutputDisabled >= 0 {
		t.Errorf("NoOutputDisabled = %v, want a negative value", domain.NoOutputDisabled)
	}
}

func TestInvocationTimeouts_NormalizedNoOutput(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"unset gets the default", 0, domain.DefaultNoOutputTimeout},
		{"positive value kept", 90 * time.Second, 90 * time.Second},
		{"disabled stays disabled", domain.NoOutputDisabled, domain.NoOutputDisabled},
		{"any negative value stays negative", -time.Hour, -time.Hour},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.InvocationTimeouts{NoOutput: tc.in}.Normalized().NoOutput
			if got != tc.want {
				t.Errorf("Normalized().NoOutput = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInvocationTimeouts_ForNeverReturnsNoOutput(t *testing.T) {
	timeouts := domain.InvocationTimeouts{NonHITL: time.Minute, HITL: time.Hour, NoOutput: 5 * time.Second}
	if got := timeouts.For(false); got != time.Minute {
		t.Errorf("For(false) = %v, want 1m", got)
	}
	if got := timeouts.For(true); got != time.Hour {
		t.Errorf("For(true) = %v, want 1h", got)
	}
}

func TestHarnessFailureNoOutput_KindText(t *testing.T) {
	f := domain.HarnessFailure{Kind: domain.HarnessFailureNoOutput, Detail: "x"}
	if got := f.Summary(); got != "no-output-timeout: x" {
		t.Errorf("Summary() = %q, want %q", got, "no-output-timeout: x")
	}
}
