package domain

// OwnedContent is the record of the Deploy-owned content of one deployed file.
type OwnedContent struct {
	Hash    string       // "sha256:<lowercase hex>" over the normalised Deploy-owned content
	Regions []RegionHash // agents only: top-level body regions in document order; nil for skills and hooks
}

// RegionHash is the hash of the Deploy-owned content of one top-level body region.
type RegionHash struct {
	Name string // boundary tag name; OutsideRegionsName for body text outside any top-level region
	Hash string
}

// OutsideRegionsName names the body text that lies outside every top-level region.
const OutsideRegionsName = "(outside regions)"
