package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/gofrs/flock"
	"mosaic-deploy/internal/domain"
)

// lockTimeout is the maximum time WithLock waits to acquire the file lock before
// returning an error. Chosen to be long enough to survive transient contention from
// another concurrent deploy process while short enough to not stall the user's run.
const lockTimeout = 3 * time.Second

// userConfigSchemaVersion is the schema version written by this build into every
// user-config.yaml. Old-schema files are migrated to this version on load.
const userConfigSchemaVersion = "1"

// UserConfig is the per-user configuration stored in
// MosaicDeploy/config/user-config.yaml. It holds state that is personal to the operator
// and should NOT be committed to the MOSAIC repository.
//
// Per-agent model selections are not stored here (AC14.6, FR-11). Only tier-level mappings
// persist between runs. The distinction matters: tier mappings describe a user's preference
// for a class of agents (e.g. "use claude-opus for all HIGH-tier agents"), whereas per-agent
// selections describe a one-time override for a specific agent that should not persist.
type UserConfig struct {
	// SchemaVersion is the config schema version, used for forward compatibility.
	SchemaVersion string

	// TierModels maps harness id -> tier -> model id, recording the user's tier-to-model
	// selections from previous runs. Tier keys are stored verbatim and are never validated
	// against the tiers currently present in source (AC14.5): a tier that has since been
	// renamed or removed in the agent files does not cause an error on load.
	TierModels map[string]map[domain.Tier]string

	// CustomModelIDs maps harness id -> list of custom model IDs entered in previous
	// runs. These are offered as selectable options (not pre-answers) in future runs'
	// QTierModel and QAgentModel questions. The list is append-only across runs and
	// deduplicated by exact string match. This does not conflict with the existing
	// "no per-agent model persistence" design decision: these are option-pool entries,
	// not per-agent mappings.
	CustomModelIDs map[string][]string

	// ToolDestinations maps harness id -> tool-destination mappings declared by this
	// user. Entries here take precedence over the project-level declarations for the
	// same harness id and generic tool. Nil or empty means no config-declared mappings.
	ToolDestinations ToolDestinationsByHarness
}

// UserConfigStore loads and saves the per-user configuration.
type UserConfigStore interface {
	// Load reads MosaicDeploy/config/user-config.yaml. When the file is absent, Load
	// returns a zero-value UserConfig and a nil error. Other I/O or parse errors are
	// returned normally.
	Load() (UserConfig, error)

	// Save writes cfg to MosaicDeploy/config/user-config.yaml, creating the config
	// directory if it does not exist.
	//
	// When the file exists, it is updated in place:
	//   - keys whose value did not change are left byte-identical, including their comments;
	//   - a rewritten key's value is written with two spaces of indentation per nesting level,
	//     however many times the key was rewritten before;
	//   - a key whose new value is empty is removed; only its key line and value lines go, and
	//     every comment line outside that block survives;
	//   - every comment line outside a rewritten or removed value block survives.
	// When the file does not exist, a plain YAML document is written.
	Save(cfg UserConfig) error

	// Path returns the absolute path of user-config.yaml.
	Path() string

	// WithLock acquires an exclusive file lock on a sidecar lock file
	// (<Path()>.lock), calls fn, and releases the lock on return. The lock is
	// scoped to the user-config.yaml file path so distinct MOSAIC roots never
	// contend. If the lock cannot be acquired within the timeout (2-3 seconds),
	// WithLock returns a non-nil error without calling fn. The error message
	// is suitable for display through notifyPersistFailure (human-readable,
	// mentioning that another process held the lock).
	//
	// Callers wrap their Load-mutate-Save cycle inside fn so the entire
	// read-modify-write is atomic with respect to other cooperating processes.
	WithLock(fn func() error) error

	// PutToolDestination writes mapping as the tool_destinations entry for
	// mapping.Generic under harnessID in user-config.yaml: it is added at the end of
	// that harness's list, or replaces the entry with the same generic in that list.
	// It does not take the lock; callers wrap it in WithLock.
	//
	// Every byte outside the added or replaced entry is kept, including comments and
	// formatting inside tool_destinations and inside other harnesses' and other
	// generics' entries; line endings (LF or CRLF) are kept. Exceptions, each limited
	// to what it names:
	//   - "tool_destinations: {}" or a null value: the key line's value is replaced by
	//     the block (a trailing comment on the key line is kept);
	//   - a harness whose value is in flow style: that harness's value is re-rendered
	//     in block style.
	// A missing key is appended as a new top-level block at the end of the file; a
	// missing file is created with that block only.
	//
	// The mapping is validated with the tool_destinations rules before anything is
	// written; an invalid mapping, an unparseable existing file or a write failure
	// returns an error and leaves the file unchanged.
	PutToolDestination(harnessID string, mapping domain.ToolMapping) error
}

// ---------------------------------------------------------------------------
// YAML wire type
// ---------------------------------------------------------------------------

// userConfigYAML is the YAML-serializable form of UserConfig. TierModels uses plain
// string keys and values for YAML marshaling; conversion to/from domain.Tier happens
// in Load and Save.
type userConfigYAML struct {
	SchemaVersion    string                       `yaml:"schema_version"`
	TierModels       map[string]map[string]string `yaml:"tier_models"`
	CustomModelIDs   map[string][]string          `yaml:"custom_model_ids"`
	ToolDestinations map[string][]wireToolMapping `yaml:"tool_destinations"`
}

// ---------------------------------------------------------------------------
// userConfigStore implementation
// ---------------------------------------------------------------------------

type userConfigStore struct {
	filePath string
}

// NewUserConfigStore returns a UserConfigStore rooted at mosaicRoot.
// Config is read from and written to <mosaicRoot>/MosaicDeploy/config/user-config.yaml.
func NewUserConfigStore(mosaicRoot string) UserConfigStore {
	return &userConfigStore{
		filePath: filepath.Join(mosaicRoot, "MosaicDeploy", "config", "user-config.yaml"),
	}
}

// Path returns the absolute path of user-config.yaml.
func (s *userConfigStore) Path() string {
	return s.filePath
}

// WithLock acquires an exclusive file lock on the sidecar lock file at
// s.filePath+".lock", calls fn under the lock, and releases the lock on return.
// If the lock cannot be acquired within lockTimeout, WithLock returns an error
// without calling fn. The lock file (and its parent directory) are created if
// they do not exist. The lock is released even if fn panics.
func (s *userConfigStore) WithLock(fn func() error) error {
	lockPath := s.filePath + ".lock"

	// Ensure the parent directory exists so flock can create the lock file.
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create lock file directory: %w", err)
	}

	fl := flock.New(lockPath)

	deadline := time.Now().Add(lockTimeout)
	for {
		ok, lockErr := fl.TryLock()
		if lockErr != nil {
			return fmt.Errorf("acquire lock on %s: %w", lockPath, lockErr)
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("could not acquire lock on %s: another process held the lock for more than %s",
				lockPath, lockTimeout)
		}
		time.Sleep(50 * time.Millisecond)
	}

	defer func() { _ = fl.Unlock() }()
	return fn()
}

// Load reads user-config.yaml and returns the parsed UserConfig. When the file is absent,
// a zero-value UserConfig is returned without error. Old-schema files are migrated
// transparently: all tier mappings are preserved and SchemaVersion is updated to the current
// value. Tier keys are never validated against the current catalog (AC14.5).
func (s *userConfigStore) Load() (UserConfig, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return UserConfig{}, nil
		}
		return UserConfig{}, fmt.Errorf("read user config: %w", err)
	}

	var raw userConfigYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return UserConfig{}, fmt.Errorf("%s: parse user config: %w", s.filePath, err)
	}

	// Migration: preserve all tier mappings and update SchemaVersion to current.
	cfg := UserConfig{
		SchemaVersion: userConfigSchemaVersion,
	}

	if len(raw.TierModels) > 0 {
		cfg.TierModels = make(map[string]map[domain.Tier]string, len(raw.TierModels))
		for harness, tiers := range raw.TierModels {
			if len(tiers) == 0 {
				continue
			}
			cfg.TierModels[harness] = make(map[domain.Tier]string, len(tiers))
			for tier, model := range tiers {
				cfg.TierModels[harness][domain.Tier(tier)] = model
			}
		}
	}

	if len(raw.CustomModelIDs) > 0 {
		cfg.CustomModelIDs = make(map[string][]string, len(raw.CustomModelIDs))
		for harness, ids := range raw.CustomModelIDs {
			if len(ids) == 0 {
				continue
			}
			copied := make([]string, len(ids))
			copy(copied, ids)
			cfg.CustomModelIDs[harness] = copied
		}
	}

	if len(raw.ToolDestinations) > 0 {
		dest, declErr := validateAndConvertToolDestinations(s.filePath, raw.ToolDestinations)
		if declErr != nil {
			return UserConfig{}, *declErr
		}
		cfg.ToolDestinations = dest
	}

	return cfg, nil
}

// Save writes cfg to user-config.yaml, creating the MosaicDeploy/config directory if
// it does not exist.
//
// When no file exists yet, Save writes a fresh document (the ordinary first-run path).
// When a file already exists and parses, Save edits the four modeled keys — schema_version,
// tier_models, custom_model_ids, tool_destinations — in place within that document instead
// of regenerating it: existing comments (a leading file-header comment, a comment above a
// key, an inline/trailing comment) and the formatting of everything Save does not touch
// survive the write. A key whose new value is empty or nil is removed from the document; a
// key with a non-empty value that changed is updated in place; a key with a non-empty value
// that already matches what is on disk is left untouched, including its comments. Comments
// and ordering are only guaranteed for these four keys — anything else in the file is not
// guaranteed to keep its comments or position, though the file remains valid YAML.
//
// If the existing file cannot be parsed as YAML, Save returns a wrapped error and leaves
// the file on disk unchanged rather than overwriting it.
func (s *userConfigStore) Save(cfg UserConfig) error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create user config directory: %w", err)
	}

	raw := buildRawUserConfig(cfg)

	existing, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveFresh(raw)
		}
		return fmt.Errorf("read existing user config: %w", err)
	}

	data, err := mergeUserConfigInPlace(existing, raw)
	if err != nil {
		return fmt.Errorf("%s: %w", s.filePath, err)
	}

	if err := os.WriteFile(s.filePath, data, 0o644); err != nil {
		return fmt.Errorf("write user config: %w", err)
	}
	return nil
}

// buildRawUserConfig converts cfg into its YAML wire form, omitting empty inner maps the
// same way the old plain-marshal Save always did.
func buildRawUserConfig(cfg UserConfig) userConfigYAML {
	raw := userConfigYAML{
		SchemaVersion: userConfigSchemaVersion,
	}

	if len(cfg.TierModels) > 0 {
		raw.TierModels = make(map[string]map[string]string, len(cfg.TierModels))
		for harness, tiers := range cfg.TierModels {
			if len(tiers) == 0 {
				continue
			}
			raw.TierModels[harness] = make(map[string]string, len(tiers))
			for tier, model := range tiers {
				raw.TierModels[harness][string(tier)] = model
			}
		}
	}

	if len(cfg.CustomModelIDs) > 0 {
		raw.CustomModelIDs = make(map[string][]string, len(cfg.CustomModelIDs))
		for harness, ids := range cfg.CustomModelIDs {
			if len(ids) == 0 {
				continue
			}
			copied := make([]string, len(ids))
			copy(copied, ids)
			raw.CustomModelIDs[harness] = copied
		}
	}

	if len(cfg.ToolDestinations) > 0 {
		wireMap := make(map[string][]wireToolMapping, len(cfg.ToolDestinations))
		for id, mappings := range cfg.ToolDestinations {
			wireMap[id] = toWireToolMappings(mappings)
		}
		raw.ToolDestinations = wireMap
	}

	return raw
}

// saveFresh writes raw as a brand-new document. This is the fallback path used when no
// file exists yet; its output is byte-identical to the original plain-marshal Save.
func (s *userConfigStore) saveFresh(raw userConfigYAML) error {
	data, err := yaml.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("marshal user config: %w", err)
	}
	if err := os.WriteFile(s.filePath, data, 0o644); err != nil {
		return fmt.Errorf("write user config: %w", err)
	}
	return nil
}
