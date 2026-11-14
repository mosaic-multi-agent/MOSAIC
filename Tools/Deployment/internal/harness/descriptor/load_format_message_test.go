package descriptor_test

// Tests that the descriptor loader's unknown-format error lists the formats of the format
// vocabulary, so adding a format to the vocabulary needs no edit to the loader message.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat/formatid"
	"mosaic-deploy/internal/harness/descriptor"
)

func TestLoad_UnknownAgentFormatID_MessageListsEveryVocabularyFormatQuoted(t *testing.T) {
	// Arrange
	path := filepath.Join(testdataDir, "invalid", "unknown-agent-format-id.yaml")

	// Act
	_, err := descriptor.Load(path)

	// Assert
	if err == nil {
		t.Fatal("Load(invalid/unknown-agent-format-id.yaml): expected an error, got nil")
	}
	for _, id := range formatid.All() {
		if quoted := fmt.Sprintf("%q", string(id)); !strings.Contains(err.Error(), quoted) {
			t.Errorf("error %q does not list the format %s from the vocabulary", err.Error(), quoted)
		}
	}
}
