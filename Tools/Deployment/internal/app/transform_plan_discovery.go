package app

// transform_plan_discovery.go recognises MOSAIC agents among the files of a source agents
// folder for transform planning. Recognition is wider than the harness-only rule used by
// update and promote: canonical sections are not required, so deployed utility, standalone
// and harness-only agents are accepted while plain markdown and deploy TODO files are not.

import (
	"os"
	"path/filepath"
	"strings"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
)

// agentFileExtPlain and agentFileExtAgent are the two agent file extensions.
const (
	agentFileExtPlain = ".md"
	agentFileExtAgent = ".agent.md"
)

// fileAgentExt returns ".agent.md" when name ends in it, otherwise ".md".
func fileAgentExt(name string) string {
	if strings.HasSuffix(name, agentFileExtAgent) {
		return agentFileExtAgent
	}
	return agentFileExtPlain
}

// recognizeTransformAgent reports whether data is a MOSAIC agent. When it is not, reason names
// the failed condition.
func recognizeTransformAgent(data []byte) (ok bool, reason string) {
	doc, err := docformat.Parse(data)
	if err != nil {
		return false, "document could not be parsed: " + err.Error()
	}
	fm := doc.Frontmatter()
	if !fm.Present() {
		return false, "document has no frontmatter block"
	}
	_, stamped := agentfields.ReadVersionStamp(func(key string) string {
		if v, found := fm.Get(key); found && v.Kind == domain.KindScalar {
			return v.Scalar
		}
		return ""
	})
	if !stamped {
		return false, "frontmatter does not carry a non-empty MOSAIC version stamp (checked: " +
			strings.Join(agentfields.VersionStampKeys(), ", ") + ")"
	}
	for _, issue := range docformat.Validate(doc, docformat.ValidateOptions{}) {
		if blockingValidationCodes[issue.Code] {
			return false, "body has blocking structural issue (" + issue.Code + "): " + issue.Message
		}
	}
	return true, ""
}

// sourceAgentFile is one .md file of the source agents folder.
type sourceAgentFile struct {
	name       string
	path       string
	recognised bool
	reason     string // why the file is not an agent
	model      string
}

// listSourceAgentFiles reads the agents folder without recursing. It returns the .md files in
// name order with their recognition result, and the lower-cased workspace-relative slash
// paths of every regular file in the folder. A missing folder yields no files and no error.
func listSourceAgentFiles(dir, relDir string, src domain.HarnessModule) ([]sourceAgentFile, map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var files []sourceAgentFile
	allFiles := map[string]bool{}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		allFiles[strings.ToLower(slashJoin(relDir, e.Name()))] = true
		if !strings.HasSuffix(e.Name(), agentFileExtPlain) {
			continue
		}
		f := sourceAgentFile{name: e.Name(), path: filepath.Join(dir, e.Name())}
		data, rerr := os.ReadFile(f.path)
		if rerr != nil {
			f.reason = "file could not be read: " + rerr.Error()
		} else if ok, why := recognizeTransformAgent(data); ok {
			f.recognised = true
			f.model = ReadSourceModel(data, *src.Descriptor())
		} else {
			f.reason = why
		}
		files = append(files, f)
	}
	return files, allFiles, nil
}
