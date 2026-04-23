package adapters

import (
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenizeRedisCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"simple", "GET foo", []string{"GET", "foo"}, false},
		{"collapses multiple spaces", "GET  foo", []string{"GET", "foo"}, false},
		{"leading and trailing spaces", "  GET foo  ", []string{"GET", "foo"}, false},
		{"double-quoted value with space", `SET k "a b"`, []string{"SET", "k", "a b"}, false},
		{"single-quoted value", `SET k 'a b'`, []string{"SET", "k", "a b"}, false},
		{"escaped quote inside string", `SET k "a\"b"`, []string{"SET", "k", `a"b`}, false},
		{"quoted empty string", `SET k ""`, []string{"SET", "k", ""}, false},
		{"tab separator", "GET\tfoo", []string{"GET", "foo"}, false},
		{"unterminated quote", `SET k "foo`, nil, true},
		{"empty input", "", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tokenizeRedisCommand(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestQueryTypeRedis(t *testing.T) {
	tests := []struct {
		query string
		want  common.QueryType
	}{
		// Reads — inspection and retrieval
		{"GET foo", common.Read},
		{"get foo", common.Read},
		{"MGET a b c", common.Read},
		{"EXISTS k", common.Read},
		{"TYPE k", common.Read},
		{"TTL k", common.Read},
		{"KEYS *", common.Read},
		{"SCAN 0", common.Read},
		{"HGET h f", common.Read},
		{"HGETALL h", common.Read},
		{"LRANGE l 0 -1", common.Read},
		{"SMEMBERS s", common.Read},
		{"SISMEMBER s m", common.Read},
		{"ZRANGE z 0 -1", common.Read},
		{"ZSCORE z m", common.Read},
		{"DBSIZE", common.Read},
		{"PING", common.Read},
		{"INFO", common.Read},
		{"BITCOUNT k", common.Read},
		// Writes — strings
		{"SET k v", common.Write},
		{"set k v", common.Write},
		{"SETEX k 10 v", common.Write},
		{"MSET a 1 b 2", common.Write},
		{"APPEND k more", common.Write},
		{"INCR counter", common.Write},
		{"DECRBY c 5", common.Write},
		// Writes — generic keys
		{"DEL k", common.Write},
		{"UNLINK k", common.Write},
		{"EXPIRE k 60", common.Write},
		{"RENAME a b", common.Write},
		// Writes — hashes
		{"HSET h f v", common.Write},
		{"HDEL h f", common.Write},
		// Writes — lists
		{"LPUSH l v", common.Write},
		{"RPUSH l v", common.Write},
		{"LPOP l", common.Write},
		{"LTRIM l 0 9", common.Write},
		// Writes — sets
		{"SADD s m", common.Write},
		{"SREM s m", common.Write},
		// Writes — sorted sets
		{"ZADD z 1 m", common.Write},
		{"ZREM z m", common.Write},
		// Writes — streams
		{"XADD stream * field value", common.Write},
		{"XDEL stream 1", common.Write},
		// Writes — pub/sub, scripting, admin
		{"PUBLISH ch msg", common.Write},
		{"EVAL \"return 1\" 0", common.Write},
		{"FLUSHDB", common.Write},
		{"FLUSHALL", common.Write},
		// Edge — empty and whitespace default to read
		{"", common.Read},
		{"   ", common.Read},
		// Unknown command defaults to read
		{"FOOBAR k", common.Read},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			assert.Equal(t, tc.want, QueryTypeRedis(tc.query))
		})
	}
}
