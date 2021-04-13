package command

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"

	yamljsontool "github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

type (
	// GlobalFlags is the global flags for the whole client.
	GlobalFlags struct {
		Server       string
		OutputFormat string
	}

	// APIErr is the standard return of error.
	APIErr struct {
		Code    int    `yaml:"code"`
		Message string `yaml:"message"`
	}
)

var (
	// CommandlineGlobalFlags is the singleton of GlobalFlags.
	CommandlineGlobalFlags GlobalFlags
)

const (
	apiURL = "/apis/v3"

	healthURL = apiURL + "/healthz"

	membersURL = apiURL + "/status/members"
	memberURL  = apiURL + "/status/members/%s"

	objectKindsURL = apiURL + "/object-kinds"
	objectsURL     = apiURL + "/objects"
	objectURL      = apiURL + "/objects/%s"

	statusObjectURL  = apiURL + "/status/objects/%s"
	statusObjectsURL = apiURL + "/status/objects"

	// MeshTenantPrefix is the mesh tenant prefix.
	MeshTenantPrefix = apiURL + "/mesh/tenants"

	// MeshTenantPath is the mesh tenant path.
	MeshTenantPath = apiURL + "/mesh/tenants/%s"

	// MeshServicePrefix is mesh service prefix.
	MeshServicePrefix = apiURL + "/mesh/services"

	// MeshServicePath is the mesh service path.
	MeshServicePath = apiURL + "/mesh/services/%s"

	// MeshServiceCanaryPath is the mesh service canary path.
	MeshServiceCanaryPath = apiURL + "/mesh/services/%s/canary"

	// MeshServiceResiliencePath is the mesh service resilience path.
	MeshServiceResiliencePath = apiURL + "/mesh/services/%s/resilience"

	// MeshServiceLoadBalancePath is the mesh service load balance path.
	MeshServiceLoadBalancePath = apiURL + "/mesh/services/%s/loadbalance"

	// MeshServiceOutputServerPath is the mesh service output server path.
	MeshServiceOutputServerPath = apiURL + "/mesh/services/%s/outputserver"

	// MeshServiceTracingsPath is the mesh service tracings path.
	MeshServiceTracingsPath = apiURL + "/mesh/services/%s/tracings"

	// MeshServiceMetricsPath is the mesh service metrics path.
	MeshServiceMetricsPath = apiURL + "/mesh/services/%s/metrics"

	// MeshServiceInstancePrefix is the mesh service prefix.
	MeshServiceInstancePrefix = apiURL + "/mesh/serviceinstances"

	// MeshServiceInstancePath is the mesh service path.
	MeshServiceInstancePath = apiURL + "/mesh/serviceinstances/%s/%s"
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

	if !successfulStatusCode(resp.StatusCode) {
		msg := string(body)
		apiErr := &APIErr{}
		err = yaml.Unmarshal(body, apiErr)
		if err == nil {
			msg = apiErr.Message
		}
		ExitWithErrorf("%d: %s", apiErr.Code, msg)
	}

	if len(body) != 0 {
		printBody(body)
	}
}

func printBody(body []byte) {
	var output []byte
	switch CommandlineGlobalFlags.OutputFormat {
	case "yaml":
		output = body
	case "json":
		var err error
		output, err = yamljsontool.YAMLToJSON(body)
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

func newKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", defaultKubernetesConfigPath)
	if err != nil {
		return nil, err
	}

	// create the clientset
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		ExitWithErrorf("Can't connect to K8S.")
	}

	return kubeClient, nil
}
