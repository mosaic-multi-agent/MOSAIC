package main

// deploy_imports_test.go verifies that the import-boundary guard keeps internal/deploy free
// of agent-format knowledge: no import of internal/agentformat or any package below it
// (including formatid). The guard covers imports only; it does not detect format IDs written
// as string literals.

import (
	"path/filepath"
	"testing"
)

func TestDeployImports_AgentformatImport_IsRejected(t *testing.T) {
	assertGuardRejectsFixture(t,
		filepath.Join("testdata", "deploy_imports", "agentformat_import_in_deploy"),
		"internal/deploy importing internal/agentformat should be rejected: deploy is format-blind",
	)
}

func TestDeployImports_FormatidImport_IsRejected(t *testing.T) {
	assertGuardRejectsFixture(t,
		filepath.Join("testdata", "deploy_imports", "formatid_import_in_deploy"),
		"internal/deploy importing internal/agentformat/formatid should be rejected: deploy is format-blind",
	)
}

func TestDeployImports_AgentformatSubpackageImport_IsRejected(t *testing.T) {
	assertGuardRejectsFixture(t,
		filepath.Join("testdata", "deploy_imports", "agentformat_subpackage_import_in_deploy"),
		"internal/deploy importing a package below internal/agentformat should be rejected",
	)
}

func TestDeployImports_PermittedImports_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t,
		filepath.Join("testdata", "deploy_imports", "permitted_imports"),
		"internal/deploy importing domain, manifest and transform should be accepted",
	)
}
