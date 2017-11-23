package plugins

import (
	"common"
	"fmt"
	"net/http"
	"path/filepath"
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

type paramMuxConfig struct {
}

type paramMux struct {
	sync.RWMutex
	rtable map[string]map[string]map[string]*plugins.HTTPMuxEntry
}

func newParamMux(conf *paramMuxConfig) (*paramMux, error) {
	return new(paramMux), nil
}

func (m *paramMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	match := false
	wrongMethod := false
	var path_params map[string]string
	var e *plugins.HTTPMuxEntry

	m.RLock()
LOOP:
	for _, pipelineRTable := range m.rtable {
		for pattern, methods := range pipelineRTable {
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
	m.RUnlock()

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

func (m *paramMux) sameRoutingRulesInDifferentGeneration(entry *plugins.HTTPMuxEntry, entryChecking *plugins.HTTPMuxEntry) bool {
	return entry.Path == entryChecking.Path &&
		entry.Method == entryChecking.Method &&
		entry.Instance.Name() == entryChecking.Instance.Name() &&
		entry.Instance != entryChecking.Instance
}

func (m *paramMux) validate(entryValidating *plugins.HTTPMuxEntry) error {
	// we do not allow to register static path and parametric path on the same segment, for example
	// client can not register the patterns `/user/jack` and `/user/{user}` on the same http method at the same time.
	for pipeline, pipelineRTable := range m.rtable {
		for p, methods := range pipelineRTable {
			for _, entry := range methods {
				if m.sameRoutingRulesInDifferentGeneration(entry, entryValidating) {
					return nil
				}
			}
			dup, err := duplicatedPath(p, entryValidating.Path)
			if err != nil {
				return err
			}

			if dup && methods[entryValidating.Method] != nil {
				return fmt.Errorf("duplicated handler on %s %s in pipeline %s",
					entryValidating.Method, entryValidating, pipeline)
			}
		}
	}

	return nil
}

func (m *paramMux) _locklessAddFunc(pipeline string, entryAdding *plugins.HTTPMuxEntry) {
	if m.rtable == nil {
		m.rtable = make(map[string]map[string]map[string]*plugins.HTTPMuxEntry)
	}

	pipelineRTable, exists := m.rtable[pipeline]
	if !exists {
		pipelineRTable = make(map[string]map[string]*plugins.HTTPMuxEntry)
		m.rtable[pipeline] = pipelineRTable
	}

	path_rules, exists := pipelineRTable[entryAdding.Path]
	if !exists {
		path_rules = make(map[string]*plugins.HTTPMuxEntry)
		pipelineRTable[entryAdding.Path] = path_rules
	}

	entry, exists := path_rules[entryAdding.Method]
	if exists { // just change generation
		entry.Instance = entryAdding.Instance
		entry.Headers = entryAdding.Headers
		entry.Handler = entryAdding.Handler
	} else {
		path_rules[entryAdding.Method] = entryAdding
	}
}

func (m *paramMux) checkValidity(e *plugins.HTTPMuxEntry) error {
	if e == nil {
		return fmt.Errorf("empty entry")
	}
	ts := strings.TrimSpace
	e.Path = ts(e.Path)
	e.Method = ts(e.Method)

	if e.Instance == nil {
		return fmt.Errorf("empty instance")
	}
	if e.Handler == nil {
		return fmt.Errorf("empty handler")
	}
	if e.Path == "" {
		return fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(e.Path) {
		return fmt.Errorf("invalid relative path")
	}

	if e.Method == "" {
		return fmt.Errorf("empty method")
	}
	_, ok := supportedMethods[e.Method]
	if !ok {
		return fmt.Errorf("unsupported http method %s", e.Method)
	}

	return nil
}

func (m *paramMux) AddFunc(pipeline string, entryAdding *plugins.HTTPMuxEntry) error {
	ts := strings.TrimSpace
	pipeline = ts(pipeline)
	if pipeline == "" {
		return fmt.Errorf("empty pipeline name")
	}

	m.checkValidity(entryAdding)

	m.Lock()
	defer m.Unlock()

	err := m.validate(entryAdding)
	if err != nil {
		return err
	}

	m._locklessAddFunc(pipeline, entryAdding)

	return nil
}

func (m *paramMux) AddFuncs(pipeline string, entriesAdding []*plugins.HTTPMuxEntry) error {
	ts := strings.TrimSpace
	pipeline = ts(pipeline)
	if len(pipeline) == 0 {
		return fmt.Errorf("empty pipeline name")
	}

	if len(entriesAdding) == 0 {
		return fmt.Errorf("empty pipeline route table")
	}

	m.Lock()
	defer m.Unlock()

	// full validation first
	for _, entry := range entriesAdding {
		err := m.validate(entry)
		if err != nil {
			return err
		}
	}

	for _, entry := range entriesAdding {
		m._locklessAddFunc(pipeline, entry)
	}

	return nil
}

func (m *paramMux) DeleteFunc(pipeline string, entryDeleting *plugins.HTTPMuxEntry) {
	ts := strings.TrimSpace
	pipeline = ts(pipeline)
	if pipeline == "" {
		return
	}

	m.Lock()
	defer m.Unlock()

	pipelineRTable, exists := m.rtable[pipeline]
	if !exists {
		return
	}
	methods, exists := pipelineRTable[entryDeleting.Path]
	if !exists {
		return
	}
	entry, exists := methods[entryDeleting.Method]
	if !exists {
		return
	}

	if m.sameRoutingRulesInDifferentGeneration(entry, entryDeleting) {
		return // The older one has already been covered.
	} else {
		delete(pipelineRTable[entryDeleting.Path], entryDeleting.Method)
		if len(pipelineRTable[entryDeleting.Path]) == 0 {
			delete(pipelineRTable, entryDeleting.Path)
		}
		if len(pipelineRTable) == 0 {
			delete(m.rtable, pipeline)
		}
	}
}

func (m *paramMux) DeleteFuncs(pipeline string) []*plugins.HTTPMuxEntry {
	ts := strings.TrimSpace
	pipeline = ts(pipeline)
	if pipeline == "" {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	pipelineRTable := m.rtable[pipeline]

	var entries []*plugins.HTTPMuxEntry
	for _, methods := range pipelineRTable {
		for _, entry := range methods {
			entries = append(entries, entry)
		}
	}

	delete(m.rtable, pipeline)
	return entries
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
