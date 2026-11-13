package engine_test

// overrideOf returns the NextInput.LastOutputsOverride value for paths: a
// non-nil override that is used verbatim (a non-nil empty list injects nothing).
func overrideOf(paths []string) *[]string { return &paths }
