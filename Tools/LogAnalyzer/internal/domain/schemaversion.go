package domain

import "strings"

// splitSchemaVersion splits "MAJOR.MINOR.PATCH" into its three components,
// each of one or more ASCII digits and nothing else. ok is false otherwise.
func splitSchemaVersion(v string) (major, minor, patch string, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "", "", "", false
	}
	for i, p := range parts {
		if p == "" {
			return "", "", "", false
		}
		for j := 0; j < len(p); j++ {
			if p[j] < '0' || p[j] > '9' {
				return "", "", "", false
			}
		}
		parts[i] = strings.TrimLeft(p, "0") // decimal value; "" is zero
	}
	return parts[0], parts[1], parts[2], true
}

// IsSupportedSchemaVersion reports whether v is "MAJOR.MINOR.PATCH" (three
// components of ASCII digits, nothing else) whose MAJOR is SupportedSchemaMajor.
// Components have no size limit and leading zeros are allowed.
func IsSupportedSchemaVersion(v string) bool {
	major, _, _, ok := splitSchemaVersion(v)
	return ok && major == "1"
}

// SchemaVersionBelow150 reports whether v is a supported version below 1.5.0,
// the versions that log compactions without a phase. An unsupported version
// is never below 1.5.0.
func SchemaVersionBelow150(v string) bool {
	major, minor, _, ok := splitSchemaVersion(v)
	if !ok || major != "1" {
		return false
	}
	// 1.5.0 is the lowest patch of minor 5, so only a smaller minor is below.
	return minor == "" || (len(minor) == 1 && minor[0] < '5')
}
