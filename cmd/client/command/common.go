package command

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

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
	apiURL = "/apis/v3"

	membersURL = apiURL + "/members"
	memberURL  = apiURL + "/members/%s"

	objectKindsURL  = apiURL + "/object-kinds"
	objectsURL      = apiURL + "/objects"
	objectURL       = apiURL + "/objects/%s"
	objectStatusURL = apiURL + "/objects/%s/status"
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
	var output []byte
	switch CommandlineGlobalFlags.OutputFormat {
	case "yaml":
		output = body
	case "json":
		var err error
		output, err = yaml.YAMLToJSON(body)
		if err != nil {
			ExitWithErrorf("yaml %s to json failed: %v", body, err)
		}
	}

	fmt.Printf("%s", output)
}

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

	var spec struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(buff, &spec)
	if err != nil {
		ExitWithErrorf("%s failed, invalid spec: %v", cmd.Short, err)
	}

	return buff, spec.Name
}
