package app

import "mosaic-log-analyzer/internal/domain"

// PricingPaths reports the global and project pricing files.
func (s *Service) PricingPaths() domain.ConfigPaths {
	return s.deps.Pricing.Paths()
}

// SettingsPaths reports the global and project settings files; the zero value
// when no settings store is configured.
func (s *Service) SettingsPaths() domain.ConfigPaths {
	if s.deps.Settings == nil {
		return domain.ConfigPaths{}
	}
	return s.deps.Settings.Paths()
}
