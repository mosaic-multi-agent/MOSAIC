package config

import (
	"fmt"

	"mosaic-deploy/internal/domain"
)

// CustomToolMapping returns the tool_destinations entry that reproduces a custom answer's
// resolution: Generic = res.Generic, Destinations = a deep copy of res.Destinations. It
// returns an error when res.Outcome != domain.ToolCustom or res has no destinations.
func CustomToolMapping(res domain.ToolResolution) (domain.ToolMapping, error) {
	if res.Outcome != domain.ToolCustom {
		return domain.ToolMapping{}, fmt.Errorf("tool %q was not resolved from a custom answer (outcome %q)", res.Generic, res.Outcome)
	}
	if len(res.Destinations) == 0 {
		return domain.ToolMapping{}, fmt.Errorf("custom answer for tool %q has no destinations", res.Generic)
	}
	return deepCopyMapping(domain.ToolMapping{Generic: res.Generic, Destinations: res.Destinations}), nil
}

// SaveToolDestination puts mapping for harnessID under store's lock
// (WithLock -> PutToolDestination). Errors from the lock or the put are returned unchanged.
func SaveToolDestination(store UserConfigStore, harnessID string, mapping domain.ToolMapping) error {
	return store.WithLock(func() error {
		return store.PutToolDestination(harnessID, mapping)
	})
}
