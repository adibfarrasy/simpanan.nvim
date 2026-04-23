//go:build autocomplete

package internal

// Skeleton tests for the unimplemented schema cache feature described
// in specs/simpanan.allium. These tests are gated behind the
// `autocomplete` build tag so the default `go test ./...` run stays
// green. Running `go test -tags autocomplete ./...` will fail to
// compile until the referenced production symbols exist — that
// compile failure is the intended signal that the feature is still
// to be implemented.
//
// Every test starts with t.Skip so that even once the symbols exist,
// the suite remains visibly "not yet implemented" until an engineer
// removes the Skip and fleshes out the assertions.
//
// Invented Go signatures are marked `// TODO: signature to be
// finalised during implementation`.

import (
	"simpanan/internal/common"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// entity SchemaCache / DatabaseSchema / TableSchema / CollectionSchema
//
// The spec says a SchemaCache is keyed by connection_label, carries
// populated_at / last_refresh_attempt timestamps, and holds a set of
// DatabaseSchema children. Tables (SQL) and Collections (Mongo) are
// mutually exclusive in practice but modelled as two disjoint fields.
// -----------------------------------------------------------------------

// TODO: signature to be finalised during implementation
// type SchemaCache struct { ConnectionLabel string; PopulatedAt *time.Time; LastRefreshAttempt *time.Time; Databases []DatabaseSchema }
// type DatabaseSchema struct { Name string; Tables []TableSchema; Collections []CollectionSchema }
// type TableSchema struct { Name string; Columns []string }
// type CollectionSchema struct { Name string; Fields []string }

// -----------------------------------------------------------------------
// rule LoadCachesOnStartup
// -----------------------------------------------------------------------

func TestLoadCachesOnStartup(t *testing.T) {
	t.Skip("autocomplete not implemented: rule LoadCachesOnStartup")

	// TODO: signature to be finalised during implementation
	// LoadCachesOnStartup(registry ConnectionRegistry, now time.Time) ([]SchemaCache, []string /*populateRequests*/, error)

	type conn struct {
		label   string
		uri     string
		ct      common.ConnType
		persist bool
		age     time.Duration
	}
	cases := []struct {
		name             string
		refreshInterval  time.Duration
		conns            []conn
		wantLoadedLabels []string
		wantRequested    []string
	}{
		{
			name:            "eligible connection with fresh persisted cache loads without request",
			refreshInterval: time.Hour,
			conns: []conn{
				{label: "pg", uri: "postgres://h/db", ct: common.Postgres, persist: true, age: 10 * time.Minute},
			},
			wantLoadedLabels: []string{"pg"},
			wantRequested:    nil,
		},
		{
			name:            "eligible connection with stale persisted cache triggers populate",
			refreshInterval: time.Hour,
			conns: []conn{
				{label: "pg", uri: "postgres://h/db", ct: common.Postgres, persist: true, age: 2 * time.Hour},
			},
			wantLoadedLabels: nil,
			wantRequested:    []string{"pg"},
		},
		{
			name:            "eligible connection without persisted cache triggers populate",
			refreshInterval: time.Hour,
			conns: []conn{
				{label: "mg", uri: "mongodb://h/db", ct: common.Mongo, persist: false},
			},
			wantLoadedLabels: nil,
			wantRequested:    []string{"mg"},
		},
		{
			name:            "redis is skipped entirely",
			refreshInterval: time.Hour,
			conns: []conn{
				{label: "rd", uri: "redis://h", ct: common.Redis, persist: false},
			},
			wantLoadedLabels: nil,
			wantRequested:    nil,
		},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// rule PopulateSchemaCache
//
// Spec:
//   - on introspection success, replace any prior cache; set populated_at
//     and last_refresh_attempt to now; attach fresh databases.
//   - on failure, keep existing cache (if any) untouched; only update
//     last_refresh_attempt.
// -----------------------------------------------------------------------

func TestPopulateSchemaCache(t *testing.T) {
	t.Skip("autocomplete not implemented: rule PopulateSchemaCache")

	// TODO: signature to be finalised during implementation
	// PopulateSchemaCache(registry ConnectionRegistry, label string) error

	cases := []struct {
		name                  string
		priorCacheExists      bool
		introspectSucceeds    bool
		wantCacheReplaced     bool
		wantAttemptUpdated    bool
		wantPopulatedAtUpdate bool
	}{
		{
			name:                  "success with no prior cache creates a fresh cache",
			priorCacheExists:      false,
			introspectSucceeds:    true,
			wantCacheReplaced:     true,
			wantAttemptUpdated:    true,
			wantPopulatedAtUpdate: true,
		},
		{
			name:                  "success with prior cache replaces it",
			priorCacheExists:      true,
			introspectSucceeds:    true,
			wantCacheReplaced:     true,
			wantAttemptUpdated:    true,
			wantPopulatedAtUpdate: true,
		},
		{
			name:                  "failure with prior cache leaves data untouched and records attempt",
			priorCacheExists:      true,
			introspectSucceeds:    false,
			wantCacheReplaced:     false,
			wantAttemptUpdated:    true,
			wantPopulatedAtUpdate: false,
		},
		{
			name:                  "failure with no prior cache is silently ignored",
			priorCacheExists:      false,
			introspectSucceeds:    false,
			wantCacheReplaced:     false,
			wantAttemptUpdated:    false,
			wantPopulatedAtUpdate: false,
		},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// rule RefreshOnTick
//
// Temporal rule. The implementation will likely schedule a background
// ticker at config.schema_refresh_interval. Without clock injection we
// cannot reliably assert the firing behaviour from tests. This
// skeleton documents the obligation; a follow-up must introduce a
// clock/scheduler seam before this test can be fleshed out without
// relying on wall-clock sleeps.
// -----------------------------------------------------------------------

func TestRefreshOnTick(t *testing.T) {
	t.Skip("autocomplete not implemented: rule RefreshOnTick (also requires clock-injection seam)")

	// TODO: signature to be finalised during implementation
	// StartSchemaRefresher(registry ConnectionRegistry, clock Clock, interval time.Duration) (stop func())

	// Obligation: when `populated_at + config.schema_refresh_interval <= now`,
	// a SchemaPopulateRequested event is emitted for every eligible
	// connection. Populate failures are silent.
}

// -----------------------------------------------------------------------
// rule PopulateOnAddConnection
// -----------------------------------------------------------------------

func TestPopulateOnAddConnection(t *testing.T) {
	t.Skip("autocomplete not implemented: rule PopulateOnAddConnection")

	// TODO: signature to be finalised during implementation
	// OnConnectionAdded(c common.KeyURIPair) (requestedLabels []string)

	cases := []struct {
		name          string
		conn          common.KeyURIPair
		wantRequested []string
	}{
		{
			name:          "postgres triggers populate",
			conn:          common.KeyURIPair{Key: "pg", URI: "postgres://h/db"},
			wantRequested: []string{"pg"},
		},
		{
			name:          "mysql triggers populate",
			conn:          common.KeyURIPair{Key: "my", URI: "mysql://h/db"},
			wantRequested: []string{"my"},
		},
		{
			name:          "mongo triggers populate",
			conn:          common.KeyURIPair{Key: "mg", URI: "mongodb://h/db"},
			wantRequested: []string{"mg"},
		},
		{
			name:          "redis does not trigger populate",
			conn:          common.KeyURIPair{Key: "rd", URI: "redis://h"},
			wantRequested: nil,
		},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// rule DropCacheOnDeleteConnection
// -----------------------------------------------------------------------

func TestDropCacheOnDeleteConnection(t *testing.T) {
	t.Skip("autocomplete not implemented: rule DropCacheOnDeleteConnection")

	// TODO: signature to be finalised during implementation
	// OnConnectionDeleted(label string) (droppedLabels []string, err error)

	cases := []struct {
		name         string
		label        string
		cacheExists  bool
		wantDropped  []string
		wantErr      bool
	}{
		{
			name:        "existing cache is dropped",
			label:       "pg",
			cacheExists: true,
			wantDropped: []string{"pg"},
		},
		{
			name:        "missing cache is a no-op",
			label:       "pg",
			cacheExists: false,
			wantDropped: nil,
		},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// invariant SchemaCachePerEligibleConnection
// -----------------------------------------------------------------------

func TestSchemaCachePerEligibleConnection(t *testing.T) {
	t.Skip("autocomplete not implemented: invariant SchemaCachePerEligibleConnection")

	// Obligation: at most one SchemaCache per connection_label across
	// all eligible connections. A second populate for the same label
	// replaces — never duplicates.
}

// -----------------------------------------------------------------------
// invariant NoSchemaCacheForRedisOrJq
// -----------------------------------------------------------------------

func TestNoSchemaCacheForRedisOrJq(t *testing.T) {
	t.Skip("autocomplete not implemented: invariant NoSchemaCacheForRedisOrJq")

	// Obligation: a SchemaCache must never exist with
	// connection_label == "jq" or with a label whose Connection has
	// connection_type == redis. Any code path that attempts to create
	// one should reject it.
	cases := []struct {
		name       string
		label      string
		connType   common.ConnType
		shouldFail bool
	}{
		{"reserved jq label rejected", "jq", common.Jq, true},
		{"redis connection rejected", "rd", common.Redis, true},
		{"postgres connection accepted", "pg", common.Postgres, false},
		{"mysql connection accepted", "my", common.Mysql, false},
		{"mongo connection accepted", "mg", common.Mongo, false},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// config defaults
// -----------------------------------------------------------------------

func TestSchemaRefreshIntervalDefault(t *testing.T) {
	// Obligation: the default value for schema_refresh_interval is 1 hour.
	want := time.Hour
	got := AutocompleteConfig().SchemaRefreshInterval
	if got != want {
		t.Fatalf("schema_refresh_interval default: want %v, got %v", want, got)
	}
}

func TestJqPathProbeTimeoutDefault(t *testing.T) {
	// Obligation: default is 2 seconds.
	want := 2 * time.Second
	got := AutocompleteConfig().JqPathProbeTimeout
	if got != want {
		t.Fatalf("jq_path_probe_timeout default: want %v, got %v", want, got)
	}
}
