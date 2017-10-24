package plugins

import (
	"common"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Handler func(w http.ResponseWriter, r *http.Request, path_params map[string]string)

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

type entry struct {
	headers map[string][]string
	handler Handler
}

type Mux struct {
	sync.Mutex
	rtable map[string]map[string]*entry
}

func NewMux() *Mux { return new(Mux) }

var defaultMux = NewMux()

func (mux *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	match := false
	wrongMethod := false
	var path_params map[string]string
	var e *entry

	for pattern, methods := range mux.rtable {
		match, path_params, _ = parsePath(r.URL.Path, pattern)

		if match {
			e = methods[r.Method]
			if e == nil {
				wrongMethod = true
			} else {
				break
			}
		}
	}

	if e == nil {
		if wrongMethod {
			w.WriteHeader(http.StatusMethodNotAllowed)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}

		return
	}

	errKeys := make([]string, 0)
	for key, valuesEnum := range e.headers {
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
		w.WriteHeader(headerErr.Code)
		return
	}

	e.handler(w, r, path_params)
}

func (mux *Mux) HandleFunc(path, method string, headers map[string][]string, handler Handler) error {
	path = strings.TrimSpace(path)
	method = strings.TrimSpace(method)

	if len(path) == 0 {
		return fmt.Errorf("empty path pattern")
	}

	if len(method) == 0 {
		return fmt.Errorf("empty http method")
	}

	_, ok := supportedMethods[method]
	if !ok {
		return fmt.Errorf("unsupported http method %s", method)
	}

	if handler == nil {
		return fmt.Errorf("invalid handler")
	}

	mux.Lock()
	defer mux.Unlock()

	if mux.rtable == nil {
		mux.rtable = make(map[string]map[string]*entry)
	}

	// we do not allow to register static path and parametric path on the same segment, for example
	// client can not register the patterns `/user/jack` and `/user/{user}` on the same http method at the same time.
	for p, methods := range mux.rtable {
		dup, err := duplicatedPath(p, path)
		if err != nil {
			return err
		}

		if dup && methods[method] != nil {
			return fmt.Errorf("duplicated handler on %s %s with existing %s %s", method, path, method, p)
		}
	}

	if mux.rtable[path] == nil {
		mux.rtable[path] = make(map[string]*entry)
	}

	mux.rtable[path][method] = &entry{
		headers: headers,
		handler: handler,
	}

	return nil
}

func (mux *Mux) DeleteFunc(path, method string) {
	mux.Lock()
	defer mux.Unlock()

	delete(mux.rtable[path], method)
	if len(mux.rtable[path]) == 0 {
		delete(mux.rtable, path)
	}
}

func findNextChar(str []byte, cs []byte, end_as_closer bool) int {
	l := len(str)

	var p int
	var c1 byte
	for p, c1 = range str {
		for _, c := range cs {
			if c1 == c {
				return p
			}
		}
	}

	if end_as_closer && (l == 0 || p == l-1) {
		return p + 1
	}

	return -1
}

func parsePath(path, pattern string) (bool, map[string]string, error) {
	ret := make(map[string]string)

	path_v := []byte(path)
	pattern_v := []byte(pattern)
	pattern_v_pos, path_v_pos := 0, 0
	pattern_v_l := len(pattern_v)
	path_v_l := len(path_v)

	for pattern_v_pos < pattern_v_l {
		if pattern_v[pattern_v_pos] == '{' {
			end1 := findNextChar(pattern_v[pattern_v_pos+1:], []byte("}"), false)
			if end1 == -1 {
				return false, nil, fmt.Errorf("invalid path pattern")
			}

			end2 := findNextChar(path_v[path_v_pos:], []byte("/?"), true)
			if end2 == -1 {
				return false, nil, fmt.Errorf("invalid path")
			}

			if path_v_pos+end2 > path_v_l {
				return false, ret, nil
			}

			ret[string(pattern_v[pattern_v_pos+1:pattern_v_pos+end1+1])] =
				string(path_v[path_v_pos : path_v_pos+end2])

			pattern_v_pos += 1 + end1 + 1 // last 1 for '}'
			path_v_pos += end2
		} else {
			if path_v_pos < path_v_l && pattern_v[pattern_v_pos] == path_v[path_v_pos] {
				pattern_v_pos++
				path_v_pos++
			} else {
				return false, ret, nil
			}
		}
	}

	if path_v_pos < path_v_l {
		return false, ret, nil
	}

	return true, ret, nil
}

func duplicatedPath(p1, p2 string) (bool, error) {
	flatter := func(pos int, token string) (care bool, replacement string) {
		return true, "*"
	}

	p1, err := common.ScanTokens(p1,
		false, /* do not remove escape char, due to escape char is not allowed in the path and pattern */
		flatter)
	if err != nil {
		return false, err
	}

	p2, err = common.ScanTokens(p2,
		false, /* do not remove escape char, due to escape char is not allowed in the path and pattern */
		flatter)
	if err != nil {
		return false, err
	}

	p1_v := []byte(p1)
	p2_v := []byte(p2)
	p1_v_pos, p2_v_pos := 0, 0
	p1_v_l := len(p1_v)
	p2_v_l := len(p2_v)

	for p1_v_pos < p1_v_l && p2_v_pos < p2_v_l {
		if p1_v[p1_v_pos] == '*' || p2_v[p2_v_pos] == '*' {
			end1 := findNextChar(p1_v[p1_v_pos+1:], []byte("/"), true)
			if end1 == -1 {
				return false, fmt.Errorf("invalid path")
			}

			end2 := findNextChar(p2_v[p2_v_pos+1:], []byte("/"), true)
			if end2 == -1 {
				return false, fmt.Errorf("invalid path")
			}

			p1_v_pos += 1 + end1
			p2_v_pos += 1 + end2
		} else {
			if p1_v[p1_v_pos] == p2_v[p2_v_pos] {
				p1_v_pos++
				p2_v_pos++
			} else {
				return false, nil
			}
		}
	}

	if p1_v_pos < p1_v_l || p2_v_pos < p2_v_l {
		return false, nil
	}

	return true, nil
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
