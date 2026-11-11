package runner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	goyaml "github.com/goccy/go-yaml"

	"mosaic-agent-test/internal/domain"
)

// checkSubjectPlacement requires that the harness can resolve the subject
// from its identity: the provisioned definition lies directly in the
// harness's project-level agents directory under subjectDir and its `name:`
// equals the identity. Anything else would launch a subject the harness
// cannot find, so it is reported as a setup error naming the subject and its
// definition file. An adapter that reports no agents directory, and a subject
// that was not provisioned from a catalogue key, are not checked.
func checkSubjectPlacement(adapter domain.HarnessAdapter, subjectDir string, subject domain.SubjectUnderTest) error {
	reader, ok := adapter.(domain.SubjectPlacementReader)
	if !ok || subject.CatalogAgentKey == "" || subject.DefinitionPath == "" {
		return nil
	}
	agentsRel := reader.SubjectAgentsDir()
	if agentsRel == "" {
		return nil
	}
	file := filepath.Join(subjectDir, filepath.FromSlash(subject.DefinitionPath))
	agentsDir := filepath.Join(subjectDir, filepath.FromSlash(agentsRel))

	if filepath.Clean(filepath.Dir(file)) != filepath.Clean(agentsDir) {
		return fmt.Errorf("runner: subject %q definition %q is not in the project agents directory %q, so the harness cannot resolve it",
			subject.Identity, file, agentsDir)
	}
	name, err := definitionName(file)
	if err != nil {
		return fmt.Errorf("runner: subject %q definition %q cannot be read: %w", subject.Identity, file, err)
	}
	if name != subject.Identity {
		return fmt.Errorf("runner: subject %q definition %q declares name %q, so the harness cannot resolve the subject by its identity",
			subject.Identity, file, name)
	}
	return nil
}

// definitionName returns the top-level `name:` of the Markdown definition's
// YAML frontmatter, or "" when there is none.
func definitionName(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	text = bytes.TrimPrefix(text, []byte("\xef\xbb\xbf"))
	if !bytes.HasPrefix(text, []byte("---\n")) {
		return "", nil
	}
	rest := text[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return "", nil
	}
	var doc map[string]any
	if err := goyaml.Unmarshal(rest[:end+1], &doc); err != nil {
		return "", fmt.Errorf("parsing frontmatter: %w", err)
	}
	if v, present := doc["name"]; present && v != nil {
		return fmt.Sprint(v), nil
	}
	return "", nil
}
