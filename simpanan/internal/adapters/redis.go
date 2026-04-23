package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"simpanan/internal/common"
	"strings"

	"github.com/go-redis/redis/v8"
)

// redisWriteCommands is the set of Redis commands that mutate state.
// Anything not listed here is treated as read by QueryTypeRedis.
var redisWriteCommands = map[string]struct{}{
	// Strings
	"set": {}, "setex": {}, "psetex": {}, "setnx": {}, "setrange": {},
	"mset": {}, "msetnx": {}, "append": {}, "getset": {}, "getdel": {},
	"incr": {}, "incrby": {}, "incrbyfloat": {}, "decr": {}, "decrby": {},
	// Generic keys
	"del": {}, "unlink": {}, "expire": {}, "pexpire": {}, "expireat": {},
	"pexpireat": {}, "persist": {}, "rename": {}, "renamenx": {},
	"move": {}, "copy": {}, "restore": {},
	// Hashes
	"hset": {}, "hsetnx": {}, "hmset": {}, "hdel": {},
	"hincrby": {}, "hincrbyfloat": {},
	// Lists
	"lpush": {}, "rpush": {}, "lpushx": {}, "rpushx": {},
	"lpop": {}, "rpop": {}, "lrem": {}, "lset": {}, "ltrim": {},
	"linsert": {}, "blpop": {}, "brpop": {}, "blmpop": {}, "blmove": {},
	"rpoplpush": {}, "lmove": {}, "lmpop": {},
	// Sets
	"sadd": {}, "srem": {}, "smove": {}, "spop": {},
	"sinterstore": {}, "sunionstore": {}, "sdiffstore": {},
	// Sorted sets
	"zadd": {}, "zrem": {}, "zincrby": {},
	"zremrangebyrank": {}, "zremrangebyscore": {}, "zremrangebylex": {},
	"zunionstore": {}, "zinterstore": {}, "zdiffstore": {},
	"zpopmin": {}, "zpopmax": {}, "bzpopmin": {}, "bzpopmax": {},
	"zmpop": {}, "bzmpop": {},
	// Bitmaps
	"setbit": {}, "bitop": {},
	// Streams
	"xadd": {}, "xdel": {}, "xtrim": {}, "xgroup": {}, "xclaim": {},
	"xack": {}, "xautoclaim": {}, "xsetid": {},
	// HyperLogLog
	"pfadd": {}, "pfmerge": {},
	// Server / admin
	"flushdb": {}, "flushall": {},
	// Pub/Sub
	"publish": {},
	// Scripting (may write; conservative)
	"eval": {}, "evalsha": {},
}

// QueryTypeRedis classifies a Redis command line as a read or a write.
// Classification looks at the first whitespace-separated token only,
// case-insensitively. Unknown commands default to read.
func QueryTypeRedis(query string) common.QueryType {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return common.Read
	}
	if _, ok := redisWriteCommands[strings.ToLower(fields[0])]; ok {
		return common.Write
	}
	return common.Read
}

func ExecuteRedisQuery(q common.QueryMetadata) ([]byte, error) {
	opts, err := redis.ParseURL(q.Conn)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	defer client.Close()

	tokens, err := tokenizeRedisCommand(q.QueryLine)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty redis command")
	}

	args := make([]interface{}, len(tokens))
	for i, v := range tokens {
		args[i] = v
	}

	res, err := client.Do(context.Background(), args...).Result()
	if err != nil {
		return nil, err
	}

	return json.Marshal(res)
}

// tokenizeRedisCommand splits a Redis command line into arguments. It
// honours double- and single-quoted strings (so `SET k "a b"` produces
// three tokens), supports backslash escapes inside quotes, and collapses
// runs of whitespace (so empty tokens are never produced).
func tokenizeRedisCommand(input string) ([]string, error) {
	var tokens []string
	var acc []rune
	inString := false
	var stringDelim rune
	escaped := false
	hasToken := false

	flush := func() {
		if hasToken {
			tokens = append(tokens, string(acc))
			acc = acc[:0]
			hasToken = false
		}
	}

	for i, c := range input {
		if escaped {
			acc = append(acc, c)
			hasToken = true
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
				continue
			}
			if c == stringDelim {
				inString = false
				continue
			}
			acc = append(acc, c)
			hasToken = true
			continue
		}
		switch c {
		case ' ', '\t', '\n', '\r':
			flush()
		case '"', '\'':
			inString = true
			stringDelim = c
			// A quoted empty string is still a token.
			hasToken = true
		case '\\':
			escaped = true
		default:
			acc = append(acc, c)
			hasToken = true
			_ = i
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated quoted string in redis command")
	}
	if escaped {
		return nil, fmt.Errorf("dangling backslash in redis command")
	}
	flush()
	return tokens, nil
}
