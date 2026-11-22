package codextoml

import (
	"fmt"
	"sort"

	"github.com/pelletier/go-toml/v2"

	"mosaic-common/agentformat"
)

// ReadSettings returns every top-level key of the deployed Codex TOML file with its
// decoded value, in the order the keys first appear in the file. It is the read-only
// counterpart of Decode for settings the canonical form carries only as key names
// (tables, arrays, floats, commented keys). The stamp block is made of comments and
// is therefore not a setting. The input is never modified.
func (codexTranslator) ReadSettings(deployed []byte) ([]agentformat.Setting, error) {
	var values map[string]interface{}
	if err := toml.Unmarshal(deployed, &values); err != nil {
		return nil, &agentformat.ArtifactError{
			Phase: "decode",
			Err:   fmt.Errorf("%w: %s", agentformat.ErrMalformedDeployed, err.Error()),
		}
	}

	settings := make([]agentformat.Setting, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, rec := range scanTomlLines(deployed) {
		key := rec.topKey
		if key == "" || seen[key] {
			continue
		}
		value, ok := values[key]
		if !ok {
			continue
		}
		seen[key] = true
		settings = append(settings, agentformat.Setting{Key: key, Value: value})
	}

	// Keys the line scan could not attribute (unusual key spellings) follow in
	// sorted order so that no declared key is dropped.
	var rest []string
	for key := range values {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		settings = append(settings, agentformat.Setting{Key: key, Value: values[key]})
	}
	return settings, nil
}
