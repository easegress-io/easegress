// Mux testing utils.
package plugins

import (
	"fmt"
	"testing"

	"github.com/megaease/easegateway/pkg/pipelines"
)

type fakePipelineContext struct {
	pipelineName string
	pluginNames  []string
}

func (pc *fakePipelineContext) PipelineName() string                     { return pc.pipelineName }
func (pc *fakePipelineContext) PluginNames() []string                    { return pc.pluginNames }
func (pc *fakePipelineContext) Statistics() pipelines.PipelineStatistics { return nil }
func (pc *fakePipelineContext) DataBucket(pluginName, pluginInstanceId string) pipelines.PipelineContextDataBucket {
	return nil
}
func (pc *fakePipelineContext) DeleteBucket(pluginName, pluginInstanceId string) pipelines.PipelineContextDataBucket {
	return nil
}
func (pc *fakePipelineContext) CommitCrossPipelineRequest(request *pipelines.DownstreamRequest, cancel <-chan struct{}) error {
	return nil
}
func (pc *fakePipelineContext) ClaimCrossPipelineRequest(cancel <-chan struct{}) *pipelines.DownstreamRequest {
	return nil
}
func (pc *fakePipelineContext) CrossPipelineWIPRequestsCount(upstreamPipelineName string) int {
	return 0
}
func (pc *fakePipelineContext) TriggerSourceInput(getterName string, getter pipelines.SourceInputQueueLengthGetter) {
}
func (pc *fakePipelineContext) Close() {}

func mockPipelineContext(pname string, pluginNames []string) pipelines.PipelineContext {
	return &fakePipelineContext{
		pipelineName: pname,
		pluginNames:  pluginNames,
	}
}

func mustNewREMux(t *testing.T) *reMux {
	muxConf := &reMuxConfig{
		CacheKeyComplete: false,
		CacheMaxCount:    1024,
	}
	m, err := newREMux(muxConf)
	if err != nil {
		t.Fatalf("new re mux failed: %v", err)
	}
	return m
}

func mustNewParamMux(t *testing.T) *paramMux {
	muxConf := &paramMuxConfig{}
	m, err := newParamMux(muxConf)
	if err != nil {
		t.Fatalf("new param mux failed: %v", err)
	}
	return m
}

func mustGetREMuxResult(t *testing.T, m *reMux,
	pipelineEntries map[string][]*HTTPMuxEntry,
	priorityEntries []*HTTPMuxEntry) {

	if len(pipelineEntries) != len(m.pipelineEntries) {
		goto FAILED
	}
	for pname, entries := range pipelineEntries {
		if len(m.pipelineEntries[pname]) != len(entries) {
			goto FAILED
		}
		for i := 0; i < len(entries); i++ {
			if !m.sameEntry(m.pipelineEntries[pname][i], entries[i]) {
				goto FAILED
			}
		}
	}

	if len(priorityEntries) != len(m.priorityEntries) {
		goto FAILED
	}
	for i := 0; i < len(priorityEntries); i++ {
		if !m.sameEntry(m.priorityEntries[i], priorityEntries[i]) {
			goto FAILED
		}
	}

	return

FAILED:
	fmt.Printf("want pipelineEntries %#v, priorityEntries %#v, got:\n",
		pipelineEntries, priorityEntries)
	m.dump()
	t.Fail()
}

func mustGetParamMuxResult(t *testing.T, m *paramMux,
	rtable map[string]map[string]map[string]*HTTPMuxEntry) {

	if len(m.rtable) != len(m.rtable) {
		goto FAILED
	}
	for pname, pipelineRTable := range rtable {
		if len(m.rtable[pname]) != len(pipelineRTable) {
			goto FAILED
		}
		for path, methods := range pipelineRTable {
			if len(m.rtable[pname][path]) != len(methods) {
				goto FAILED
			}
			for method, entry := range methods {
				if !m.sameEntry(m.rtable[pname][path][method], entry) {
					goto FAILED
				}
			}
		}
	}

	return

FAILED:
	fmt.Printf("want rtable %#v got:\n", rtable)
	m.dump()
	t.Fail()
}

func mockHTTPMuxEntry(scheme, host, port, path, query, fragment, method string,
	priority uint32, instance *httpInput) *HTTPMuxEntry {
	return &HTTPMuxEntry{
		HTTPURLPattern: HTTPURLPattern{
			Scheme:   scheme,
			Host:     host,
			Port:     port,
			Path:     path,
			Query:    query,
			Fragment: fragment,
		},
		Method:   method,
		Priority: priority,
		Instance: instance,
		Headers:  instance.conf.HeadersEnum,
		Handler:  instance.handler,
	}

}

func mockHTTPInput(name string) *httpInput {
	return &httpInput{
		conf: &HTTPInputConfig{
			PluginCommonConfig: PluginCommonConfig{
				Name: name,
			},
		},
	}
}

func mustAddFunc(t *testing.T, m HTTPMux,
	ctx pipelines.PipelineContext, entry *HTTPMuxEntry) {
	err := m.AddFunc(ctx, entry)
	if err != nil {
		t.Fatalf("add entry %#v failed: %v", entry, err)
	}
}

func mustAddFuncs(t *testing.T, m HTTPMux,
	ctx pipelines.PipelineContext, pipelineEntries []*HTTPMuxEntry) {
	err := m.AddFuncs(ctx, pipelineEntries)
	if err != nil {
		t.Fatalf("add entries %#v failed: %v", pipelineEntries, err)
	}
}

func mustDeleteFunc(t *testing.T, m HTTPMux,
	ctx pipelines.PipelineContext, entry *HTTPMuxEntry) {
	m.DeleteFunc(ctx, entry)
}

func mustDeleteFuncs(t *testing.T, m HTTPMux,
	ctx pipelines.PipelineContext) []*HTTPMuxEntry {
	return m.DeleteFuncs(ctx)
}
