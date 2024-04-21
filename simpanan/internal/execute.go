package internal

import (
	"errors"
	"fmt"
	"simpanan/internal/adapters"
	"simpanan/internal/common"

	_ "github.com/lib/pq"
)

func execute(q common.QueryMetadata, previousResults []byte) ([]byte, error) {
	if len(previousResults) != 0 {
		common.PipeData(&q, previousResults)
	}

	switch q.ConnType {
	case common.Postgres:
		switch adapters.QueryTypePostgres(q.QueryLine) {
		case common.Read:
			return adapters.ExecutePostgresQuery(q)
		case common.Write:
			return nil, errors.New("Not implemented.")
		default:
			return nil, fmt.Errorf("Unknown query type: '%s'", q.QueryLine)
		}

	case common.Mongo:
		switch adapters.QueryTypeMongo(q.QueryLine) {
		case common.Read:
			return adapters.ExecuteMongoQuery(q)
		case common.Write:
			return nil, errors.New("Not implemented.")
		default:
			return nil, fmt.Errorf("Unknown query type: '%s'", q.QueryLine)
		}

	case common.Redis:
		return nil, errors.New("Not implemented.")
	default:
		return nil, errors.New("Unknown connection type.")
	}
}
