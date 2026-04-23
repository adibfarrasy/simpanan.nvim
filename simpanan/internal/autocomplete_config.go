package internal

import "time"

// AutocompleteConfiguration holds the tunable knobs for the
// autocompletion subsystem. Defaults mirror the `config` block in
// specs/simpanan.allium.
type AutocompleteConfiguration struct {
	SchemaRefreshInterval time.Duration
	JqPathProbeTimeout    time.Duration
}

// AutocompleteConfig returns the currently-effective configuration.
// Overrides from the environment or the editor are not wired yet; the
// function exists so callers and tests can reference the defaults
// through a single entry point.
func AutocompleteConfig() AutocompleteConfiguration {
	return AutocompleteConfiguration{
		SchemaRefreshInterval: time.Hour,
		JqPathProbeTimeout:    2 * time.Second,
	}
}
