// Package testjson is a test-only agent format whose on-disk form is a JSON object,
// deliberately unlike Markdown and TOML. It is pure and is never imported by the
// production wiring package (internal/agentformat/all).
package testjson

import (
	"errors"

	"mosaic-common/docformat"
	"mosaic-common/mosaic"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/agentformat/formatid"
)

// ID is the format id of the test-only JSON agent format. It is outside formatid.All().
const ID formatid.ID = "mosaic-test-json"

// Register registers the translator under ID through agentformat.RegisterTestOnly.
// Idempotent. Importing the package does not register anything (no init).
func Register() {
	agentformat.RegisterTestOnly(ID, translator{})
}

// translator is the test-only JSON translator. Reached only through the runtime lookup.
type translator struct{}

// Encode writes the canonical document as the JSON on-disk form. On update it copies the
// native member of the prior deployed bytes when those decode; carriage containers are
// never written and each held key is reported as stripped.
func (translator) Encode(canonical []byte, ctx agentformat.ArtifactContext) ([]byte, agentformat.Report, error) {
	if err := checkOperation(ctx); err != nil {
		return nil, agentformat.Report{}, err
	}
	doc, err := docformat.Parse(canonical)
	if err != nil {
		return nil, agentformat.Report{}, &agentformat.ArtifactError{Phase: "encode", Err: err}
	}
	_, body, err := docformat.SplitFrontmatter(canonical)
	if err != nil {
		return nil, agentformat.Report{}, &agentformat.ArtifactError{Phase: "encode", Err: err}
	}

	var report agentformat.Report
	var meta []mosaic.FieldPair
	fm := doc.Frontmatter()
	for _, key := range fm.Keys() {
		value, _ := fm.Get(key)
		if agentformat.IsCarriageKey(key) {
			report.Entries = append(report.Entries, strippedEntries(key, value)...)
			continue
		}
		meta = append(meta, mosaic.FieldPair{Key: key, Value: value})
	}

	file := fileContent{instructions: string(body), meta: meta}
	if ctx.Op == agentformat.OpUpdate {
		if prior, perr := decodeFile(ctx.PriorDeployed); perr == nil && prior.hasNative {
			file.hasNative = true
			file.native = prior.native
		}
	}
	return writeFile(file), report, nil
}

// Decode reads the JSON on-disk form and returns the canonical Markdown document.
// The native member is never represented in the canonical form.
func (translator) Decode(deployed []byte, _ agentformat.ArtifactContext) ([]byte, agentformat.Report, error) {
	file, err := decodeFile(deployed)
	if err != nil {
		return nil, agentformat.Report{}, err
	}
	return renderCanonical(file), agentformat.Report{}, nil
}

// checkOperation enforces the create/update contract of the prior-bytes channel.
func checkOperation(ctx agentformat.ArtifactContext) error {
	switch ctx.Op {
	case agentformat.OpCreate:
		if ctx.PriorDeployed != nil {
			return &agentformat.ArtifactError{Phase: "encode", Err: agentformat.ErrUnspecifiedOperation}
		}
	case agentformat.OpUpdate:
		if ctx.PriorDeployed == nil {
			return &agentformat.ArtifactError{Phase: "encode", Err: agentformat.ErrMissingPriorBytes}
		}
	default:
		return &agentformat.ArtifactError{Phase: "encode", Err: agentformat.ErrUnspecifiedOperation}
	}
	return nil
}

// strippedEntries reports every user key held in a carriage container key.
func strippedEntries(containerKey string, value mosaic.FieldValue) []agentformat.ReportEntry {
	var keys []string
	switch value.Kind {
	case mosaic.KindMapping:
		for _, p := range value.Pairs {
			keys = append(keys, p.Key)
		}
	case mosaic.KindList:
		for _, item := range value.Items {
			if item.Kind == mosaic.KindScalar {
				keys = append(keys, item.Scalar)
			}
		}
	}
	entries := make([]agentformat.ReportEntry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, agentformat.ReportEntry{
			Kind:   agentformat.EntryStrippedContainer,
			Key:    k,
			Reason: "test JSON format cannot carry user TOML keys; the " + containerKey + " container was not written",
		})
	}
	return entries
}

// malformed builds the decode error for a file that is not a test-format agent.
func malformed(key string, detail string) error {
	return &agentformat.ArtifactError{
		Phase: "decode",
		Key:   key,
		Err:   errors.Join(agentformat.ErrMalformedDeployed, errors.New(detail)),
	}
}
