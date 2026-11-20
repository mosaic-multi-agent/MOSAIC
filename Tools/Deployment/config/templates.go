// Package defaultconfig embeds the commented config templates shipped with mosaic-deploy,
// so the binary can seed a missing MosaicDeploy/config/ from them without needing the
// source tree next to it.
package defaultconfig

import "embed"

// Templates holds user-config.yaml and tool-config.yaml at its root.
//
//go:embed user-config.yaml tool-config.yaml
var Templates embed.FS
