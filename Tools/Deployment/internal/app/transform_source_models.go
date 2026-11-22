package app

// transform_source_models.go reads the source model of a deployed agent, which groups the
// agents of a transform run for the model questions.

import (
	"mosaic-deploy/internal/domain"
)

// UnsetSourceModel is the mapping key for the group of agents whose source file
// carries no model at all. It is the empty string so that a zero-value mapping
// lookup and an explicit "unset" lookup are the same operation.
const UnsetSourceModel = ""

// ReadSourceModel extracts the source harness's model identifier from an agent
// file's frontmatter. The content is decoded through the decode funnel with srcDesc first,
// so a source in a non-Markdown format is read from its canonical form. Resolution order:
//
//  1. srcDesc.Frontmatter.ModelKey, when that key is declared and present as a
//     non-empty scalar.
//  2. The generic "model" key, when present as a non-empty scalar.
//  3. Otherwise UnsetSourceModel.
//
// Undecodable or unparseable content yields UnsetSourceModel and no error: a file that
// cannot be read is classified by the planning step, not here.
func ReadSourceModel(content []byte, srcDesc domain.HarnessDescriptor) string {
	if len(content) == 0 {
		return UnsetSourceModel
	}
	read := decodeDeployedBytes(content, domain.ArtifactAgent, &srcDesc)
	if read.DecodeErr != nil {
		return UnsetSourceModel
	}
	return readCanonicalSourceModel(read.Canonical, &srcDesc)
}

// readCanonicalSourceModel reads the source model from canonical (already decoded) bytes.
func readCanonicalSourceModel(canonical []byte, srcDesc *domain.HarnessDescriptor) string {
	doc, err := ParseGenericSource(canonical)
	if err != nil {
		return UnsetSourceModel
	}
	fm := doc.Frontmatter()

	if srcDesc.Frontmatter.ModelKey != "" {
		if v, ok := fm.Get(srcDesc.Frontmatter.ModelKey); ok {
			if v.Kind == domain.KindScalar && v.Scalar != "" {
				return v.Scalar
			}
		}
	}

	if v, ok := fm.Get("model"); ok {
		if v.Kind == domain.KindScalar && v.Scalar != "" {
			return v.Scalar
		}
	}

	return UnsetSourceModel
}
