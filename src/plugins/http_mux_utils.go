package plugins

import (
	"net/http"
)

type muxType string

var (
	regexpMuxType muxType = "regexp"
	paramMuxType  muxType = "param"
)

type headerErr struct {
	Code    int
	Message string
}

var defaultHeaderErr = headerErr{
	Code:    http.StatusBadRequest,
	Message: "invalid request",
}

var headerPriority = []string{"User-Agent", "Content-Type", "Content-Encoding"}

var headerErrs = map[string]headerErr{
	"User-Agent": {
		Code:    http.StatusForbidden,
		Message: "unsupported User-Agent",
	},
	"Content-Type": {
		Code:    http.StatusUnsupportedMediaType,
		Message: "unsupported media type",
	},
	"Content-Encoding": {
		Code:    http.StatusBadRequest,
		Message: "invalid request",
	},
}

func getHeaderError(keys ...string) headerErr {
	for _, keyPattern := range headerPriority {
		for _, k := range keys {
			if keyPattern == k {
				he, ok := headerErrs[k]
				if !ok {
					return defaultHeaderErr
				}
				return he
			}
		}
	}

	return defaultHeaderErr
}
