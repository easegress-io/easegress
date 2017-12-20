package plugins

import (
	"testing"

	"github.com/hexdecteam/easegateway-types/plugins"
)

func TestREMuxDifferentGenerations(t *testing.T) {
	m := mustNewREMux(t)
	pluginA := "plugin-a"
	pluginA1 := mockHTTPInput(pluginA)
	pluginA2 := mockHTTPInput(pluginA)
	ctx1 := mockPipelineContext("pipeline-1", []string{pluginA})
	entryA1_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA1)
	entryA1_2 := mockHTTPMuxEntry("", "", "", "/a", "", "", "POST", 0, pluginA1)
	entryA2_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA2)
	entryA2_2 := mockHTTPMuxEntry("", "", "", "/a", "", "", "POST", 0, pluginA2)
	entryA2_3 := mockHTTPMuxEntry("", "", "", "/a", "", "", "PUT", 0, pluginA2)

	mustAddFunc(t, m, ctx1, entryA1_1)
	mustAddFunc(t, m, ctx1, entryA1_2)
	mustAddFunc(t, m, ctx1, entryA2_1)
	resultPipelineEntries := map[string][]*plugins.HTTPMuxEntry{
		ctx1.PipelineName(): {entryA2_1},
	}
	resultPriorityEntries := []*plugins.HTTPMuxEntry{entryA2_1}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)

	mustAddFunc(t, m, ctx1, entryA2_2)
	mustAddFunc(t, m, ctx1, entryA2_3)
	resultPipelineEntries = map[string][]*plugins.HTTPMuxEntry{
		ctx1.PipelineName(): {entryA2_1, entryA2_2, entryA2_3},
	}
	resultPriorityEntries = []*plugins.HTTPMuxEntry{entryA2_1, entryA2_2, entryA2_3}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)
}

func TestREMuxCleanOutdatedEntries(t *testing.T) {
	m := mustNewREMux(t)
	pluginA := "plugin-a"
	pluginA1 := mockHTTPInput(pluginA)
	pluginA2 := mockHTTPInput(pluginA)
	ctx1 := mockPipelineContext("pipeline-1", []string{pluginA})
	ctx2 := mockPipelineContext("pipeline-2", []string{pluginA})
	entryA1_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA1)
	entryA1_2 := mockHTTPMuxEntry("", "", "", "/a", "", "", "POST", 0, pluginA1)
	entryA2_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA2)

	mustAddFunc(t, m, ctx1, entryA1_1)
	mustAddFunc(t, m, ctx1, entryA1_2)
	pipelineEntries := mustDeleteFuncs(t, m, ctx1)
	// NOTICE: Even the plugin comes back from a different pipeline, the older rules
	// of it (same plugin name) will be cleaned too.
	mustAddFunc(t, m, ctx2, entryA2_1)
	mustAddFuncs(t, m, ctx2, pipelineEntries)

	resultPipelineEntries := map[string][]*plugins.HTTPMuxEntry{
		ctx2.PipelineName(): {entryA2_1},
	}
	resultPriorityEntries := []*plugins.HTTPMuxEntry{entryA2_1}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)
}

func TestCleanDeadEntries(t *testing.T) {
	m := mustNewREMux(t)
	pluginA := "plugin-a"
	pluginB := "plugin-b"
	pluginA1 := mockHTTPInput(pluginA)
	pluginB1 := mockHTTPInput(pluginB)
	ctx1_1 := mockPipelineContext("pipeline-1", []string{pluginA, pluginB})
	ctx1_2 := mockPipelineContext("pipeline-1", []string{pluginA})
	entryA1_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA1)
	entryB1_1 := mockHTTPMuxEntry("", "", "", "/b", "", "", "GET", 0, pluginB1)
	entryB1_2 := mockHTTPMuxEntry("", "", "", "/b", "", "", "POST", 0, pluginB1)

	mustAddFunc(t, m, ctx1_1, entryA1_1)
	mustAddFunc(t, m, ctx1_1, entryB1_1)
	mustAddFunc(t, m, ctx1_1, entryB1_2)
	pipelineEntries := mustDeleteFuncs(t, m, ctx1_1)
	// NOTICE: The absence of pluginB in ctx2 leads to clean all entryB*.
	m.AddFuncs(ctx1_2, pipelineEntries)

	resultPipelineEntries := map[string][]*plugins.HTTPMuxEntry{
		ctx1_2.PipelineName(): {entryA1_1},
	}
	resultPriorityEntries := []*plugins.HTTPMuxEntry{entryA1_1}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)
}

func TestREMuxFatigue(t *testing.T) {
	m := mustNewREMux(t)
	pluginA := "plugin-a"
	pluginB := "plugin-b"
	pluginC := "plugin-c"
	pluginD := "plugin-d"
	pluginA1 := mockHTTPInput(pluginA)
	pluginA2 := mockHTTPInput(pluginA)
	pluginB1 := mockHTTPInput(pluginB)
	pluginB2 := mockHTTPInput(pluginB)
	pluginC1 := mockHTTPInput(pluginC)
	pluginC2 := mockHTTPInput(pluginC)
	pluginC3 := mockHTTPInput(pluginC)
	pluginD1 := mockHTTPInput(pluginD)
	pluginD2 := mockHTTPInput(pluginD)
	pluginD3 := mockHTTPInput(pluginD)
	pluginD4 := mockHTTPInput(pluginD)

	ctx1 := mockPipelineContext("pipeline-1", []string{pluginA, pluginB, pluginC})
	ctx2 := mockPipelineContext("pipeline-2", []string{pluginD})
	// add entry
	entryA1_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA1)
	entryA1_2 := mockHTTPMuxEntry("", "", "", "/a", "", "", "POST", 0, pluginA1)
	entryA1_3 := mockHTTPMuxEntry("", "", "", "/a", "", "", "PUT", 0, pluginA1)
	entryA2_1 := mockHTTPMuxEntry("", "", "", "/a", "", "", "GET", 0, pluginA2)
	entryA2_2 := mockHTTPMuxEntry("", "", "", "/a", "", "", "POST", 0, pluginA2)
	messA := func() {
		mustAddFunc(t, m, ctx1, entryA1_1)
		mustAddFunc(t, m, ctx1, entryA1_2)
		mustAddFunc(t, m, ctx1, entryA1_3)
		mustAddFunc(t, m, ctx1, entryA2_1)
		mustDeleteFunc(t, m, ctx1, entryA1_1)
		mustDeleteFunc(t, m, ctx1, entryA1_2)
		mustAddFunc(t, m, ctx1, entryA2_2)
		mustDeleteFunc(t, m, ctx1, entryA1_3)
	}

	// delete entry
	entryB1_1 := mockHTTPMuxEntry("", "", "", "/b", "", "", "GET", 1, pluginB1)
	entryB1_2 := mockHTTPMuxEntry("", "", "", "/b", "", "", "DELETE", 1, pluginB1)
	entryB2_1 := mockHTTPMuxEntry("", "", "", "/b", "", "", "GET", 1, pluginB2)
	messB := func() {
		mustAddFunc(t, m, ctx1, entryB1_1)
		mustAddFunc(t, m, ctx1, entryB1_2)
		mustAddFunc(t, m, ctx1, entryB2_1)
		mustDeleteFunc(t, m, ctx1, entryB1_1)
		// mock missing mustDeleteFunc(t, m, ctx1, entryB1_1)
	}

	// no change
	entryC1_1 := mockHTTPMuxEntry("", "", "", "/c", "", "", "GET", 1, pluginC1)
	entryC1_2 := mockHTTPMuxEntry("", "", "", "/c", "", "", "HEAD", 1, pluginC1)
	entryC2_1 := mockHTTPMuxEntry("", "", "", "/c", "", "", "GET", 1, pluginC2)
	entryC2_2 := mockHTTPMuxEntry("", "", "", "/c", "", "", "HEAD", 1, pluginC2)
	entryC3_1 := mockHTTPMuxEntry("", "", "", "/c", "", "", "GET", 1, pluginC3)
	entryC3_2 := mockHTTPMuxEntry("", "", "", "/c", "", "", "HEAD", 1, pluginC3)
	messC := func() {
		mustAddFunc(t, m, ctx1, entryC1_1)
		mustAddFunc(t, m, ctx1, entryC1_2)
		pipelineEntries1 := mustDeleteFuncs(t, m, ctx1)
		mustDeleteFunc(t, m, ctx1, entryC1_1)
		mustAddFunc(t, m, ctx1, entryC2_1)
		mustAddFuncs(t, m, ctx1, pipelineEntries1)
		// mock missing mustDeleteFunc(t, m, ctx1, entryC1_2)
		mustAddFunc(t, m, ctx1, entryC2_2)
		mustAddFunc(t, m, ctx1, entryC3_1)
		mustDeleteFunc(t, m, ctx1, entryC2_1)
		mustAddFunc(t, m, ctx1, entryC3_2)
		mustDeleteFunc(t, m, ctx1, entryC2_2)
	}

	// mess up
	entryD1_1 := mockHTTPMuxEntry("", "", "", "/d", "", "", "GET", 2, pluginD1)
	entryD2_1 := mockHTTPMuxEntry("", "", "", "/dd", "", "", "POST", 20, pluginD2)
	entryD3_1 := mockHTTPMuxEntry("", "", "", "/ddd", "", "", "PUT", 200, pluginD3)
	entryD4_1 := mockHTTPMuxEntry("", "", "", "/dddd", "", "", "GET", 2000, pluginD4)
	entryD4_2 := mockHTTPMuxEntry("", "", "", "/dddd", "", "", "POST", 2000, pluginD4)
	entryD4_3 := mockHTTPMuxEntry("", "", "", "/dddd", "", "", "PUT", 2000, pluginD4)
	messD := func() {
		mustAddFunc(t, m, ctx2, entryD1_1)
		mustAddFunc(t, m, ctx2, entryD2_1)
		// mock missing mustDeleteFunc(t, m, ctx2, entryD1_1)
		mustAddFunc(t, m, ctx2, entryD3_1)
		mustDeleteFunc(t, m, ctx2, entryD3_1)
		mustDeleteFunc(t, m, ctx2, entryD2_1)
		mustAddFunc(t, m, ctx2, entryD4_1)
		mustAddFunc(t, m, ctx2, entryD4_2)
		mustAddFunc(t, m, ctx2, entryD4_3)
	}

	messA()
	messB()
	messC()
	messD()

	resultPipelineEntries := map[string][]*plugins.HTTPMuxEntry{
		ctx1.PipelineName(): {
			entryA2_1, entryA2_2,
			entryB2_1,
			entryC3_1, entryC3_2,
		},
		ctx2.PipelineName(): {
			entryD4_1, entryD4_2, entryD4_3,
		},
	}
	resultPriorityEntries := []*plugins.HTTPMuxEntry{
		entryA2_1, entryA2_2,
		entryB2_1,
		entryC3_1, entryC3_2,
		entryD4_1, entryD4_2, entryD4_3,
	}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)

	////

	pipelineEntries := mustDeleteFuncs(t, m, ctx1)
	// delete pluginA pluginC
	ctx1 = mockPipelineContext("pipeline-1", []string{pluginB})
	mustAddFuncs(t, m, ctx1, pipelineEntries)

	resultPipelineEntries = map[string][]*plugins.HTTPMuxEntry{
		ctx1.PipelineName(): {
			entryB2_1,
		},
		ctx2.PipelineName(): {
			entryD4_1, entryD4_2, entryD4_3,
		},
	}
	resultPriorityEntries = []*plugins.HTTPMuxEntry{
		entryB2_1,
		entryD4_1, entryD4_2, entryD4_3,
	}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)
}
