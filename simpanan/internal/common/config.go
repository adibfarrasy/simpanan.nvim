package common

import (
	"strconv"
	"strings"
)

type config struct {
	MaxRowLimit int
	DebugMode   bool
}

var cfg = config{
	MaxRowLimit: 20,
	DebugMode:   false,
}

func GetConfig() *config {
	return &cfg
}

func SetConfig(opts []string) error {
	for _, opt := range opts {
		kv := strings.Split(opt, "=")
		switch kv[0] {
		case "max_row_limit":
			v, err := strconv.Atoi(kv[1])
			if err != nil {
				return err
			}
			cfg.MaxRowLimit = v
		case "debug_mode":
			v, err := strconv.ParseBool(kv[1])
			if err != nil {
				return err
			}
			cfg.DebugMode = v
		default:
			// noop
		}
	}
	return nil
}
