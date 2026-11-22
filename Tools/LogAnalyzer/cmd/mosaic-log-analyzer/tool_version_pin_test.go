package main

import "testing"

func TestToolVersion_IsBumpedForRunsListStartTime(t *testing.T) {
	if ToolVersion != "1.6.0" {
		t.Fatalf("ToolVersion = %q, want 1.6.0", ToolVersion)
	}
}
