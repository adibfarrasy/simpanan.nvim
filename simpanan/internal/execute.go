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
			return adapters.ExecutePostgresReadQuery(q)
		case common.Write:
			return adapters.ExecutePostgresWriteQuery(q)
		case common.Admin:
			// psql meta-commands, e.g. \dt, \d <table>.
			return adapters.ExecutePostgresAdminCmd(q)
		default:
			return nil, fmt.Errorf("Unknown query type: '%s'", q.QueryLine)
		}

	case common.Mysql:
		switch adapters.QueryTypeMysql(q.QueryLine) {
		case common.Read:
			return adapters.ExecuteMysqlReadQuery(q)
		case common.Write:
			return adapters.ExecuteMysqlWriteQuery(q)
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
