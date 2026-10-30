package domain

// ConfigLevel names where a configuration value comes from.
type ConfigLevel uint8

const (
	LevelBuiltIn ConfigLevel = iota // not from a file
	LevelGlobal
	LevelProject
)

// String returns "built-in", "global" or "project".
func (l ConfigLevel) String() string {
	switch l {
	case LevelGlobal:
		return "global"
	case LevelProject:
		return "project"
	default:
		return "built-in"
	}
}

// ConfigFile describes one configuration file of one level.
type ConfigFile struct {
	Level       ConfigLevel
	Path        string // absolute; "" when Unavailable is set
	Exists      bool
	Unavailable string // reason the level cannot be used; "" when usable
	// Currency is the currency a pricing file states ("" when absent, unavailable,
	// unparseable or without the key; USD for an invalid value). Pricing files only.
	Currency CurrencyCode
}

// ConfigPaths reports both levels of one configuration store.
type ConfigPaths struct {
	Global  ConfigFile
	Project ConfigFile
}
