package identity_test

import (
	"errors"
	"testing"

	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/domain"
)

func TestRead_DeclaresEveryTargetKind(t *testing.T) {
	root := writeIdentityFile(t, withHeader(`overrides:
  agents: [code-reviewer, planner]
  workflows: [feature-dev]
  skills: [efficient-file-reading]
  hooks: [pre-commit-guard]
  orchestrator_pair: true
  harness_injections: ["Claude Code", "OpenCode"]
  system_files: [CommunicationProtocol.md, DeployedSections.md]
`))

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	declared := []domain.CatalogItem{
		item(domain.CatalogItemAgent, "code-reviewer"),
		item(domain.CatalogItemAgent, "planner"),
		item(domain.CatalogItemWorkflow, "feature-dev"),
		item(domain.CatalogItemSkill, "efficient-file-reading"),
		item(domain.CatalogItemHook, "pre-commit-guard"),
		item(domain.CatalogItemOrchestratorPair, ""),
		item(domain.CatalogItemHarnessInjections, "Claude Code"),
		item(domain.CatalogItemHarnessInjections, "OpenCode"),
		item(domain.CatalogItemSystemFile, "CommunicationProtocol.md"),
		item(domain.CatalogItemSystemFile, "DeployedSections.md"),
	}
	for _, it := range declared {
		if !got.Overrides.Declares(it) {
			t.Errorf("expected %+v to be declared", it)
		}
	}
	if n := len(got.Overrides.Items()); n != len(declared) {
		t.Errorf("Items() len = %d, want %d", n, len(declared))
	}
}

func TestRead_DeclarationIsSpecificToKindAndKey(t *testing.T) {
	root := writeIdentityFile(t, withHeader(`overrides:
  agents: [shared-name]
  harness_injections: ["Claude Code"]
  system_files: [CommunicationProtocol.md]
`))

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !got.Overrides.Declares(item(domain.CatalogItemAgent, "shared-name")) {
		t.Error("expected agent shared-name to be declared")
	}
	notDeclared := []domain.CatalogItem{
		item(domain.CatalogItemSkill, "shared-name"),
		item(domain.CatalogItemWorkflow, "shared-name"),
		item(domain.CatalogItemAgent, "other"),
		item(domain.CatalogItemHarnessInjections, "OpenCode"),
		item(domain.CatalogItemSystemFile, "DeployedSections.md"),
		item(domain.CatalogItemOrchestratorPair, ""),
	}
	for _, it := range notDeclared {
		if got.Overrides.Declares(it) {
			t.Errorf("did not expect %+v to be declared", it)
		}
	}
}

func TestRead_OrchestratorPairFalseDeclaresNothing(t *testing.T) {
	root := writeIdentityFile(t, withHeader("overrides:\n  orchestrator_pair: false\n"))

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Overrides.Declares(item(domain.CatalogItemOrchestratorPair, "")) {
		t.Error("orchestrator_pair: false must not declare the pair")
	}
}

func TestRead_OrchestratorPairIsOneItemNotTwoAgents(t *testing.T) {
	root := writeIdentityFile(t, withHeader("overrides:\n  orchestrator_pair: true\n"))

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	assertItems(t, got.Overrides.Items(), []domain.CatalogItem{item(domain.CatalogItemOrchestratorPair, "")})
	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if got.Overrides.Declares(item(domain.CatalogItemAgent, key)) {
			t.Errorf("pair declaration must not declare agent %q", key)
		}
	}
}

func TestRead_ItemsSortedByKindThenKey(t *testing.T) {
	root := writeIdentityFile(t, withHeader(`overrides:
  workflows: [zeta, alpha]
  skills: [s1]
  agents: [b, a]
`))

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	assertItems(t, got.Overrides.Items(), []domain.CatalogItem{
		item(domain.CatalogItemAgent, "a"),
		item(domain.CatalogItemAgent, "b"),
		item(domain.CatalogItemSkill, "s1"),
		item(domain.CatalogItemWorkflow, "alpha"),
		item(domain.CatalogItemWorkflow, "zeta"),
	})
}

func TestRead_RejectsMalformedOverrides(t *testing.T) {
	cases := map[string]string{
		"unknown target kind":            "overrides:\n  commands: [x]\n",
		"unknown system file":            "overrides:\n  system_files: [Other.md]\n",
		"agents is a scalar":             "overrides:\n  agents: code-reviewer\n",
		"workflows is a map":             "overrides:\n  workflows: {a: b}\n",
		"harness injections is a scalar": "overrides:\n  harness_injections: Claude Code\n",
		"orchestrator agent key":         "overrides:\n  agents: [orchestrator]\n",
		"orchestrator-script agent key":  "overrides:\n  agents: [code-reviewer, orchestrator-script]\n",
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeIdentityFile(t, withHeader(block))

			_, err := identity.Read(root)

			if !errors.Is(err, identity.ErrOverrideInvalid) {
				t.Fatalf("expected ErrOverrideInvalid, got %v", err)
			}
			assertContains(t, err.Error(), root)
		})
	}
}

func TestRead_OrchestratorAgentRejectionPointsToPairForm(t *testing.T) {
	root := writeIdentityFile(t, withHeader("overrides:\n  agents: [orchestrator]\n"))

	_, err := identity.Read(root)

	if !errors.Is(err, identity.ErrOverrideInvalid) {
		t.Fatalf("expected ErrOverrideInvalid, got %v", err)
	}
	assertContains(t, err.Error(), "orchestrator_pair")
}
