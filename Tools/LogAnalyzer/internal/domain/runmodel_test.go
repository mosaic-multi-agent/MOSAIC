package domain_test

// Tests for the duration rules of the run model: durations are derived only
// from valid timestamp pairs, never guessed.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

var rmBase = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func TestToolCallDuration(t *testing.T) {
	cases := []struct {
		name   string
		call   domain.ToolCall
		want   time.Duration
		wantOK bool
	}{
		{"paired with both timestamps", domain.ToolCall{Pairing: domain.ToolCallPaired, StartedAt: rmBase, EndedAt: rmBase.Add(1500 * time.Millisecond)}, 1500 * time.Millisecond, true},
		{"paired with equal timestamps is zero but usable", domain.ToolCall{Pairing: domain.ToolCallPaired, StartedAt: rmBase, EndedAt: rmBase}, 0, true},
		{"paired but end before start", domain.ToolCall{Pairing: domain.ToolCallPaired, StartedAt: rmBase.Add(time.Second), EndedAt: rmBase}, 0, false},
		{"paired but start timestamp unparseable", domain.ToolCall{Pairing: domain.ToolCallPaired, EndedAt: rmBase}, 0, false},
		{"paired but end timestamp unparseable", domain.ToolCall{Pairing: domain.ToolCallPaired, StartedAt: rmBase}, 0, false},
		{"start only has no duration", domain.ToolCall{Pairing: domain.ToolCallStartOnly, StartedAt: rmBase}, 0, false},
		{"end only has no duration", domain.ToolCall{Pairing: domain.ToolCallEndOnly, EndedAt: rmBase}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.call.Duration()
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("Duration() = (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestInvocationDuration(t *testing.T) {
	cases := []struct {
		name   string
		inv    domain.Invocation
		want   time.Duration
		wantOK bool
	}{
		{"closed with both timestamps", domain.Invocation{State: domain.InvocationClosed, StartedAt: rmBase, EndedAt: rmBase.Add(90 * time.Second)}, 90 * time.Second, true},
		{"open has no duration even with a start", domain.Invocation{State: domain.InvocationOpen, StartedAt: rmBase}, 0, false},
		{"open never gets an invented end", domain.Invocation{State: domain.InvocationOpen, StartedAt: rmBase, EndedAt: rmBase.Add(time.Minute)}, 0, false},
		{"closed without start timestamp", domain.Invocation{State: domain.InvocationClosed, EndedAt: rmBase}, 0, false},
		{"closed without end timestamp", domain.Invocation{State: domain.InvocationClosed, StartedAt: rmBase}, 0, false},
		{"closed with end before start", domain.Invocation{State: domain.InvocationClosed, StartedAt: rmBase.Add(time.Second), EndedAt: rmBase}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.inv.Duration()
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("Duration() = (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
