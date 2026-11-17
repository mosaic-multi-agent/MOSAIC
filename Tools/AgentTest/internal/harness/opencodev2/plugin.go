package opencodev2

// Interceptor plugin generation. The generated module is checked by running it
// under a fake host (see the plugin tests); a live V2 runtime is not needed to
// build it.

import (
	"encoding/json"
	"fmt"
	"strings"
)

// generatePlugin renders the interceptor plugin for the three phase bridges.
// Pure over its inputs: equal bridges give byte-identical content.
func generatePlugin(pre, post, completion bridge) ([]byte, error) {
	literals := make([]string, 0, 6)
	for _, b := range []bridge{pre, post, completion} {
		lit, err := bridgeLiteral(b)
		if err != nil {
			return nil, err
		}
		literals = append(literals, lit)
	}
	out := strings.NewReplacer("__PRE__", literals[0], "__POST__", literals[1], "__COMPLETION__", literals[2]).Replace(pluginTemplate)
	return []byte(out), nil
}

// bridgeLiteral encodes a bridge as the JSON object literal embedded in the
// module.
func bridgeLiteral(b bridge) (string, error) {
	exec, err := json.Marshal(b.executable)
	if err != nil {
		return "", fmt.Errorf("opencode-v2: encoding bridge executable: %w", err)
	}
	args, err := json.Marshal(b.args)
	if err != nil {
		return "", fmt.Errorf("opencode-v2: encoding bridge args: %w", err)
	}
	return `{ executable: ` + string(exec) + `, args: ` + string(args) + ` }`, nil
}
