package main

import "testing"

func TestToolVersion_IsBumpedForRunsListStartTime(t *testing.T) {
	if ToolVersion != "1.7.0" {
		t.Fatalf("ToolVersion = %q, want 1.7.0", ToolVersion)
	}
}
