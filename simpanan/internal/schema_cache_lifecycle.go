package internal

import (
	"simpanan/internal/common"
	"time"
)

// introspectFn is the pluggable introspection entry point. Tests
// replace this to avoid real database I/O; production uses the default
// IntrospectSchema implementation.
var introspectFn = IntrospectSchema

// EnsureSchemaCache is the lazy, per-completion-request entry point
// for fetching a connection's SchemaCache. Because the Go binary is
// invoked per RPC call rather than running as a daemon, there is no
// in-memory state to keep caches warm between invocations — disk is
// authoritative, and freshness is checked at every call.
//
// Behaviour:
//   - Ineligible types (Redis, Jq, unregistered labels) return (nil, nil).
//   - A disk cache younger than AutocompleteConfig.SchemaRefreshInterval
//     is returned without re-introspecting.
//   - A missing or stale cache triggers introspection. On success the
//     fresh cache is persisted and returned. On failure the existing
//     stale cache is returned unchanged (populated_at preserved) with
//     last_refresh_attempt updated to now — matching the spec rule
//     PopulateSchemaCache's "keep existing data on failure".
func EnsureSchemaCache(label string) (*SchemaCache, error) {
	uri, ct, ok, err := lookupConnection(label)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if ct == common.Redis || ct == common.Jq {
		return nil, nil
	}

	existing, loadErr := LoadSchemaCache(label)
	// A corrupted on-disk cache is treated as absent: the lazy refresh
	// will overwrite it. The error is intentionally swallowed rather
	// than propagated so completion requests stay silent.
	if loadErr != nil {
		existing = nil
	}

	if existing != nil && !isSchemaCacheStale(existing, time.Now()) {
		return existing, nil
	}

	return populateSchemaCache(label, uri, ct, existing)
}

// PopulateSchemaCacheForConnection is the hook called on AddConnection.
// Best-effort: errors are swallowed so adding a connection never fails
// because the backing database is momentarily unreachable. The spec
// rule PopulateOnAddConnection is satisfied when the DB is available;
// otherwise the cache is populated lazily at first completion request.
func PopulateSchemaCacheForConnection(label, uri string, ct common.ConnType) {
	if ct == common.Redis || ct == common.Jq {
		return
	}
	_, _ = populateSchemaCache(label, uri, ct, nil)
}

// populateSchemaCache runs introspection and persists the result,
// falling back to the supplied stale cache on failure.
func populateSchemaCache(label, uri string, ct common.ConnType, stale *SchemaCache) (*SchemaCache, error) {
	fresh, err := introspectFn(label, uri, ct)
	now := time.Now()
	if err != nil || fresh == nil {
		if stale != nil {
			stale.LastRefreshAttempt = &now
			// Best-effort save of the updated attempt timestamp. A write
			// failure here doesn't escalate — the returned struct is
			// still usable in memory.
			_ = SaveSchemaCache(stale)
		}
		return stale, err
	}
	if fresh.ConnectionLabel == "" {
		fresh.ConnectionLabel = label
	}
	fresh.PopulatedAt = &now
	fresh.LastRefreshAttempt = &now
	if err := SaveSchemaCache(fresh); err != nil {
		// Couldn't persist, but the in-memory cache is still useful.
		return fresh, err
	}
	return fresh, nil
}

// isSchemaCacheStale reports whether the cache's PopulatedAt is older
// than the configured refresh interval. A nil PopulatedAt (never
// successfully populated) counts as stale.
func isSchemaCacheStale(c *SchemaCache, now time.Time) bool {
	if c == nil || c.PopulatedAt == nil {
		return true
	}
	return now.Sub(*c.PopulatedAt) >= AutocompleteConfig().SchemaRefreshInterval
}

// lookupConnection returns the URI and connection type registered for
// a label. The third return is false when the label is not registered.
func lookupConnection(label string) (string, common.ConnType, bool, error) {
	conns, err := GetConnectionList()
	if err != nil {
		return "", "", false, err
	}
	for _, c := range conns {
		if c.Key == label {
			ct, err := common.URI(c.URI).ConnType()
			if err != nil {
				return "", "", false, err
			}
			return string(c.URI), *ct, true, nil
		}
	}
	return "", "", false, nil
}
