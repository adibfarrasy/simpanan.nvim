package adapters

import (
	"context"
	"encoding/json"
	"simpanan/internal/common"
	"strings"

	"github.com/go-redis/redis/v8"
)

func ExecuteRedisQuery(q common.QueryMetadata) ([]byte, error) {
	options, err := redis.ParseURL(q.Conn)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)
	qArgs := strings.Split(q.QueryLine, " ")
	result := make([]interface{}, len(qArgs))
	for i, v := range qArgs {
		result[i] = v
	}

	res, err := client.Do(context.Background(), result...).Result()
	if err != nil {
		return nil, err
	}

	return json.Marshal(res)
}
