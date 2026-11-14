package domain

// IsBundleLevelHook reports whether the entry is a hook record written before hook files were
// recorded one by one: a single entry per bundle that carries the hash of empty content.
func (e ManifestEntry) IsBundleLevelHook() bool {
	return e.Ref.Kind == ArtifactHook && e.ContentHash == EmptyContentHash
}
