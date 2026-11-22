package app

import (
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
)

// toolSettingsStamp loads tool-config.yaml and user-config.yaml and returns the run's
// tool_mappings_version stamp for module (config.ToolSettingsStamp over module.Descriptor().Tools
// and the loaded tool_destinations). A user-config load failure is returned and aborts the run;
// a tool-config load failure counts as no tool-config tool_destinations.
//
// Source decision: the tool spec comes from the module the run writes with. On built-in and
// descriptor-only modules its Mappings already include the configuration merged at Discover
// time; the configuration part is reloaded here, at plan time. Within one run every site sees
// the same module and stores, so all sites agree. If the configuration changes between Discover
// and plan time, the next process computes a different stamp and the next update rewrites the
// agent once, which is correct because the file was written with the Discover-time mappings.
func (s *service) toolSettingsStamp(module domain.HarnessModule) (string, error) {
	toolCfg, _ := s.deps.ToolConfig.Load()
	userCfg, err := s.deps.UserConfig.Load()
	if err != nil {
		return "", err
	}
	return config.ToolSettingsStamp(module.Ref().ID, module.Descriptor().Tools, toolCfg.ToolDestinations, userCfg.ToolDestinations), nil
}
