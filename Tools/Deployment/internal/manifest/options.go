package manifest

// HarnessPaths maps a harness id to the workspace-relative directories it deploys into (slash
// form, no trailing slash), for example "ghcp-cli": {".github/agents", ".github/skills",
// ".github/hooks"}. The old-format upgrade uses it to attribute entries outside the fixed
// .claude/ and .opencode/ prefixes.
type HarnessPaths map[string][]string

type storeOptions struct {
	harnessPaths HarnessPaths
}

// StoreOption configures NewStore.
type StoreOption func(*storeOptions)

// WithHarnessPaths gives the store the path knowledge the old-format upgrade needs.
func WithHarnessPaths(p HarnessPaths) StoreOption {
	return func(o *storeOptions) { o.harnessPaths = p }
}
