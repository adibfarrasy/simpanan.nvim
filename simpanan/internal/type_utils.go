package internal

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
	protocol := strings.Split(string(u), "://")[0]
	switch {
	case strings.Contains(protocol, "postgres"):
		return &Postgres, nil
	case strings.Contains(protocol, "mongodb"):
		return &Mongo, nil
	case strings.Contains(protocol, "redis"):
		return &Redis, nil
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

func (rd rowData) MarshallJSON() (out []byte, err error) {
	if rd == nil {
		return []byte(`null`), nil
	}
	if len(rd) == 0 {
		return []byte(`{}`), nil
	}

	out = append(out, '{')
	for _, e := range rd {
		key, err := json.Marshal(e.key)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(e.value)
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
