package internal

type (
	KeyURIPairs []KeyURIPair
	URI         string
	KeyURIPair  struct {
		Key string `json:"key_name"`
		URI URI    `json:"uri"`
	}

	ConnType      string
	ExecType      string
	QueryMetadata struct {
		Conn     string
		ConnType ConnType
		ExecLine string
		ExecType ExecType
	}

	ColumnValuePair []string // index 0 is column name, index 1 is value
	RowData         []ColumnValuePair
)

var (
	Postgres ConnType = "postgres"
	Mongo    ConnType = "mongo"
	Redis    ConnType = "redis"
	Unknown  ConnType = "unknown"

	Command ExecType = "command"
	Query   ExecType = "query"

	QUERY_PREFIXES = []string{
		"SELECT",
		"select",
	}
)
