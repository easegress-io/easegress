package plugins

import (
	"bytes"
	"fmt"
	"logger"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/allegro/bigcache"
	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/ugorji/go/codec"

	"common"
	eghttp "http"
)

// For quickly substituting another implementation.
var compile = regexp.Compile

type reMuxConfig struct {
	CacheKeyComplete bool   `json:"cache_key_complete"`
	CacheMaxCount    uint32 `json:"cache_max_count"`
}

type reMux struct {
	sync.RWMutex
	// The key is pipeline name.
	pipelineEntries map[string][]*reEntry
	// The priorityEntries sorts by priority from smaller to bigger.
	priorityEntries []*reEntry

	cache *bigcache.BigCache

	conf *reMuxConfig
}

type reEntry struct {
	*plugins.HTTPMuxEntry

	// Not necessary but for information complete.
	urlLiteral string
	urlRE      *regexp.Regexp
}

func (entry *reEntry) String() string {
	return fmt.Sprintf("[urlRE:%p, method:%s, priority:%d, instance:%p, headers: %v, handler:%p, pattern:%s, urlLiteral:%s]",
		entry.urlRE, entry.Method, entry.Priority, entry.Instance, entry.Headers, entry.Handler,
		fmt.Sprintf("%s://%s:%s%s?%s#%s", entry.Scheme, entry.Host, entry.Port,
			entry.Path, entry.Query, entry.Fragment), entry.urlLiteral)
}

func newREMux(conf *reMuxConfig) (*reMux, error) {
	cacheConfig := bigcache.Config{
		Shards: int(common.NextNumberPowerOf2(uint64(conf.CacheMaxCount))),
	}
	cache, err := bigcache.NewBigCache(cacheConfig)
	if err != nil {
		logger.Errorf("[BUG: new big cache failed: %v]", err)
		return nil, err
	}
	return &reMux{
		pipelineEntries: make(map[string][]*reEntry),
		cache:           cache,
		conf:            conf,
	}, nil
}

type cacheValue struct {
	PriorityEntriesIndex int
	URLParams            map[string]string
}

func (m *reMux) clearCache() {
	err := m.cache.Reset()
	if err != nil {
		logger.Errorf("[BUG: reset cache failed: %v]", err)
	}
}

func (m *reMux) addCache(key string, value *cacheValue) {
	buff := bytes.NewBuffer(nil)
	encoder := codec.NewEncoder(buff, &codec.MsgpackHandle{})
	err := encoder.Encode(*value)
	if err != nil {
		logger.Errorf("[BUG: msgpack encode failed: %v]", err)
		return
	}

	err = m.cache.Set(key, buff.Bytes())
	if err != nil {
		logger.Errorf("[BUG: cache set failed: %v]", err)
		return
	}
}

func (m *reMux) getCache(key string) *cacheValue {
	buff, err := m.cache.Get(key)
	if err != nil {
		return nil
	}

	value := cacheValue{}
	decoder := codec.NewDecoder(bytes.NewReader(buff), &codec.MsgpackHandle{})
	err = decoder.Decode(&value)
	if err != nil {
		logger.Errorf("[BUG: msgpack decode failed: %v]", err)
		return nil
	}

	return &value
}

func (m *reMux) dump() {
	m.RLock()
	fmt.Println("pipelineEntries:")
	for pipeline, entries := range m.pipelineEntries {
		fmt.Println(pipeline)
		for _, entry := range entries {
			fmt.Printf("\t%s\n", entry)
		}
	}
	fmt.Println("priorityEntries:")
	for _, entry := range m.priorityEntries {
		fmt.Printf("\t%s\n", entry)
	}
	m.RUnlock()
}

func (m *reMux) ServeHTTP(c plugins.HTTPCtx) {
	serveStartAt := common.Now()
	header := c.RequestHeader()
	var requestURL string
	if m.conf.CacheKeyComplete {
		requestURL = m.generateCompleteRequestURL(header)
	} else {
		requestURL = m.generatePathEndingRequestURL(header)
	}

	matchURL := false
	matchMethod := false
	urlParams := make(map[string]string)
	var entryServing *plugins.HTTPMuxEntry

	keyCache := fmt.Sprintf("%s %s", header.Method(), requestURL)
	valueCache := m.getCache(keyCache)
	m.RLock()
	if valueCache != nil && valueCache.PriorityEntriesIndex < len(m.priorityEntries) {
		matchURL = true
		matchMethod = true
		urlParams = valueCache.URLParams
		entryServing = m.priorityEntries[valueCache.PriorityEntriesIndex].HTTPMuxEntry
		m.RUnlock()
	} else {
		for priorityEntriesIndex, entry := range m.priorityEntries {
			pathValues := entry.urlRE.FindStringSubmatch(requestURL)
			if pathValues != nil {
				matchURL = true
				for i, subName := range entry.urlRE.SubexpNames() {
					if len(subName) != 0 {
						urlParams[subName] = pathValues[i]
					}
				}

				if header.Method() == entry.Method {
					matchMethod = true
					entryServing = entry.HTTPMuxEntry
					m.addCache(keyCache, &cacheValue{
						PriorityEntriesIndex: priorityEntriesIndex,
						URLParams:            urlParams,
					})
					break
				}
			}

			urlParams = make(map[string]string)
		}
		m.RUnlock()

		if !matchURL {
			c.SetStatusCode(http.StatusNotFound)
			return
		}
		if !matchMethod {
			c.SetStatusCode(http.StatusMethodNotAllowed)
			return
		}
	}

	errKeys := make([]string, 0)
	for key, valuesEnum := range entryServing.Headers {
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
		c.SetStatusCode(headerErr.Code)
		return
	}

	entryServing.Handler(c, urlParams, common.Since(serveStartAt))
}

func (m *reMux) AddFunc(ctx pipelines.PipelineContext, entryAdding *plugins.HTTPMuxEntry) error {
	pname := ctx.PipelineName()
	if pname == "" {
		return fmt.Errorf("empty pipeline name")
	}

	err := m.checkValidity(entryAdding)
	if err != nil {
		return err
	}

	if !common.StrInSlice(entryAdding.Instance.Name(), ctx.PluginNames()) {
		return fmt.Errorf("plugin %s doesn't exist in pipeline %s",
			entryAdding.Instance.Name(), pname)
	}

	m.Lock()
	defer m.Unlock()

	err = m._locklessCheckConflict(entryAdding)
	if err != nil {
		return err
	}

	// Clear entries with same plugin but different generation.
	// NOTICE: The change of m.pipelineEntries[pname] in _locklessDeleteFunc
	// does not effect the whole iteration, because it changes the address of
	// m.pipelineEntries[pname] to point a new slice but not the elements
	// in the original place.
	for _, entry := range m.pipelineEntries[pname] {
		if m.samePluginDifferentGeneration(entry, entryAdding) {
			m._locklessDeleteFunc(pname, entry.HTTPMuxEntry)
		}
	}

	m._locklessAddFunc(pname, entryAdding)

	m.clearCache()

	return nil
}

func (m *reMux) AddFuncs(ctx pipelines.PipelineContext, entriesAdding []*plugins.HTTPMuxEntry) error {
	pname := ctx.PipelineName()
	if pname == "" {
		return fmt.Errorf("empty pipeline name")
	}

	for _, entry := range entriesAdding {
		err := m.checkValidity(entry)
		if err != nil {
			return err
		}
	}

	m.Lock()
	defer m.Unlock()

	// Clean outdated enytries.
	var entriesAddingExcludeOutdated []*plugins.HTTPMuxEntry
	for _, entryAdding := range entriesAdding {
		needExclude := false
		for _, entry := range m.priorityEntries {
			if m.samePluginDifferentGeneration(entry, entryAdding) {
				needExclude = true
				break
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

	for _, entry := range entriesAdding {
		err := m._locklessCheckConflict(entry)
		if err != nil {
			return err
		}
	}

	for _, entry := range entriesAdding {
		m._locklessAddFunc(pname, entry)
	}

	m.clearCache()

	return nil
}

func (m *reMux) DeleteFunc(ctx pipelines.PipelineContext, entryDeleting *plugins.HTTPMuxEntry) {
	pname := ctx.PipelineName()
	if pname == "" {
		return
	}

	err := m.checkValidity(entryDeleting)
	if err != nil {
		return
	}

	m.Lock()
	defer m.Unlock()

	m._locklessDeleteFunc(pname, entryDeleting)

	m.clearCache()
}

func (m *reMux) DeleteFuncs(ctx pipelines.PipelineContext) []*plugins.HTTPMuxEntry {
	pname := ctx.PipelineName()
	if pname == "" {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	pipelineEntries := m.pipelineEntries[pname]
	if len(pipelineEntries) == 0 {
		return nil
	}
	delete(m.pipelineEntries, pname)

	var priorityEntries []*reEntry
	for _, entry := range m.priorityEntries {
		needDelete := false
		for _, entryDeleting := range pipelineEntries {
			if entry == entryDeleting {
				needDelete = true
				break
			}
		}
		if !needDelete {
			priorityEntries = append(priorityEntries, entry)

		}
	}
	m.priorityEntries = priorityEntries

	m.clearCache()

	var adaptionPipelineEntries []*plugins.HTTPMuxEntry
	for _, entry := range pipelineEntries {
		adaptionPipelineEntries = append(adaptionPipelineEntries, entry.HTTPMuxEntry)
	}

	return adaptionPipelineEntries
}

func (m *reMux) generateCompleteRequestURL(h plugins.Header) string {
	return h.FullURI()
}

func (m *reMux) generatePathEndingRequestURL(h plugins.Header) string {
	scheme := h.Scheme()
	host, _ /* port */, err := net.SplitHostPort(h.Host())
	if err != nil { // h.Host() doesn't contain port
		if h.Scheme() == "http" {
			host = h.Host() + ":" + eghttp.DefaultHTTPPort
		} else {
			host = h.Host() + ":" + eghttp.DefaultHTTPSPort
		}
	} else {
		host = h.Host()
	}
	return fmt.Sprintf(`%s://%s%s?#`,
		scheme, host, common.RemoveRepeatedByte(h.Path(), '/'))
}

func (m *reMux) generateREEntry(entryAdding *plugins.HTTPMuxEntry) *reEntry {
	entry := new(reEntry)
	entry.HTTPMuxEntry = entryAdding

	scheme := ""
	if entry.Scheme == "" {
		scheme = `(http|https)`
	} else {
		scheme = entryAdding.Scheme
	}

	host := ""
	if entry.Host == "" {
		host = `.*`
	} else {
		host = entryAdding.Host
	}

	port := ""
	if entry.Port == "" {
		port = `\d*`
	} else {
		port = entryAdding.Port
	}

	query := ""
	if entry.Query == "" {
		query = ".*"
	} else {
		query = entryAdding.Query
	}

	fragment := ""
	if entry.Fragment == "" {
		fragment = ".*"
	} else {
		fragment = entryAdding.Fragment
	}

	entry.urlLiteral = fmt.Sprintf(`^%s://%s:%s%s\?%s#%s$`,
		scheme, host, port, entryAdding.Path, query, fragment)
	entry.urlRE, _ = compile(entry.urlLiteral)

	return entry
}

func (m *reMux) checkValidity(e *plugins.HTTPMuxEntry) error {
	if e == nil {
		return fmt.Errorf("empty http mux entry")
	}
	ts := strings.TrimSpace
	e.Scheme = ts(e.Scheme)
	e.Host = ts(e.Host)
	e.Port = ts(e.Port)
	e.Path = ts(e.Path)
	e.Query = ts(e.Query)
	e.Fragment = ts(e.Fragment)
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
	_, err := compile(e.Path)
	if err != nil {
		return fmt.Errorf("compile regular expression path %s failed: %v", e.Path, err)
	}

	if e.Method == "" {
		return fmt.Errorf("empty method")
	}
	match := false
	for method := range supportedMethods {
		if method == e.Method {
			match = true
			break
		}
	}
	if !match {
		return fmt.Errorf("unsupported method: %s", e.Method)
	}

	if e.Scheme != "" {
		schemeRE, err := compile(fmt.Sprintf("^%s$", e.Scheme))
		if err != nil {
			return fmt.Errorf("invalid scheme: %v", err)
		}
		if !schemeRE.MatchString("http") && !schemeRE.MatchString("https") {
			return fmt.Errorf("invalid scheme: can't match http/https (case-sensitive)")
		}
	}

	if e.Host != "" {
		_, err := compile(e.Host)
		if err != nil {
			return fmt.Errorf("compile regular expression host %s failed: %v", e.Host, err)
		}
	}

	if e.Port != "" {
		_, err := compile(e.Port)
		if err != nil {
			return fmt.Errorf("compile regular expression port %s failed: %v", e.Port, err)
		}
	}

	if e.Query != "" {
		_, err := compile(e.Query)
		if err != nil {
			return fmt.Errorf("compile regular expression query %s failed: %v", e.Query, err)
		}
	}

	if e.Fragment != "" {
		_, err := compile(e.Fragment)
		if err != nil {
			return fmt.Errorf("compile regular expression query %s failed: %v", e.Fragment, err)
		}
	}

	return nil
}

func (m *reMux) _locklessCheckConflict(entryChecking *plugins.HTTPMuxEntry) error {
	for _, entry := range m.priorityEntries {
		if m.samePluginDifferentGeneration(entry, entryChecking) {
			continue
		}
		err := m._locklessCheckConflictHelper(entry, entryChecking)
		if err != nil {
			return fmt.Errorf("pattern %v is in conflict with existed pattern %v: %v",
				*entryChecking, *entry.HTTPMuxEntry, err)
		}
	}

	return nil
}

func (m *reMux) _locklessCheckConflictHelper(entry *reEntry, entryChecking *plugins.HTTPMuxEntry) error {
	dupPattern := entry.HTTPURLPattern == entryChecking.HTTPURLPattern
	dupPriority := entry.Priority == entryChecking.Priority

	if dupPattern && !dupPriority {
		return fmt.Errorf("url conflict: same url pattern with different priority(%d,%d)",
			entry.Priority, entryChecking.Priority)
	}

	if !dupPattern && dupPriority {
		conflict := true
		conflict = m.patternHasIntersection(entry.Scheme, entryChecking.Scheme)
		if !conflict {
			return nil
		}
		conflict = m.patternHasIntersection(entry.Host, entryChecking.Host)
		if !conflict {
			return nil
		}
		conflict = m.patternHasIntersection(entry.Port, entryChecking.Port)
		if !conflict {
			return nil
		}
		conflict = m.patternHasIntersection(entry.Path, entryChecking.Path)
		if !conflict {
			return nil
		}
		conflict = m.patternHasIntersection(entry.Query, entryChecking.Query)
		if !conflict {
			return nil
		}
		conflict = m.patternHasIntersection(entry.Fragment, entryChecking.Fragment)
		if !conflict {
			return nil
		}
		return fmt.Errorf("url conflict: same priority %d with url matching intersection", entry.Priority)
	}

	if dupPattern && dupPriority {
		if entry.Method == entryChecking.Method {
			return fmt.Errorf("method conflict: "+
				"same url pattern with same method: %s", entry.Method)
		}
	}

	// !dupPattern && !dupPriority
	return nil
}

func (m *reMux) patternHasIntersection(s1, s2 string) bool {
	if s1 == "" || s2 == "" || s1 == s2 {
		return true
	}

	re1, _ := compile(s1)
	re2, _ := compile(s2)
	lp1, complete1 := re1.LiteralPrefix()
	lp2, complete2 := re2.LiteralPrefix()

	if complete1 && complete2 {
		return lp1 == lp2
	}

	if complete1 {
		re2WithStartAnchor, _ := compile(fmt.Sprintf("^%s", s2))
		return re2WithStartAnchor.MatchString(lp1)
	}
	if complete2 {
		re1WithStartAnchor, _ := compile(fmt.Sprintf("^%s", s1))
		return re1WithStartAnchor.MatchString(lp2)
	}

	minLen := len(lp1)
	if len(lp2) < minLen {
		minLen = len(lp2)
	}
	if lp1[:minLen] == lp2[:minLen] {
		return true
	} else {
		return false
	}
}

func (m *reMux) _locklessAddFunc(pname string, entryAdding *plugins.HTTPMuxEntry) {
	entryNew := m.generateREEntry(entryAdding)
	m.pipelineEntries[pname] = append(m.pipelineEntries[pname], entryNew)

	added := false
	var priorityEntries []*reEntry
	for _, entry := range m.priorityEntries {
		if !added && entryNew.Priority < entry.Priority {
			priorityEntries = append(priorityEntries, entryNew)
			priorityEntries = append(priorityEntries, entry)
			added = true
		} else {
			priorityEntries = append(priorityEntries, entry)
		}
	}
	if !added {
		priorityEntries = append(priorityEntries, entryNew)
	}
	m.priorityEntries = priorityEntries
}

func (m *reMux) _locklessDeleteFunc(pname string, entryDeleting *plugins.HTTPMuxEntry) {
	var pipelineEntries []*reEntry
	var entryDeleted *reEntry
	for _, entry := range m.pipelineEntries[pname] {
		if m.sameEntry(entry, entryDeleting) {
			entryDeleted = entry
		} else {
			pipelineEntries = append(pipelineEntries, entry)
		}
	}
	m.pipelineEntries[pname] = pipelineEntries
	if len(m.pipelineEntries[pname]) == 0 {
		delete(m.pipelineEntries, pname)
	}

	if entryDeleted == nil {
		return
	}

	var priorityEntries []*reEntry
	for _, entry := range m.priorityEntries {
		if entry != entryDeleted {
			priorityEntries = append(priorityEntries, entry)
		}
	}
	m.priorityEntries = priorityEntries
}

func (m *reMux) samePluginDifferentGeneration(entry *reEntry, entryChecking *plugins.HTTPMuxEntry) bool {
	return entry.Instance.Name() == entryChecking.Instance.Name() &&
		entry.Instance != entryChecking.Instance
}

func (m *reMux) sameEntry(entry *reEntry, entryChecking *plugins.HTTPMuxEntry) bool {
	return entry.Instance.Name() == entryChecking.Instance.Name() &&
		entry.Instance == entryChecking.Instance &&
		entry.HTTPURLPattern == entryChecking.HTTPURLPattern &&
		entry.Method == entryChecking.Method &&
		entry.Priority == entryChecking.Priority
}
