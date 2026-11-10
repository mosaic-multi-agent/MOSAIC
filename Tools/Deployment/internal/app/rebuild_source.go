package app

// rebuild_source.go reads the source agent of a rebuild through the decode funnel, so a
// source in a non-Markdown format (Codex TOML) is rebuilt from its canonical Markdown form.

import (
	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// decodeRebuildSource decodes the source file of a rebuild with the source harness's
// descriptor and parses the canonical result. A source that cannot be decoded fails with the
// funnel's decode error, which wraps agentformat.ErrMalformedDeployed. It returns the
// canonical bytes alongside the parsed document.
func decodeRebuildSource(in RebuildInput) ([]byte, *docformat.Document, error) {
	read := decodeDeployedBytes(in.Source, domain.ArtifactAgent, in.SourceModule.Descriptor())
	if read.DecodeErr != nil {
		return nil, nil, read.DecodeErr
	}
	doc, err := ParseGenericSource(read.Canonical)
	if err != nil {
		return nil, nil, err
	}
	return read.Canonical, doc, nil
}
