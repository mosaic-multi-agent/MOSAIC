package preflight

// Diagnostic codes added by the model guards. They are stable identifiers
// consumed by frontends and tests.
const (
	// DiagMissingSubjectModel: no subject model was selected.
	DiagMissingSubjectModel = "missing-subject-model"
	// DiagInvalidCatalogTier: a catalog agent declares a recommended_tier
	// outside the AgentTest tier vocabulary.
	DiagInvalidCatalogTier = "invalid-catalog-tier"
	// DiagCatalogTierUnverifiable: the catalog tiers cannot be checked.
	DiagCatalogTierUnverifiable = "catalog-tier-unverifiable"
	// DiagUnresolvedTierModel: the strict dry-run deploy left an agent
	// without a model.
	DiagUnresolvedTierModel = "unresolved-tier-model"
)
