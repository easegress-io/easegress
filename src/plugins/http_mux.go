package plugins

import (
	"common"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/hexdecteam/easegateway-types/plugins"
)

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

////

type mux struct {
	sync.Mutex
	rtable map[string]map[string]map[string]*plugins.HTTPMuxEntry
}

func newMux() *mux {
	return new(mux)
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	match := false
	wrongMethod := false
	var path_params map[string]string
	var e *plugins.HTTPMuxEntry

LOOP:
	for _, pipeline_rtable := range m.rtable {
		for pattern, methods := range pipeline_rtable {
			match, path_params, _ = parsePath(r.URL.Path, pattern)
			if match {
				e = methods[r.Method]
				if e == nil {
					wrongMethod = true
				} else {
					break LOOP
				}
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
	for key, valuesEnum := range e.Headers {
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

	e.Handler(w, r, path_params)
}

func (m *mux) validate(path, method string) error {
	// we do not allow to register static path and parametric path on the same segment, for example
	// client can not register the patterns `/user/jack` and `/user/{user}` on the same http method at the same time.
	for pipeline, pipeline_rtable := range m.rtable {
		for p, methods := range pipeline_rtable {
			dup, err := duplicatedPath(p, path)
			if err != nil {
				return err
			}

			if dup && methods[method] != nil {
				return fmt.Errorf("duplicated handler on %s %s with existing %s %s in pipeline %s",
					method, path, method, p, pipeline)
			}
		}
	}

	return nil
}

func (m *mux) _locklessAddFunc(pipeline, path, method string, headers map[string][]string,
	handler plugins.HTTPHandler) {

	if m.rtable == nil {
		m.rtable = make(map[string]map[string]map[string]*plugins.HTTPMuxEntry)
	}

	pipeline_rtable, exists := m.rtable[pipeline]
	if !exists {
		pipeline_rtable = make(map[string]map[string]*plugins.HTTPMuxEntry)
		m.rtable[pipeline] = pipeline_rtable
	}

	path_rules, exists := pipeline_rtable[path]
	if !exists {
		path_rules = make(map[string]*plugins.HTTPMuxEntry)
		pipeline_rtable[path] = path_rules
	}

	path_rules[method] = &plugins.HTTPMuxEntry{
		Headers: headers,
		Handler: handler,
	}
}

func (m *mux) AddFunc(pipeline, path, method string, headers map[string][]string, handler plugins.HTTPHandler) error {
	ts := strings.TrimSpace
	pipeline = ts(pipeline)
	path = ts(path)
	method = ts(method)

	if len(pipeline) == 0 {
		return fmt.Errorf("empty pipeline name")
	}

	if len(path) == 0 {
		return fmt.Errorf("empty path")
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

	m.Lock()
	defer m.Unlock()

	err := m.validate(path, method)
	if err != nil {
		return err
	}

	m._locklessAddFunc(pipeline, path, method, headers, handler)

	return nil
}

func (m *mux) AddFuncs(pipeline string, pipeline_rtable map[string]map[string]*plugins.HTTPMuxEntry) error {
	ts := strings.TrimSpace
	pipeline = ts(pipeline)

	if len(pipeline) == 0 {
		return fmt.Errorf("empty pipeline name")
	}

	if pipeline_rtable == nil {
		return fmt.Errorf("empty pipeline route table")
	}

	m.Lock()
	defer m.Unlock()

	// full validation first
	for path, methods := range pipeline_rtable {
		for method := range methods {
			err := m.validate(path, method)
			if err != nil {
				return err
			}

		}
	}

	for path, methods := range pipeline_rtable {
		for method, entry := range methods {
			m._locklessAddFunc(pipeline, path, method, entry.Headers, entry.Handler)
		}
	}

	return nil
}

func (m *mux) DeleteFunc(pipeline, path, method string) {
	m.Lock()
	defer m.Unlock()

	pipeline_rtable, exists := m.rtable[pipeline]
	if !exists {
		return
	}

	delete(pipeline_rtable[path], method)
	if len(pipeline_rtable[path]) == 0 {
		delete(pipeline_rtable, path)
	}
	if len(pipeline_rtable) == 0 {
		delete(m.rtable, pipeline)
	}
}

func (m *mux) DeleteFuncs(pipeline string) map[string]map[string]*plugins.HTTPMuxEntry {
	m.Lock()
	defer m.Unlock()

	pipeline_rtable := m.rtable[pipeline]
	delete(m.rtable, pipeline)
	return pipeline_rtable
}

////

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

////

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
