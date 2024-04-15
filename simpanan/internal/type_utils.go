package internal

import (
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

func (rd RowData) Map() map[string]string {
	res := map[string]string{}

	for _, r := range rd {
		res[r[0]] = r[1]
	}

	return res
}
