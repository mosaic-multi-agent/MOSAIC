package identity

import (
	"fmt"

	"github.com/goccy/go-yaml/ast"

	"mosaic-deploy/internal/domain"
)

// Orchestrator agent keys cannot be declared as agent overrides: the pair is one target.
var orchestratorAgentKeys = map[string]bool{"orchestrator": true, "orchestrator-script": true}

// Values accepted under overrides.system_files.
var systemFileNames = map[string]bool{"CommunicationProtocol.md": true, "DeployedSections.md": true}

// keyedTargets maps an overrides list key to the catalog item kind its entries declare.
var keyedTargets = map[string]domain.CatalogItemKind{
	"agents":             domain.CatalogItemAgent,
	"workflows":          domain.CatalogItemWorkflow,
	"skills":             domain.CatalogItemSkill,
	"hooks":              domain.CatalogItemHook,
	"harness_injections": domain.CatalogItemHarnessInjections,
	"system_files":       domain.CatalogItemSystemFile,
}

// parseOverrides reads the optional overrides block. An absent or null block declares nothing.
func parseOverrides(node ast.Node) (Overrides, error) {
	if node == nil {
		return Overrides{}, nil
	}
	if _, isNull := node.(*ast.NullNode); isNull {
		return Overrides{}, nil
	}
	entries, err := mappingEntries(node)
	if err != nil {
		return Overrides{}, fmt.Errorf("overrides must be a mapping")
	}
	var items []domain.CatalogItem
	for key, value := range entries {
		if key == "orchestrator_pair" {
			declared, err := parsePairFlag(value)
			if err != nil {
				return Overrides{}, err
			}
			if declared {
				items = append(items, domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair})
			}
			continue
		}
		kind, ok := keyedTargets[key]
		if !ok {
			return Overrides{}, fmt.Errorf("unknown override target %q", key)
		}
		keys, err := parseKeyList(key, value)
		if err != nil {
			return Overrides{}, err
		}
		for _, k := range keys {
			if err := checkEntry(kind, k); err != nil {
				return Overrides{}, err
			}
			items = append(items, domain.CatalogItem{Kind: kind, Key: k})
		}
	}
	return NewOverrides(items...), nil
}

func parsePairFlag(node ast.Node) (bool, error) {
	b, ok := node.(*ast.BoolNode)
	if !ok {
		return false, fmt.Errorf("overrides.orchestrator_pair must be true or false")
	}
	return b.Value, nil
}

func parseKeyList(target string, node ast.Node) ([]string, error) {
	if _, isNull := node.(*ast.NullNode); isNull {
		return nil, nil
	}
	seq, ok := node.(*ast.SequenceNode)
	if !ok {
		return nil, fmt.Errorf("overrides.%s must be a list", target)
	}
	keys := make([]string, 0, len(seq.Values))
	for _, v := range seq.Values {
		text, present, err := scalarText(v)
		if err != nil || !present || text == "" {
			return nil, fmt.Errorf("overrides.%s entries must be non-empty scalars", target)
		}
		keys = append(keys, text)
	}
	return keys, nil
}

func checkEntry(kind domain.CatalogItemKind, key string) error {
	switch {
	case kind == domain.CatalogItemAgent && orchestratorAgentKeys[key]:
		return fmt.Errorf("overrides.agents entry %q is invalid: the orchestrator pair is declared with orchestrator_pair: true", key)
	case kind == domain.CatalogItemSystemFile && !systemFileNames[key]:
		return fmt.Errorf("overrides.system_files entry %q is not CommunicationProtocol.md or DeployedSections.md", key)
	}
	return nil
}
