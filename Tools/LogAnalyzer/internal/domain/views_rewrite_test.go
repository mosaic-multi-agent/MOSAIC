package domain_test

// Tests for the context rewrite vocabulary: kind keys, the label shown for a
// rewrite (minutes truncated, under one minute named as such, no other cause)
// and the count of a figures block.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func TestRewriteKind_KeysAreStable(t *testing.T) {
	want := map[domain.RewriteKind]string{
		domain.RewriteModelChanged: "model_changed",
		domain.RewriteColdIdle:     "cold_idle",
		domain.RewritePartial:      "partial",
	}
	for kind, key := range want {
		if got := kind.Key(); got != key {
			t.Errorf("kind %d key = %q, want %q", kind, got, key)
		}
	}
}

func TestContextRewrite_LabelNamesOnlyWhatTheLogsProve(t *testing.T) {
	cases := []struct {
		name string
		r    domain.ContextRewrite
		want string
	}{
		{"model changed", domain.ContextRewrite{Kind: domain.RewriteModelChanged, Pause: 20 * time.Minute}, "model changed"},
		{"partial", domain.ContextRewrite{Kind: domain.RewritePartial, Pause: 3 * time.Minute}, "partial rewrite"},
		{"cold ten minutes", domain.ContextRewrite{Kind: domain.RewriteColdIdle, Pause: 10 * time.Minute}, "cold after 10 min idle"},
		{"cold one minute exactly", domain.ContextRewrite{Kind: domain.RewriteColdIdle, Pause: time.Minute}, "cold after 1 min idle"},
		{"cold minutes are truncated", domain.ContextRewrite{Kind: domain.RewriteColdIdle, Pause: 119 * time.Second}, "cold after 1 min idle"},
		{"cold under a minute", domain.ContextRewrite{Kind: domain.RewriteColdIdle, Pause: 59 * time.Second}, "cold after <1 min idle"},
		{"cold without a pause", domain.ContextRewrite{Kind: domain.RewriteColdIdle}, "cold after <1 min idle"},
		{"cold over an hour", domain.ContextRewrite{Kind: domain.RewriteColdIdle, Pause: 125 * time.Minute}, "cold after 125 min idle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.r.Label(); got != c.want {
				t.Errorf("Label() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRewriteFigures_CountSumsEveryKind(t *testing.T) {
	f := domain.RewriteFigures{ModelChanged: 2, ColdIdle: 3, Partial: 5}

	if got := f.Count(); got != 10 {
		t.Errorf("Count() = %d, want 10", got)
	}
	if got := (domain.RewriteFigures{}).Count(); got != 0 {
		t.Errorf("Count() of the zero value = %d, want 0", got)
	}
}
