package internal

import (
	"errors"
	"fmt"
	"simpanan/internal/adapters"
	"simpanan/internal/common"

	_ "github.com/lib/pq"
)

func execute(q common.QueryMetadata, previousResults []byte) ([]byte, error) {
	if len(previousResults) != 0 && q.ConnType != common.Jq {
		common.PipeData(&q, previousResults)
	}

	switch q.ConnType {
	case common.Postgres:
		switch adapters.QueryTypePostgres(q.QueryLine) {
		case common.Read:
			if q.QueryLine[0] == '\\' {
				// special postgres syntax, e.g. \dt, \d <table>, etc.
				return adapters.ExecutePostgresAdminCmd(q)
			} else {
				return adapters.ExecutePostgresReadQuery(q)
			}
		case common.Write:
			return adapters.ExecutePostgresWriteQuery(q)
		default:
			return nil, fmt.Errorf("Unknown query type: '%s'", q.QueryLine)
		}

	case common.Mongo:
		switch adapters.QueryTypeMongo(q.QueryLine) {
		case common.Read:
			return adapters.ExecuteMongoReadQuery(q)
		case common.Write:
			return adapters.ExecuteMongoWriteQuery(q)
		default:
			return nil, fmt.Errorf("Unknown query type: '%s'", q.QueryLine)
		}

	case common.Redis:
		return adapters.ExecuteRedisQuery(q)

	case common.Jq:
		return adapters.ExecuteJqQuery(q, previousResults)

	default:
		return nil, errors.New("Unknown connection type.")
	}
}
