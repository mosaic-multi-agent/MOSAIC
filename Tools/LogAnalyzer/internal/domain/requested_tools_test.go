package domain_test

// Tests for domain.RequestedTools: the tools one API call requested.
// The zero value is "not provided"; a provided value may be empty.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestRequestedTools_ZeroValue_IsNotProvided(t *testing.T) {
	var r domain.RequestedTools

	if r.IsProvided() || r.Total() != 0 || r.Names() != nil || r.Counts() != nil {
		t.Errorf("zero value = provided %v total %d names %v counts %v, want not provided",
			r.IsProvided(), r.Total(), r.Names(), r.Counts())
	}
}

func TestRequestedTools_NilCounts_IsProvidedAndEmpty(t *testing.T) {
	r := domain.NewRequestedTools(nil)

	if !r.IsProvided() {
		t.Fatal("NewRequestedTools(nil) is not provided, want provided")
	}
	if r.Total() != 0 {
		t.Errorf("Total = %d, want 0", r.Total())
	}
	if c := r.Counts(); c == nil || len(c) != 0 {
		t.Errorf("Counts = %#v, want a non-nil empty map", c)
	}
	if _, ok := r.Single(); ok {
		t.Error("Single ok on an empty value")
	}
}

func TestRequestedTools_Counts_AreKeptWithTotalNamesAndCount(t *testing.T) {
	r := domain.NewRequestedTools(map[string]int{"read_file": 2, "glob": 1})

	if r.Total() != 3 {
		t.Errorf("Total = %d, want 3", r.Total())
	}
	if got := r.Names(); !reflect.DeepEqual(got, []string{"glob", "read_file"}) {
		t.Errorf("Names = %v, want sorted [glob read_file]", got)
	}
	if r.Count("read_file") != 2 || r.Count("glob") != 1 || r.Count("edit") != 0 {
		t.Errorf("counts = %d/%d/%d, want 2/1/0", r.Count("read_file"), r.Count("glob"), r.Count("edit"))
	}
}

func TestRequestedTools_InvalidEntries_AreIgnored(t *testing.T) {
	r := domain.NewRequestedTools(map[string]int{"": 3, "zero": 0, "neg": -1, "ok": 1})

	if r.Total() != 1 || !reflect.DeepEqual(r.Names(), []string{"ok"}) {
		t.Errorf("Total %d names %v, want only ok", r.Total(), r.Names())
	}
}

func TestRequestedTools_Single_IsOnlyOneToolRequestedOnce(t *testing.T) {
	cases := []struct {
		name   string
		counts map[string]int
		tool   string
		ok     bool
	}{
		{"one tool once", map[string]int{"glob": 1}, "glob", true},
		{"one tool twice", map[string]int{"glob": 2}, "", false},
		{"two tools", map[string]int{"glob": 1, "edit": 1}, "", false},
		{"empty", map[string]int{}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool, ok := domain.NewRequestedTools(c.counts).Single()

			if tool != c.tool || ok != c.ok {
				t.Errorf("Single = (%q, %v), want (%q, %v)", tool, ok, c.tool, c.ok)
			}
		})
	}
}

func TestRequestedTools_Counts_ReturnsACopyAndInputIsCopied(t *testing.T) {
	in := map[string]int{"glob": 1}
	r := domain.NewRequestedTools(in)

	in["glob"] = 9
	if c := r.Counts(); c != nil {
		c["glob"] = 7
	}

	if r.Count("glob") != 1 {
		t.Errorf("Count = %d, want 1: value must not alias caller maps", r.Count("glob"))
	}
}

func TestRequestedTools_Equal_DistinguishesEmptyFromNotProvided(t *testing.T) {
	var absent domain.RequestedTools
	empty := domain.NewRequestedTools(nil)
	glob := domain.NewRequestedTools(map[string]int{"glob": 1})

	if absent.Equal(empty) || empty.Equal(absent) {
		t.Error("not provided equals provided-empty, want different")
	}
	if !absent.Equal(domain.RequestedTools{}) {
		t.Error("two not-provided values are different, want equal")
	}
	if !glob.Equal(domain.NewRequestedTools(map[string]int{"glob": 1})) {
		t.Error("equal counts are different")
	}
	if glob.Equal(domain.NewRequestedTools(map[string]int{"glob": 2})) || glob.Equal(empty) {
		t.Error("different counts are equal")
	}
}
