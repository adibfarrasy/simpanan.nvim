package common

type (
	KeyURIPairs []KeyURIPair
	URI         string
	KeyURIPair  struct {
		Key string `json:"key_name"`
		URI URI    `json:"uri"`
	}

	ConnType      string
	QueryType     string
	QueryMetadata struct {
		Conn      string   `json:"conn"`
		ConnType  ConnType `json:"conn_type"`
		QueryLine string   `json:"query"`
	}

	ColumnValuePair struct {
		Key   string
		Value any
	}
	RowData []ColumnValuePair
)

var (
	Postgres ConnType = "postgres"
	Mongo    ConnType = "mongo"
	Redis    ConnType = "redis"
	Jq       ConnType = "jq"

	Write QueryType = "write"
	Read  QueryType = "read"
)
