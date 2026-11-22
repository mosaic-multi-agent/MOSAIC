package main

import "testing"

func TestToolVersion_IsBumpedForCustomToolDestinations(t *testing.T) {
	if ToolVersion != "1.13.0" {
		t.Fatalf("ToolVersion = %q, want 1.13.0", ToolVersion)
	}
}
