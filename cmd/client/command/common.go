package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

type GlobalFlags struct {
	Server       string
	OutputFormat string
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

	if len(body) != 0 {
		printBody(body)
	}

	if !successfulStatusCode(resp.StatusCode) {
		ExitWithErrorf("%s failed, http status code: %d", cmd.Short, resp.StatusCode)
	}

}

func printBody(body []byte) {
	var obj interface{}
	err := json.Unmarshal(body, &obj)
	if err != nil {
		ExitWithErrorf("unmarshal json failed: %v", err)
	}
	var output []byte
	switch CommandlineGlobalFlags.OutputFormat {
	case "yaml":
		output, err = yaml.Marshal(obj)
		if err != nil {
			ExitWithErrorf("marchal yaml failed: %v", err)
		}
	case "json":
		output, err = json.MarshalIndent(obj, "", "\t")
		if err != nil {
			ExitWithErrorf("marchal json failed: %v", err)
		}
	}

	fmt.Printf("%s\n", output)
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
	var buff []byte
	var err error
	if specFile != "" {
		buff, err = ioutil.ReadFile(specFile)
		if err != nil {
			ExitWithErrorf("%s failed: %v", cmd.Short, err)
		}
	} else {
		buff, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			ExitWithErrorf("%s failed: %v", cmd.Short, err)
		}
	}

	buff, _ = yaml.YAMLToJSON(buff)

	if strings.Contains(cmd.CommandPath(), "plugin") {
		spec := new(PipelineSpec)
		err := json.Unmarshal(buff, &spec)
		if err != nil {
			ExitWithErrorf("%s failed, invalid spec: %v", cmd.Short, err)
		}

		return buff, spec.Name
	}

	if strings.Contains(cmd.CommandPath(), "pipeline") {
		spec := new(PipelineSpec)
		err := json.Unmarshal(buff, &spec)
		if err != nil {
			ExitWithErrorf("%s failed, invalid spec: %v", cmd.Short, err)
		}

		return buff, spec.Name
	}

	// should never come here
	ExitWithErrorf("Only 'plugin' and 'pipeline' cmd supported, but got: %s", cmd.Use)
	return nil, ""
}
