package command

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type GlobalFlags struct {
	Server string
}

var (
	CommandlineGlobalFlags GlobalFlags
)

const (
	apiURL = "/apis/v2"

	membersURL = apiURL + "/members"

	pluginsURL     = apiURL + "/plugins"
	pluginURL      = apiURL + "/plugins/%s"
	pluginTypesURL = apiURL + "/plugin-types"

	pipelinesURL = apiURL + "/plugins"
	pipelineURL  = apiURL + "/plugins/%s"

	statsURL = apiURL + "/stats"
)

func makeURL(urlTemplate string, a ...interface{}) string {
	return "http://" + CommandlineGlobalFlags.Server + fmt.Sprintf(urlTemplate, a...)
}

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}

func handleRequest(req *http.Request) (int, []byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request failed: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response body failed: %v", err)
	}
	err = resp.Body.Close()
	if err != nil {
		return 0, nil, fmt.Errorf("close response body failed: %v", err)
	}

	if !successfulStatusCode(resp.StatusCode) {
		return resp.StatusCode, body, fmt.Errorf("code %d, message: %s", resp.StatusCode, string(body))
	}
	return resp.StatusCode, body, nil
}
