/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/intstr"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"

	yamljsontool "github.com/ghodss/yaml"
)

const (
	eurekaRegistryType      string = "eureka"
	defaultKubeDir                 = ".kube"
	defaultKubernetesConfig        = "config"
	writerClusterRole              = "writer"
	readerClusterRole              = "reader"
)

var (
	defaultKubernetesConfigDir  = path.Join(homedir.HomeDir(), defaultKubeDir)
	defaultKubernetesConfigPath = path.Join(defaultKubernetesConfigDir, defaultKubernetesConfig)
)

const (
	defaultEgControlPlaneFilePath = "./manifests/easegress/control-plane"
	defaultEgIngressFilePath      = "./manifests/easegress/ingress-controller"
	defaultOperatorPath           = "./manifests/mesh-operator-config/default"
	// Easegress deploy default params
	defaultMeshNameSpace = "easemesh"

	defaultEgClusterName = "easegress-cluster"

	defaultEgClientPortName = "client-port"
	defaultEgPeerPortName   = "peer-port"
	defaultEgAdminPortName  = "admin-port"
	defaultEgClientPort     = 2379
	defaultEgPeerPort       = 2380
	defaultEgAdminPort      = 2381

	defaultEgServiceName      = "easegress-public"
	defaultEgServicePeerPort  = 2380
	defaultEgServiceAdminPort = 2381

	defaultEgHeadlessServiceName = "easegress-hs"

	// EaseMesh Controller default Params
	defaultMeshRegistryType   = eurekaRegistryType
	defaultHeartbeatInterval  = 5
	defaultMeshApiPort        = 13009
	defaultMeshControllerName = "easemesh-controller"
	meshControllerKind        = "MeshController"

	// easemesh-operator default params

)

type installArgs struct {
	meshNameSpace string

	egClusterName string
	egClientPort  int
	egAdminPort   int
	egPeerPort    int

	egServiceName      string
	egServicePeerPort  int
	egServiceAdminPort int

	// EaseMesh Controller default Params
	eashRegistryType  string
	heartbeatInterval int
	meshApiPort       int

	specFile string
}

func installCmd() *cobra.Command {
	iArgs := &installArgs{}
	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Deploy EaseMesh Components",
		Long:    "",
		Example: "egctl mesh install <args>",
		Run: func(cmd *cobra.Command, args []string) {
			if iArgs.specFile != "" {
				var buff []byte
				var err error
				buff, err = ioutil.ReadFile(iArgs.specFile)
				if err != nil {
					ExitWithErrorf("%s failed: %v", cmd.Short, err)
				}

				err = yaml.Unmarshal(buff, &iArgs)
				if err != nil {
					ExitWithErrorf("%s failed: %v", cmd.Short, err)
				}
			}
			install(cmd, iArgs)
		},
	}

	addInstallArgs(cmd, iArgs)
	return cmd
}

func addInstallArgs(cmd *cobra.Command, args *installArgs) {

	cmd.Flags().StringVar(&args.meshNameSpace, "mesh-namespace", defaultMeshNameSpace, "")

	cmd.Flags().StringVar(&args.egClusterName, "eg-cluster-name", defaultEgClusterName, "")
	cmd.Flags().IntVar(&args.egClientPort, "eg-client-port", defaultEgClientPort, "")
	cmd.Flags().IntVar(&args.egAdminPort, "eg-admin-port", defaultEgAdminPort, "")
	cmd.Flags().IntVar(&args.egPeerPort, "eg-peer-port", defaultEgPeerPort, "")

	cmd.Flags().StringVar(&args.egServiceName, "eg-service-name", defaultEgServiceName, "")
	cmd.Flags().IntVar(&args.egServicePeerPort, "eg-service-peer-port", defaultEgServicePeerPort, "")
	cmd.Flags().IntVar(&args.egServiceAdminPort, "eg-service-admin-port", defaultEgServiceAdminPort, "")

	cmd.Flags().StringVar(&args.eashRegistryType, "registry-type", defaultMeshRegistryType, "")
	cmd.Flags().IntVar(&args.heartbeatInterval, "heartbeat-interval", defaultHeartbeatInterval, "")
	cmd.Flags().IntVar(&args.meshApiPort, "mesh-api-port", defaultMeshApiPort, "")

	cmd.Flags().StringVarP(&args.specFile, "file", "f", "", "A yaml file specifying the install params.")
}
func install(cmd *cobra.Command, args *installArgs) {
	kubeClient, err := newKubernetesClient()
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}

	err = deployEasegress(cmd, kubeClient, args)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	fmt.Println("Easegress deploy success.")

	err = startUpMeshController(cmd, kubeClient, args)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	fmt.Println("Easegress control plane deploy success.")

	err = deployEasegressIngress(cmd, kubeClient, args)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	fmt.Println("Easegress Ingress  deploy success.")
	err = deployEaseMeshOperator(cmd, kubeClient, args)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	fmt.Println("EaseMesh Operator deploy success.")
	fmt.Println("Done.")
}

func deployEasegress(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	var err error
	err = completeEasegressNameSpace(cmd, kubeClient, args)
	err = completeEasegressConfig(cmd, kubeClient, args)
	err = completeEasegressServices(cmd, kubeClient, args)
	if err != nil {
		return err
	}

	var configPath = manifestPath(defaultEgControlPlaneFilePath)
	applyCmd := "cd " + configPath + " && kustomize edit set namespace " + args.meshNameSpace + " && kustomize build | kubectl apply -f -"
	command := exec.Command("bash", "-c", applyCmd)
	err = command.Run()
	return err
}

func completeEasegressNameSpace(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   args.meshNameSpace,
		Labels: map[string]string{},
	}}

	err := createNameSpace(ns, kubeClient)
	return err
}

func completeEasegressConfig(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {
	host := "0.0.0.0"

	cfg := EasegressConfig{
		args.egClusterName,
		args.egClusterName,
		writerClusterRole,
		"http://" + host + ":" + strconv.Itoa(args.egClientPort),
		"http://" + host + ":" + strconv.Itoa(args.egPeerPort),
		"",
		host + ":" + strconv.Itoa(args.egAdminPort),
		"/opt/eg-data/data",
		"",
		"",
		"",
		"/opt/eg-data/log",
		"/opt/eg-data/member",
		"INFO",
	}

	data := map[string]string{}
	out, err := yaml.Marshal(cfg)
	data["eg-master.yaml"] = string(out)
	buff, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal %#v to yaml failed: %v", cfg, err)
	}
	jsonBytes, err := yamljsontool.YAMLToJSON(buff)
	if err != nil {
		return fmt.Errorf("convert yaml %s to json failed: %v", buff, err)
	}

	var params map[string]string
	err = json.Unmarshal(jsonBytes, &params)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "easegress-cluster-cm",
			Namespace: args.meshNameSpace,
		},
		Data: params,
	}

	err = createConfigMap(configMap, kubeClient, args.meshNameSpace)
	if err != nil {
		return fmt.Errorf("create configMap failed: %v", err)
	}
	return err
}

func completeEasegressServices(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	selector := map[string]string{}
	selector["app"] = "easegress"

	service := easegressService(args)
	service.Spec.Selector = selector
	err := createService(service, kubeClient, args.meshNameSpace)

	headlessService := easegressHeadlessService(args)
	headlessService.Spec.Selector = selector
	err = createService(headlessService, kubeClient, args.meshNameSpace)
	return err
}

func easegressService(args *installArgs) *v1.Service {
	selector := map[string]string{}
	selector["app"] = "easegress"

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.egServiceName,
			Namespace: args.meshNameSpace,
		},
	}
	service.Spec.Ports = []v1.ServicePort{
		v1.ServicePort{
			Name:       defaultEgAdminPortName,
			Port:       int32(args.egAdminPort),
			TargetPort: intstr.IntOrString{IntVal: 2381},
		},
		v1.ServicePort{
			Name:       defaultEgPeerPortName,
			Port:       int32(args.egPeerPort),
			TargetPort: intstr.IntOrString{IntVal: 2380},
		},
		v1.ServicePort{
			Name:       defaultEgClientPortName,
			Port:       int32(args.egClientPort),
			TargetPort: intstr.IntOrString{IntVal: 2379},
		},
	}
	service.Spec.Selector = selector
	return service
}

func easegressHeadlessService(args *installArgs) *v1.Service {
	headlessService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultEgHeadlessServiceName,
			Namespace: args.meshNameSpace,
		},
	}

	headlessService.Spec.ClusterIP = "None"
	headlessService.Spec.Ports = []v1.ServicePort{
		v1.ServicePort{
			Name:       defaultEgAdminPortName,
			Port:       int32(args.egAdminPort),
			TargetPort: intstr.IntOrString{IntVal: 2381},
		},
		v1.ServicePort{
			Name:       defaultEgPeerPortName,
			Port:       int32(args.egPeerPort),
			TargetPort: intstr.IntOrString{IntVal: 2380},
		},
		v1.ServicePort{
			Name:       defaultEgClientPortName,
			Port:       int32(args.egClientPort),
			TargetPort: intstr.IntOrString{IntVal: 2379},
		},
	}

	return headlessService
}

func manifestPath(filePath string) string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	return path.Join(filepath.Dir(exPath), filePath)
}

func easegressDeploySuccess(httpMethod string, url string, reqBody []byte, cmd *cobra.Command) bool {
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(reqBody))
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	return successfulStatusCode(resp.StatusCode)
}

func startUpMeshController(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	service, err := kubeClient.CoreV1().Services(args.meshNameSpace).Get(context.TODO(), args.egServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("StartUp MeshController failed: %v", err)
	}

	c := make(chan bool)

	probeUrl := "http://" + service.Spec.ClusterIP + ":" + strconv.Itoa(args.egServiceAdminPort) + apiURL
	go func() {
		for !easegressDeploySuccess(http.MethodGet, probeUrl, nil, cmd) {
			time.Sleep(3 * time.Second)
		}
		c <- true
		close(c)

	}()

	meshControllerConfig := MeshControllerConfig{
		defaultMeshControllerName,
		meshControllerKind,
		args.eashRegistryType,
		strconv.Itoa(args.heartbeatInterval) + "s",
	}

	<-c
	configBody, err := yaml.Marshal(meshControllerConfig)
	url := "http://" + service.Spec.ClusterIP + ":" + strconv.Itoa(args.egServiceAdminPort) + objectsURL
	handleRequest(http.MethodPost, url, configBody, cmd)
	return nil
}

func deployEasegressIngress(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	err := completeEasegressIngressConfig(cmd, kubeClient, args)
	if err != nil {
		return err
	}

	var configPath = manifestPath(defaultEgIngressFilePath)
	applyCmd := "cd " + configPath + " && kustomize edit set namespace  " + args.meshNameSpace + " && kustomize build | kubectl apply -f -"
	command := exec.Command("bash", "-c", applyCmd)
	err = command.Run()
	return err

}

func completeEasegressIngressConfig(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {
	params := &EasegressReaderParams{}
	params.ClusterRole = readerClusterRole
	params.ClusterRequestTimeout = "10s"
	params.ClusterJoinUrls = "http://" + defaultEgHeadlessServiceName + "." + args.meshNameSpace + ":" + strconv.Itoa(args.egPeerPort)
	params.ClusterName = args.egClusterName
	params.Name = "mesh-mesh-ingress"

	labels := make(map[string]string)
	labels["mesh-role"] = "ingress-controller"
	params.Labels = labels

	data := map[string]string{}
	ingressControllerConfig, err := yaml.Marshal(params)
	data["eg-ingress.yaml"] = string(ingressControllerConfig)
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "easegress-ingress-config-cm",
			Namespace: args.meshNameSpace,
		},
		Data: data,
	}

	err = createConfigMap(configMap, kubeClient, args.meshNameSpace)
	if err != nil {
		return fmt.Errorf("create configMap failed: %v", err)
	}
	return err
}

func deployEaseMeshOperator(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	var configPath = manifestPath(defaultOperatorPath)
	applyCmd := "cd " + configPath + " && kustomize edit set namespace  " + args.meshNameSpace + " && kustomize build | kubectl apply -f -"
	command := exec.Command("bash", "-c", applyCmd)
	err := command.Run()
	if err != nil {
		return err
	}
	err = completeEaseMeshOperatorConfig(cmd, kubeClient, args)
	if err != nil {
		return err
	}
	return nil
}

func completeEaseMeshOperatorConfig(cmd *cobra.Command, kubeClient *kubernetes.Clientset, args *installArgs) error {

	cfg := MeshOperatorConfig{
		args.egClusterName,
		"http://" + defaultEgHeadlessServiceName + "." + args.meshNameSpace + ":" + strconv.Itoa(args.egPeerPort),
		"127.0.0.1:8080",
		false,
		":8081",
	}

	data := map[string]string{}
	operatorConfig, err := yaml.Marshal(cfg)
	data["operator-config.yaml"] = string(operatorConfig)
	buff, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal %#v to yaml failed: %v", cfg, err)
	}
	jsonBytes, err := yamljsontool.YAMLToJSON(buff)
	if err != nil {
		return fmt.Errorf("convert yaml %s to json failed: %v", buff, err)
	}

	var params map[string]string
	err = json.Unmarshal(jsonBytes, &params)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "easemesh-operator-config",
			Namespace: args.meshNameSpace,
		},
		Data: params,
	}

	err = createConfigMap(configMap, kubeClient, args.meshNameSpace)
	if err != nil {
		return fmt.Errorf("create configMap failed: %v", err)
	}
	return err
}
