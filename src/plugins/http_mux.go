package plugins

import (
	"fmt"
	"net/http"
	"sync"
)

type Handler func(http.ResponseWriter, *http.Request)

func Error(w http.ResponseWriter, code int) { w.WriteHeader(code) }

var supportedMethods = map[string]interface{}{
	http.MethodGet:     nil,
	http.MethodHead:    nil,
	http.MethodPost:    nil,
	http.MethodPut:     nil,
	http.MethodPatch:   nil,
	http.MethodDelete:  nil,
	http.MethodConnect: nil,
	http.MethodOptions: nil,
	http.MethodTrace:   nil,
}

type muxEntry struct {
	headers map[string][]string
	handler Handler
}

type Mux struct {
	mu sync.RWMutex
	m  map[string]map[string]*muxEntry
}

func NewMux() *Mux { return new(Mux) }

var defaultMux = NewMux()

func (mux *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Supports regex on url
	if mux.m[r.URL.Path] == nil {
		Error(w, http.StatusNotFound)
		return
	}
	if mux.m[r.URL.Path][r.Method] == nil {
		Error(w, http.StatusMethodNotAllowed)
		return
	}

	errKeys := make([]string, 0)
	for key, valuesEnum := range mux.m[r.URL.Path][r.Method].headers {
		errKeys = append(errKeys, key)
		v := r.Header.Get(key)
		for _, valueEnum := range valuesEnum {
			if v == valueEnum {
				errKeys = errKeys[:len(errKeys)-1]
				break
			}
		}
	}
	if len(errKeys) > 0 {
		headerErr := getHeaderError(errKeys...)
		Error(w, headerErr.Code)
		return
	}

	mux.m[r.URL.Path][r.Method].handler(w, r)
}

func (mux *Mux) HandleFunc(pattern, method string, headers map[string][]string, handler Handler) error {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if mux.m[pattern][method] != nil {
		return fmt.Errorf("duplicate handle on %s %s", method, pattern)
	}
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}
	_, ok := supportedMethods[method]
	if !ok {
		return fmt.Errorf("unsupported method %s", method)
	}
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}

	if mux.m == nil {
		mux.m = make(map[string]map[string]*muxEntry)
	}
	if mux.m[pattern] == nil {
		mux.m[pattern] = make(map[string]*muxEntry)
	}
	mux.m[pattern][method] = &muxEntry{
		headers: headers,
		handler: handler,
	}

	return nil
}

func (mux *Mux) DeleteFunc(pattern, method string) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	delete(mux.m[pattern], method)
	if len(mux.m[pattern]) == 0 {
		delete(mux.m, pattern)
	}
}

type headerErr struct {
	Code    int
	Message string
}

var defaultHeaderErr = headerErr{
	Code:    http.StatusNotFound,
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
