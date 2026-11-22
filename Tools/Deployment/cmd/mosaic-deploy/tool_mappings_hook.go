package main

// tool_mappings_hook.go supplies the registry ToolMappings hook of the composition root.

import (
	"runtime"

	commonharness "mosaic-common/harness"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// registryOptions returns the options the composition root passes to harness discovery: the
// mosaic root, the external-module opt-in, and the ToolMappings hook of toolMappingsHook over the
// given stores, so main uses the refreshing hook and not a one-time merge of the startup configs.
func registryOptions(
	mosaicRoot string, allowExternal bool,
	toolStore config.ToolConfigStore, userStore config.UserConfigStore,
	startupTool config.ToolConfig, startupUser config.UserConfig,
) registry.Options {
	return registry.Options{
		MosaicRoot:    mosaicRoot,
		AllowExternal: allowExternal,
		GOOS:          runtime.GOOS,
		ToolMappings:  toolMappingsHook(toolStore, userStore, startupTool, startupUser),
		// The ModelCatalog hook overlays the shared catalog's descriptor-derived model data onto
		// every harness the catalog knows; a harness it does not know keeps its own catalog.
		ModelCatalog: func(harnessID string, _ domain.ModelCatalog) domain.ModelCatalog {
			cat, ok := commonharness.LookupModelCatalog(harnessID)
			if !ok {
				return domain.ModelCatalog{}
			}
			return domain.ModelCatalog{IDs: cat.IDs, FormatHint: cat.FormatHint}
		},
	}
}

// toolMappingsHook returns the registry ToolMappings hook. At each invocation it loads
// tool-config.yaml and user-config.yaml from the stores and merges their tool_destinations for the
// harness with the declared mappings (config.EffectiveToolMappings). When a store fails to load,
// the matching startup config is used instead.
func toolMappingsHook(
	toolStore config.ToolConfigStore, userStore config.UserConfigStore,
	startupTool config.ToolConfig, startupUser config.UserConfig,
) func(harnessID string, declared []domain.ToolMapping) []domain.ToolMapping {
	return func(harnessID string, declared []domain.ToolMapping) []domain.ToolMapping {
		toolCfg, err := toolStore.Load()
		if err != nil {
			toolCfg = startupTool
		}
		userCfg, err := userStore.Load()
		if err != nil {
			userCfg = startupUser
		}
		return config.EffectiveToolMappings(harnessID, declared, toolCfg.ToolDestinations, userCfg.ToolDestinations)
	}
}
