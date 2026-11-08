package domain_test

import (
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

func TestInvocationTimeouts_Defaults(t *testing.T) {
	if domain.DefaultTimeout != 30*time.Minute {
		t.Errorf("DefaultTimeout = %v, want 30m", domain.DefaultTimeout)
	}
	if domain.DefaultHITLTimeout != 4*time.Hour {
		t.Errorf("DefaultHITLTimeout = %v, want 4h", domain.DefaultHITLTimeout)
	}
}

func TestInvocationTimeouts_Normalized(t *testing.T) {
	tests := []struct {
		name string
		in   domain.InvocationTimeouts
		want domain.InvocationTimeouts
	}{
		{"zero value gets both defaults", domain.InvocationTimeouts{},
			domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: domain.DefaultHITLTimeout}},
		{"negative values get defaults", domain.InvocationTimeouts{NonHITL: -time.Second, HITL: -time.Hour},
			domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: domain.DefaultHITLTimeout}},
		{"only HITL unset", domain.InvocationTimeouts{NonHITL: time.Minute},
			domain.InvocationTimeouts{NonHITL: time.Minute, HITL: domain.DefaultHITLTimeout}},
		{"only non-HITL unset", domain.InvocationTimeouts{HITL: 2 * time.Hour},
			domain.InvocationTimeouts{NonHITL: domain.DefaultTimeout, HITL: 2 * time.Hour}},
		{"explicit values kept", domain.InvocationTimeouts{NonHITL: time.Minute, HITL: time.Hour},
			domain.InvocationTimeouts{NonHITL: time.Minute, HITL: time.Hour}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Normalized(); got != tc.want {
				t.Errorf("Normalized() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestInvocationTimeouts_For_SelectsByHITL(t *testing.T) {
	timeouts := domain.InvocationTimeouts{NonHITL: time.Minute, HITL: time.Hour}
	if got := timeouts.For(false); got != time.Minute {
		t.Errorf("For(false) = %v, want 1m", got)
	}
	if got := timeouts.For(true); got != time.Hour {
		t.Errorf("For(true) = %v, want 1h", got)
	}
}

func TestInvocationTimeouts_For_FallsBackToDefaults(t *testing.T) {
	var timeouts domain.InvocationTimeouts
	if got := timeouts.For(false); got != domain.DefaultTimeout {
		t.Errorf("For(false) = %v, want default %v", got, domain.DefaultTimeout)
	}
	if got := timeouts.For(true); got != domain.DefaultHITLTimeout {
		t.Errorf("For(true) = %v, want default %v", got, domain.DefaultHITLTimeout)
	}
}
