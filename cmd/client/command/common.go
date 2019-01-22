package command

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
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

	pipelinesURL = apiURL + "/pipelines"
	pipelineURL  = apiURL + "/pipelines/%s"

	statsURL = apiURL + "/stats"
)

func makeURL(urlTemplate string, a ...interface{}) string {
	return "http://" + CommandlineGlobalFlags.Server + fmt.Sprintf(urlTemplate, a...)
}

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}

func handleRequest(httpMethod string, url string, reqBody []byte, cmd *cobra.Command) {
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(reqBody))
	if err != nil {
		ExitWithError(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}

	prettyJSON := printPrettyJson(body)
	fmt.Println(prettyJSON)

	if !successfulStatusCode(resp.StatusCode) {
		ExitWithErrorf("%s failed, http status code: %d", cmd.Short, resp.StatusCode)
	}

}

func printPrettyJson(body []byte) string {
	var prettyJSON []byte
	var jsonObj interface{} = nil
	err := json.Unmarshal(body, &jsonObj)
	if err != nil {
		ExitWithErrorf("Marshal failed: %v", err)
	}
	prettyJSON, err = json.MarshalIndent(jsonObj, "", "\t")
	if err != nil {
		ExitWithErrorf("Marchal indent failed: %v", err)
	}
	return string(prettyJSON)
}

type (
	PluginSpec struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}

	PipelineSpec struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
)

func readFromFileOrStdin(specFile string, cmd *cobra.Command) ([]byte, string) {
	var jsonText []byte
	var err error
	if specFile != "" {
		jsonText, err = ioutil.ReadFile(specFile)
		if err != nil {
			ExitWithErrorf("%s failed: %v", cmd.Short, err)
		}
	} else {
		reader := bufio.NewReader(os.Stdin)
		jsonText, err = ioutil.ReadAll(reader)
		if err != nil {
			ExitWithErrorf("%s failed: %v", cmd.Short, err)
		}
	}

	if strings.Contains(cmd.CommandPath(), "plugin") {
		spec := new(PipelineSpec)
		err := json.Unmarshal(jsonText, &spec)
		if err != nil {
			ExitWithErrorf("%s failed, invalid spec: %v", cmd.Short, err)
		}

		return jsonText, spec.Name
	}

	if strings.Contains(cmd.CommandPath(), "pipeline") {
		spec := new(PipelineSpec)
		err := json.Unmarshal(jsonText, &spec)
		if err != nil {
			ExitWithErrorf("%s failed, invalid spec: %v", cmd.Short, err)
		}

		return jsonText, spec.Name
	}

	// should never come here
	ExitWithErrorf("Only 'plugin' and 'pipeline' cmd supported, but got: %s", cmd.Use)
	return nil, ""
}
