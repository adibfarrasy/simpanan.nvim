package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (kup KeyURIPair) String() string {
	return fmt.Sprintf("%s → %s", kup.Key, kup.URI)
}

func (u URI) ConnType() (*ConnType, error) {
	s := string(u)
	idx := strings.Index(s, "://")
	if idx < 0 {
		return nil, errors.New("unknown connection type")
	}
	scheme := s[:idx]
	switch scheme {
	case "postgres", "postgresql":
		return &Postgres, nil
	case "mysql":
		return &Mysql, nil
	case "mongodb", "mongodb+srv":
		return &Mongo, nil
	case "redis", "rediss":
		return &Redis, nil
	case "jq":
		return &Jq, nil
	default:
		return nil, errors.New("unknown connection type")
	}
}

func (kups KeyURIPairs) Map() map[string]string {
	res := map[string]string{}

	for _, kup := range kups {
		res[kup.Key] = string(kup.URI)
	}
	return res
}

func (rd RowData) MarshallJSON() (out []byte, err error) {
	if rd == nil {
		return []byte(`null`), nil
	}
	if len(rd) == 0 {
		return []byte(`{}`), nil
	}

	out = append(out, '{')
	for _, e := range rd {
		key, err := json.Marshal(e.Key)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(e.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, key...)
		out = append(out, ':')
		out = append(out, val...)
		out = append(out, ',')
	}
	// replace last ',' with '}'
	out[len(out)-1] = '}'
	return out, nil
}
