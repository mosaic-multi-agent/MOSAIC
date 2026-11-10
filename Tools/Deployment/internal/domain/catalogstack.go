package domain

// Catalog-stack carriers are grouped in this file. They are data only (no behaviour) and are
// shared by override declarations, provenance, shadow records, the conflict report and the TUI.
//
// JSON tags (a deliberate deviation): internal/domain has no json tags elsewhere. The carriers in
// this file carry lowerCamel tags because they are nested inside lowerCamel-tagged app result
// types. Do not "align" either side.

// CatalogItemKind names the kind of a catalog item.
type CatalogItemKind string

const (
	CatalogItemAgent             CatalogItemKind = "agent"
	CatalogItemWorkflow          CatalogItemKind = "workflow"
	CatalogItemSkill             CatalogItemKind = "skill"
	CatalogItemHook              CatalogItemKind = "hook"
	CatalogItemOrchestratorPair  CatalogItemKind = "orchestrator-pair"
	CatalogItemSystemFile        CatalogItemKind = "system-file"
	CatalogItemHarnessInjections CatalogItemKind = "harness-injections"
)

// CatalogItem identifies one item of a catalog by kind and key. Key convention per kind:
//
//	agent              the agent key, for every role, including orchestrator and orchestrator-script
//	workflow           the workflow id
//	skill              the skill key (folder name)
//	hook               the hook bundle key
//	orchestrator-pair  "" (one item for both orchestrator files)
//	system-file        the file name: CommunicationProtocol.md or DeployedSections.md
//	harness-injections the harness directory name under HarnessInjections/ (for example Claude Code)
type CatalogItem struct {
	Kind CatalogItemKind `json:"kind"`
	Key  string          `json:"key,omitempty"`
}

// CatalogIdentity is a catalog's self-description: its id, display name and version. It is read
// from the catalog's identity file and travels with provenance, issue attribution and manifests.
type CatalogIdentity struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version"`
}

// CatalogLayer is one layer of a resolved catalog stack: the catalog identity, the catalog root
// and whether it is the MOSAIC default layer. It is also the provenance carrier: "which layer
// supplied this item" is answered with a CatalogLayer.
type CatalogLayer struct {
	Identity CatalogIdentity `json:"identity"`
	Root     string          `json:"root"`
	Default  bool            `json:"default"`
}

// PersistedCatalogLayer is one layer of the catalog stack persisted in the workspace manifest.
// Deliberately untagged, unlike CatalogIdentity and CatalogLayer: it is never JSON-encoded (it
// is not in any CLI document or app result), and its only wire form is the manifest YAML, which
// the store's private wire types own. Do not add tags.
type PersistedCatalogLayer struct {
	Path      string // absolute cleaned root; "" when Default
	CatalogID string // stored identity id; "" when Default
	Default   bool
}
