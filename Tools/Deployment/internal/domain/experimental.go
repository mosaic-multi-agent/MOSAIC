package domain

import "strings"

// ExperimentalLabel is the marker word shown next to an experimental harness in pickers.
const ExperimentalLabel = "Experimental"

// ExperimentalHarness is one experimental harness an operation involves, with the
// user-facing reason its descriptor declares.
type ExperimentalHarness struct {
	HarnessID   string `json:"harnessId"`
	DisplayName string `json:"displayName"`
	Reason      string `json:"reason"`
}

// IsExperimental reports whether the harness descriptor declares the harness experimental.
func (r HarnessRef) IsExperimental() bool { return r.ExperimentalReason != "" }

// Message returns the single user-facing warning sentence shown on every surface: the display
// name, "is experimental:" and the reason with every run of whitespace collapsed to one space.
func (e ExperimentalHarness) Message() string {
	return e.DisplayName + " is experimental: " + strings.Join(strings.Fields(e.Reason), " ")
}

// ExperimentalHarnessesOf returns one ExperimentalHarness per experimental ref, in argument
// order, skipping refs that are not experimental and later refs whose ID was already
// reported. It returns nil when no ref is experimental.
func ExperimentalHarnessesOf(refs ...HarnessRef) []ExperimentalHarness {
	var out []ExperimentalHarness
	seen := map[string]bool{}
	for _, ref := range refs {
		if !ref.IsExperimental() || seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		name := ref.DisplayName
		if name == "" {
			name = ref.ID
		}
		out = append(out, ExperimentalHarness{HarnessID: ref.ID, DisplayName: name, Reason: ref.ExperimentalReason})
	}
	return out
}

// HarnessOption returns the pick-list option for a harness: its display name (or id), marked
// with ExperimentalLabel and described by the reason when the harness is experimental.
// Disabled state is left to the caller.
func HarnessOption(ref HarnessRef) Option {
	label := ref.DisplayName
	if label == "" {
		label = ref.ID
	}
	opt := Option{ID: ref.ID, Label: ref.DisplayName}
	if ref.IsExperimental() {
		opt.Label = label + " [" + ExperimentalLabel + "]"
		opt.Description = ref.ExperimentalReason
	}
	return opt
}
