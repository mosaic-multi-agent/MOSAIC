package domain_test

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func sampleDamage(resource string) *domain.ArtifactDamageError {
	return &domain.ArtifactDamageError{
		Resource: resource,
		Problems: []domain.ArtifactDamageProblem{
			{Section: "ExecutionLog", Line: 101, Problem: "row has 8 cells, the header has 10"},
			{Section: "ExecutionLog", Line: 103, Problem: "non-table line before the closing tag"},
		},
	}
}

func TestArtifactDamageError_Message_SaysDamagedAndListsEveryProblemWithSectionAndLine(t *testing.T) {
	msg := strings.ToLower(sampleDamage("C:/ws/Orchestration-x/Orchestration.md").Error())

	for _, want := range []string{
		"damaged",
		"c:/ws/orchestration-x/orchestration.md",
		"executionlog", "101", "row has 8 cells, the header has 10",
		"103", "non-table line before the closing tag",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not contain %q", msg, want)
		}
	}
}

func TestArtifactDamageError_Message_WithoutResource_StillSaysDamagedAndWhere(t *testing.T) {
	msg := strings.ToLower(sampleDamage("").Error())

	for _, want := range []string{"damaged", "executionlog", "101", "103"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not contain %q", msg, want)
		}
	}
}

func TestArtifactDamageError_IsDistinctFromRefusalAndPositionUnresolved(t *testing.T) {
	var err error = sampleDamage("p.md")

	var re *domain.RefusalError
	var pu *domain.PositionUnresolvedError
	if errors.As(err, &re) {
		t.Error("a damage error must not match *domain.RefusalError")
	}
	if errors.As(err, &pu) {
		t.Error("a damage error must not match *domain.PositionUnresolvedError")
	}
}

func TestArtifactDamageProblem_String_NamesSectionLineAndProblem(t *testing.T) {
	got := domain.ArtifactDamageProblem{Section: "Artifacts", Line: 7, Problem: "row has 2 cells, the header has 3"}.String()

	for _, want := range []string{"Artifacts", "7", "row has 2 cells, the header has 3"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, does not contain %q", got, want)
		}
	}
}

func TestArtifactDamageProblem_String_WithoutLine_OmitsTheLineNumber(t *testing.T) {
	got := domain.ArtifactDamageProblem{Section: "frontmatter", Problem: "unparseable"}.String()

	if !strings.Contains(got, "frontmatter") || !strings.Contains(got, "unparseable") {
		t.Errorf("String() = %q, want the section and the problem", got)
	}
	if strings.Contains(got, "line") {
		t.Errorf("String() = %q, must not mention a line when none is known", got)
	}
}
