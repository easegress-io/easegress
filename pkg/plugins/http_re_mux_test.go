package plugins

import (
	"testing"
)

var defaultReMux = &reMux{}

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
	resultPipelineEntries := map[string][]*HTTPMuxEntry{
		ctx1.PipelineName(): {entryA2_1},
	}
	resultPriorityEntries := []*HTTPMuxEntry{entryA2_1}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)

	mustAddFunc(t, m, ctx1, entryA2_2)
	mustAddFunc(t, m, ctx1, entryA2_3)
	resultPipelineEntries = map[string][]*HTTPMuxEntry{
		ctx1.PipelineName(): {entryA2_1, entryA2_2, entryA2_3},
	}
	resultPriorityEntries = []*HTTPMuxEntry{entryA2_1, entryA2_2, entryA2_3}
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

	resultPipelineEntries := map[string][]*HTTPMuxEntry{
		ctx2.PipelineName(): {entryA2_1},
	}
	resultPriorityEntries := []*HTTPMuxEntry{entryA2_1}
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

	resultPipelineEntries := map[string][]*HTTPMuxEntry{
		ctx1_2.PipelineName(): {entryA1_1},
	}
	resultPriorityEntries := []*HTTPMuxEntry{entryA1_1}
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

	resultPipelineEntries := map[string][]*HTTPMuxEntry{
		ctx1.PipelineName(): {
			entryA2_1, entryA2_2,
			entryB2_1,
			entryC3_1, entryC3_2,
		},
		ctx2.PipelineName(): {
			entryD4_1, entryD4_2, entryD4_3,
		},
	}
	resultPriorityEntries := []*HTTPMuxEntry{
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

	resultPipelineEntries = map[string][]*HTTPMuxEntry{
		ctx1.PipelineName(): {
			entryB2_1,
		},
		ctx2.PipelineName(): {
			entryD4_1, entryD4_2, entryD4_3,
		},
	}
	resultPriorityEntries = []*HTTPMuxEntry{
		entryB2_1,
		entryD4_1, entryD4_2, entryD4_3,
	}
	mustGetREMuxResult(t, m, resultPipelineEntries, resultPriorityEntries)
}

type generateURLTest struct {
	url string
	// generated
	header                Header
	expectedFullURL       string
	expectedPathEndingURL string
}

var generateURLTests = []generateURLTest{
	{
		url: "http://localhost:8080//r/megaease/easegateway//tags",
		// should remove repeated '/'
		expectedFullURL:       "http://localhost:8080/r/megaease/easegateway/tags?#",
		expectedPathEndingURL: "http://localhost:8080/r/megaease/easegateway/tags?#",
	},
	/* https url is hard to simulate, so we don't use https */
	{
		url: "http://www.megaease.com/r/megaease/easegateway/tags/server-0.1?foo=bar&com=true#head",
		// should add default port
		expectedFullURL:       "http://www.megaease.com:80/r/megaease/easegateway/tags/server-0.1?foo=bar&com=true#head",
		expectedPathEndingURL: "http://www.megaease.com:80/r/megaease/easegateway/tags/server-0.1?#",
	},
}

func testGenerateCompleteRequestURL(tests []generateURLTest, t *testing.T) {
	for i, test := range tests {
		url := defaultReMux.generateCompleteRequestURL(test.header)
		if url != test.expectedFullURL {
			t.Fatalf("#%d: \n url: %s\n expected url: %s\n, but got: %s",
				i, test.url, test.expectedFullURL, url)
		}
	}
}

func testGeneratePathEndingRequestURL(tests []generateURLTest, t *testing.T) {
	for i, test := range tests {
		url := defaultReMux.generatePathEndingRequestURL(test.header)
		if url != test.expectedPathEndingURL {
			t.Fatalf("#%d: \n url: %s\n expected url: %s\n, but got: %s",
				i, test.url, test.expectedPathEndingURL, url)
		}
	}
}

func TestFastGenerateCompleteRequestURL(t *testing.T) {
	for i := range generateURLTests {
		generateURLTests[i].header = newFastRequestHeader(generateURLTests[i].url, t)
	}
	testGenerateCompleteRequestURL(generateURLTests, t)
}

func TestFastGeneratePathEndingRequestURL(t *testing.T) {
	for i := range generateURLTests {
		generateURLTests[i].header = newFastRequestHeader(generateURLTests[i].url, t)
	}
	testGeneratePathEndingRequestURL(generateURLTests, t)
}

func TestNetGenerateCompleteRequestURL(t *testing.T) {
	for i := range generateURLTests {
		generateURLTests[i].header = newNetRequestHeader(generateURLTests[i].url)
	}
	testGenerateCompleteRequestURL(generateURLTests, t)
}

func TestNetGeneratePathEndingRequestURL(t *testing.T) {
	for i := range generateURLTests {
		generateURLTests[i].header = newNetRequestHeader(generateURLTests[i].url)
	}
	testGeneratePathEndingRequestURL(generateURLTests, t)
}
