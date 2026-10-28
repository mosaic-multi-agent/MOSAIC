// Package settings implements the domain.SettingsStore port. It owns the
// external YAML analysis settings file end to end: loading the write-class and
// user-interaction tool lists and the detected-compaction threshold with
// built-in defaults for anything absent, reporting invalid entries as findings
// rather than aborting the load, and persisting a new user-interaction tool
// entry. The file lives next to the pricing file under the MOSAIC runtime root.
package settings
