package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"simpanan/internal/common"

	"github.com/go-redis/redis/v8"
)

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
