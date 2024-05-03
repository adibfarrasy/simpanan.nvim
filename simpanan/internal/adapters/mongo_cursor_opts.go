package adapters

import (
	"fmt"
	"regexp"
	"strconv"

	"go.mongodb.org/mongo-driver/mongo/options"
)

type cursorOpt interface {
	Apply(any) (any, error)
}

func newFindCursorOpts(opts [][]string) (cOpts []cursorOpt, err error) {
	for _, o := range opts {
		switch o[1] {
		case "sort":
			param, err := constructBsonObject(o[2])
			if err != nil {
				return nil, err
			}

			cOpts = append(cOpts, findSort{param})
		case "limit":
			intParam, err := strconv.Atoi(o[2])
			if err != nil {
				return nil, fmt.Errorf("Failed to parse param %v to int: %s", o[2], err.Error())
			}
			cOpts = append(cOpts, findLimit{int64(intParam)})
		}
	}
	return
}

type findSort struct{ param any }

func (fsort findSort) Apply(opts any) (any, error) {
	fo, ok := opts.(*options.FindOptions)
	if !ok {
		return nil, fmt.Errorf("findSort Apply: failed to cast opts %v.", &opts)
	}
	return fo.SetSort(fsort.param), nil
}

type findLimit struct{ param int64 }

func (flimit findLimit) Apply(opts any) (any, error) {
	fo, ok := opts.(*options.FindOptions)
	if !ok {
		return nil, fmt.Errorf("findLimit Apply: failed to cast opts %v.", &opts)
	}

	return fo.SetLimit(flimit.param), nil
}

func parseCursorOpts(method method, cursorOptStr string) ([]cursorOpt, error) {
	if cursorOptStr == "" {
		return []cursorOpt{}, nil
	}
	matches := regexp.MustCompile(`\.(.*?)\((.*?)\)`).FindAllStringSubmatch(cursorOptStr, -1)
	if len(matches) < 2 {
		return nil, fmt.Errorf("parseCursorOpts: invalid option length. method: %s, cursorOptStr: %s", string(method), cursorOptStr)
	}
	switch method {
	case find:
		return newFindCursorOpts(matches)
	default:
		return nil, fmt.Errorf("parseCursorOpts: method %s not implemented.", string(method))
	}

}
