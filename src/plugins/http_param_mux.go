package plugins

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"

	"common"
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
	//         pname      path       method
	rtable map[string]map[string]map[string]*plugins.HTTPMuxEntry
}

func newParamMux(conf *paramMuxConfig) (*paramMux, error) {
	return new(paramMux), nil
}

func (m *paramMux) dump() {
	m.RLock()
	fmt.Printf("%#v\n", m.rtable)
	m.RUnlock()
}
func (m *paramMux) ServeHTTP(ctx plugins.HTTPCtx) {
	match := false
	wrongMethod := false
	var path_params map[string]string
	var e *plugins.HTTPMuxEntry

	serveStartAt := common.Now()
	header := ctx.RequestHeader()
	m.RLock()
LOOP:
	for _, pipelineRTable := range m.rtable {
		for pattern, methods := range pipelineRTable {
			match, path_params, _ = parsePath(header.Path(), pattern)
			if match {
				e = methods[header.Method()]
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
			ctx.SetStatusCode(http.StatusMethodNotAllowed)
		} else {
			ctx.SetStatusCode(http.StatusNotFound)
		}

		return
	}

	errKeys := make([]string, 0)
	for key, valuesEnum := range e.Headers {
		errKeys = append(errKeys, key)
		v := header.Get(key)
		for _, valueEnum := range valuesEnum {
			if v == valueEnum {
				errKeys = errKeys[:len(errKeys)-1]
				break
			}
		}
	}
	if len(errKeys) > 0 {
		headerErr := getHeaderError(errKeys...)
		ctx.SetStatusCode(headerErr.Code)
		return
	}

	e.Handler(ctx, path_params, common.Since(serveStartAt))
}

func (m *paramMux) samePluginDifferentGeneration(entry *plugins.HTTPMuxEntry, entryChecking *plugins.HTTPMuxEntry) bool {
	return entry.Instance.Name() == entryChecking.Instance.Name() &&
		entry.Instance != entryChecking.Instance
}

func (m *paramMux) sameEntry(entry *plugins.HTTPMuxEntry, entryChecking *plugins.HTTPMuxEntry) bool {
	return entry.Instance.Name() == entryChecking.Instance.Name() &&
		entry.Instance == entryChecking.Instance &&
		entry.HTTPURLPattern == entryChecking.HTTPURLPattern &&
		entry.Method == entryChecking.Method &&
		entry.Priority == entryChecking.Priority
}

func (m *paramMux) validate(entryValidating *plugins.HTTPMuxEntry) error {
	// we do not allow to register static path and parametric path on the same segment, for example
	// client can not register the patterns `/user/jack` and `/user/{user}` on the same http method at the same time.
	for pname, pipelineRTable := range m.rtable {
		for p, methods := range pipelineRTable {
			for _, entry := range methods {
				if m.samePluginDifferentGeneration(entry, entryValidating) {
					return nil
				}
			}
			dup, err := duplicatedPath(p, entryValidating.Path)
			if err != nil {
				return err
			}

			if dup && methods[entryValidating.Method] != nil {
				return fmt.Errorf("duplicated handler on %s %s in pipeline %s",
					entryValidating.Method, entryValidating, pname)
			}
		}
	}

	return nil
}

func (m *paramMux) _locklessAddFunc(pname string, entryAdding *plugins.HTTPMuxEntry) {
	if m.rtable == nil {
		m.rtable = make(map[string]map[string]map[string]*plugins.HTTPMuxEntry)
	}

	pipelineRTable, exists := m.rtable[pname]
	if !exists {
		pipelineRTable = make(map[string]map[string]*plugins.HTTPMuxEntry)
		m.rtable[pname] = pipelineRTable
	}

	path_rules, exists := pipelineRTable[entryAdding.Path]
	if !exists {
		path_rules = make(map[string]*plugins.HTTPMuxEntry)
		pipelineRTable[entryAdding.Path] = path_rules
	}

	// Not only adding a new one but also covering existed entry.
	path_rules[entryAdding.Method] = entryAdding
}

func (m *paramMux) _locklessDeleteFunc(pname string, entryDeleting *plugins.HTTPMuxEntry) {
	pipelineRTable, exists := m.rtable[pname]
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

	// Can't delete the same routing rule from newer plugin.
	if m.sameEntry(entry, entryDeleting) {
		delete(pipelineRTable[entryDeleting.Path], entryDeleting.Method)
		if len(pipelineRTable[entryDeleting.Path]) == 0 {
			delete(pipelineRTable, entryDeleting.Path)
		}
		if len(pipelineRTable) == 0 {
			delete(m.rtable, pname)
		}
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

func (m *paramMux) AddFunc(ctx pipelines.PipelineContext, entryAdding *plugins.HTTPMuxEntry) error {
	pname := ctx.PipelineName()
	if pname == "" {
		return fmt.Errorf("empty pipeline name")
	}

	m.checkValidity(entryAdding)

	if !common.StrInSlice(entryAdding.Instance.Name(), ctx.PluginNames()) {
		return fmt.Errorf("plugin %s doesn't exist in pipeline %s",
			entryAdding.Instance.Name(), pname)
	}

	m.Lock()
	defer m.Unlock()

	err := m.validate(entryAdding)
	if err != nil {
		return err
	}

	// Clean entries from the same plugin but different genrations.
	for pname, pipelineRTable := range m.rtable {
		var entriesToClean []*plugins.HTTPMuxEntry
		for _, methods := range pipelineRTable {
			for _, entry := range methods {
				if m.samePluginDifferentGeneration(entry, entryAdding) {
					entriesToClean = append(entriesToClean, entry)
				}
			}
		}
		for _, entry := range entriesToClean {
			m._locklessDeleteFunc(pname, entry)
		}
	}

	m._locklessAddFunc(pname, entryAdding)

	return nil
}

func (m *paramMux) AddFuncs(ctx pipelines.PipelineContext, entriesAdding []*plugins.HTTPMuxEntry) error {
	pname := ctx.PipelineName()
	if len(pname) == 0 {
		return fmt.Errorf("empty pipeline name")
	}

	if len(entriesAdding) == 0 {
		return fmt.Errorf("empty pipeline route table")
	}

	m.Lock()
	defer m.Unlock()

	// Clean outdated entries.
	var entriesAddingExcludeOutdated []*plugins.HTTPMuxEntry
	for _, entryAdding := range entriesAdding {
		needExclude := false
	LOOP:
		for _, pipelineRTable := range m.rtable {
			for _, methods := range pipelineRTable {
				for _, entry := range methods {
					if m.samePluginDifferentGeneration(
						entry, entryAdding) {
						needExclude = true
						break LOOP
					}
				}
			}
		}
		if !needExclude {
			entriesAddingExcludeOutdated = append(
				entriesAddingExcludeOutdated, entryAdding)
		}
	}
	entriesAdding = entriesAddingExcludeOutdated

	// Clean entries of dead plugins.
	pluginNames := ctx.PluginNames()
	var entriesAddingExcludeDead []*plugins.HTTPMuxEntry
	for _, entry := range entriesAdding {
		if common.StrInSlice(entry.Instance.Name(), pluginNames) {
			entriesAddingExcludeDead = append(
				entriesAddingExcludeDead, entry)
		}
	}
	entriesAdding = entriesAddingExcludeDead

	// full validation first
	for _, entry := range entriesAdding {
		err := m.validate(entry)
		if err != nil {
			return err
		}
	}

	for _, entry := range entriesAdding {
		m._locklessAddFunc(pname, entry)
	}

	return nil
}

func (m *paramMux) DeleteFunc(ctx pipelines.PipelineContext, entryDeleting *plugins.HTTPMuxEntry) {
	pname := ctx.PipelineName()
	if pname == "" {
		return
	}

	m.Lock()
	defer m.Unlock()

	m._locklessDeleteFunc(pname, entryDeleting)
}

func (m *paramMux) DeleteFuncs(ctx pipelines.PipelineContext) []*plugins.HTTPMuxEntry {
	pname := ctx.PipelineName()
	if pname == "" {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	pipelineRTable := m.rtable[pname]

	var entries []*plugins.HTTPMuxEntry
	for _, methods := range pipelineRTable {
		for _, entry := range methods {
			entries = append(entries, entry)
		}
	}

	delete(m.rtable, pname)
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
