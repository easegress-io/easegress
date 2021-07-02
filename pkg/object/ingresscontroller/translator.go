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

package ingresscontroller

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/supervisor"
	apicorev1 "k8s.io/api/core/v1"
	apinetv1 "k8s.io/api/networking/v1"
)

const (
	defaultPipelineName = "pipeline-default"
)

// specTranslator translates k8s ingress related specs to Easegress http server
// spec and pipeline specs
type specTranslator struct {
	k8sClient    *k8sClient
	httpSvr      *supervisor.Spec
	pipelines    map[string]*supervisor.Spec
	httpSvrCfg   *httpserver.Spec
	ingressClass string
}

func newSpecTranslator(k8sClient *k8sClient, ingressClass string, httpSvrCfg *httpserver.Spec) *specTranslator {
	return &specTranslator{
		k8sClient:    k8sClient,
		httpSvrCfg:   httpSvrCfg,
		ingressClass: ingressClass,
	}
}

func (st *specTranslator) httpServerSpec() *supervisor.Spec {
	return st.httpSvr
}

func (st *specTranslator) pipelineSpecs() map[string]*supervisor.Spec {
	return st.pipelines
}

func generatePipelineSpec(name string, endpoints []string) (*supervisor.Spec, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "name: %s\n", name)
	buf.WriteString(`kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      loadBalance:
        policy: roundRobin
      servers:
`)
	for _, ep := range endpoints {
		fmt.Fprintf(&buf, "        - url: %s\n", ep)
	}

	logger.Debugf("pipeline spec generated:\n%s", buf.String())
	return supervisor.NewSpec(buf.String())
}

func (st *specTranslator) getEndpoints(namespace string, service *apinetv1.IngressServiceBackend) ([]string, error) {
	svc, err := st.k8sClient.getService(namespace, service.Name)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, fmt.Errorf("service %s/%s does not exist", namespace, service.Name)
	}

	var svcPort *apicorev1.ServicePort
	for _, p := range svc.Spec.Ports {
		if p.Port == service.Port.Number || (len(p.Name) > 0 && p.Name == service.Port.Name) {
			svcPort = &p
			break
		}
	}
	if svcPort == nil {
		return nil, fmt.Errorf("service port in service %s/%s", namespace, service.Name)
	}
	protocal := "http"
	if svcPort.Port == 443 {
		protocal = "https"
	}

	var result []string
	if svc.Spec.Type == apicorev1.ServiceTypeExternalName {
		hostPort := net.JoinHostPort(svc.Spec.ExternalName, strconv.Itoa(int(svcPort.Port)))
		ep := fmt.Sprintf("%s://%s", protocal, hostPort)
		result = append(result, ep)
		return result, nil
	}

	endpoints, err := st.k8sClient.getEndpoints(namespace, service.Name)
	if err != nil {
		return nil, err
	}

	for _, subset := range endpoints.Subsets {
		var port int32
		for _, p := range subset.Ports {
			if svcPort.Name == p.Name {
				port = p.Port
			}
		}
		if port == 0 {
			continue
		}

		for _, addr := range subset.Addresses {
			hostPort := net.JoinHostPort(addr.IP, strconv.Itoa(int(port)))
			ep := fmt.Sprintf("%s://%s", protocal, hostPort)
			result = append(result, ep)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("service %s/%s endpoint not found", namespace, service.Name)
	}
	return result, nil
}

func (st *specTranslator) serviceToPipeline(namespace string, service *apinetv1.IngressServiceBackend) (*supervisor.Spec, error) {
	if service == nil || len(service.Name) == 0 {
		err := fmt.Errorf("invalid service name, ingress backend is object ref")
		logger.Errorf("%v", err)
		return nil, err
	}

	port := service.Port.Name
	if len(port) == 0 {
		port = strconv.Itoa(int(service.Port.Number))
	}
	pipelineName := fmt.Sprintf("pipeline-%s-%s-%s", namespace, service.Name, port)
	if st.pipelines[pipelineName] != nil {
		return st.pipelines[pipelineName], nil
	}

	endpoints, err := st.getEndpoints(namespace, service)
	if err != nil {
		logger.Errorf("failed to get service endpoints: %v", err)
		return nil, err
	}

	spec, err := generatePipelineSpec(pipelineName, endpoints)
	if err != nil {
		logger.Errorf("failed to generate pipeline spec: %v", err)
		return nil, err
	}

	st.pipelines[pipelineName] = spec
	return spec, err
}

func (st *specTranslator) translateDefaultPipeline(ingress *apinetv1.Ingress) error {
	if st.pipelines[defaultPipelineName] != nil {
		err := fmt.Errorf("the default pipeline has already been created")
		logger.Errorf("%v", err)
		return err
	}

	endpoints, err := st.getEndpoints(ingress.Namespace, ingress.Spec.DefaultBackend.Service)
	if err != nil {
		logger.Errorf("failed to get service endpoints: %v", err)
		return err
	}

	spec, err := generatePipelineSpec(defaultPipelineName, endpoints)
	if err != nil {
		logger.Errorf("failed to generate pipeline spec: %v", err)
		return err
	}

	st.pipelines[defaultPipelineName] = spec
	return nil
}

func getCertificateBlocks(secret *apicorev1.Secret, namespace, secretName string) (string, string, error) {
	crt, ok := secret.Data["tls.crt"]
	if !ok || len(crt) == 0 {
		err := fmt.Errorf("'tls.crt' is missing or empty in secret %s/%s", namespace, secretName)
		return "", "", err
	}

	key, ok := secret.Data["tls.key"]
	if !ok || len(key) == 0 {
		err := fmt.Errorf("'tls.crt' is missing or empty in secret %s/%s", namespace, secretName)
		return "", "", err
	}

	return string(crt), string(key), nil
}

func (st *specTranslator) translateTLSConfig(buf *bytes.Buffer, ingresses []*apinetv1.Ingress) error {
	certs := map[string]string{}
	keys := map[string]string{}

	for _, ingress := range ingresses {
		for _, t := range ingress.Spec.TLS {
			if len(t.SecretName) == 0 {
				continue
			}
			cfgKey := ingress.Namespace + "/" + t.SecretName
			if _, ok := certs[cfgKey]; ok {
				continue
			}

			secret, err := st.k8sClient.getSecret(ingress.Namespace, t.SecretName)
			if err != nil {
				logger.Errorf("failed to get secret %s%s: %v", ingress.Namespace, t.SecretName, err)
				continue
			}
			if secret == nil {
				logger.Errorf("secret %s/%s does not exist", ingress.Namespace, t.SecretName)
				continue
			}

			cert, key, err := getCertificateBlocks(secret, ingress.Namespace, t.SecretName)
			if err != nil {
				continue
			}

			certs[cfgKey] = cert
			keys[cfgKey] = key
		}
	}

	buf.WriteString("certs:\n")
	for k, v := range certs {
		fmt.Fprintf(buf, "  %s: %s\n", k, v)
	}

	buf.WriteString("keys:\n")
	for k, v := range keys {
		fmt.Fprintf(buf, "  %s: %s\n", k, v)
	}

	return nil
}

func (st *specTranslator) translateIngressRules(buf *bytes.Buffer, ingress *apinetv1.Ingress) {
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		var numPath int
		for _, path := range rule.HTTP.Paths {
			pipeline, err := st.serviceToPipeline(ingress.Namespace, path.Backend.Service)
			if err != nil {
				continue
			}

			if numPath == 0 {
				buf.WriteString("  - paths:\n")
			}
			fmt.Fprintf(buf, "    - backend: %s\n", pipeline.Name())
			if path.PathType != nil && *path.PathType == apinetv1.PathTypeExact {
				fmt.Fprintf(buf, "      path: %s\n", path.Path)
			} else {
				fmt.Fprintf(buf, "      pathPrefix: %s\n", path.Path)
			}
			numPath++
		}

		if numPath == 0 || len(rule.Host) == 0 {
			continue
		}

		if rule.Host[0] == '*' {
			host := strings.ReplaceAll(rule.Host[1:], ".", "\\\\.")
			fmt.Fprintf(buf, "    hostRegexp: \"^[^.]+%s$\"\n", host)
		} else {
			fmt.Fprintf(buf, "    host: %s\n", rule.Host)
		}
	}
}

func (st *specTranslator) beginTranslate(buf *bytes.Buffer) {
	st.httpSvr = nil
	st.pipelines = make(map[string]*supervisor.Spec)

	buf.WriteString("name: http-server-ingress-controller\n")
	buf.WriteString("kind: HTTPServer\n")
	fmt.Fprintf(buf, "port: %d\n", st.httpSvrCfg.Port)
	fmt.Fprintf(buf, "keepAlive: %t\n", st.httpSvrCfg.KeepAlive)
	if len(st.httpSvrCfg.KeepAliveTimeout) > 0 {
		fmt.Fprintf(buf, "keepAliveTimeout: %s\n", st.httpSvrCfg.KeepAliveTimeout)
	}
	fmt.Fprintf(buf, "https: %t\n", st.httpSvrCfg.HTTPS)
	if st.httpSvrCfg.MaxConnections > 0 {
		fmt.Fprintf(buf, "maxConnections: %d\n", st.httpSvrCfg.MaxConnections)
	}
	buf.WriteString("rules:\n")
}

func (st *specTranslator) endTranslate(buf *bytes.Buffer) error {
	logger.Debugf("http server spec generated:\n%s", buf.String())
	spec, err := supervisor.NewSpec(buf.String())
	if err != nil {
		return err
	}
	st.httpSvr = spec
	return nil
}

func (st *specTranslator) translate() error {
	buf := &bytes.Buffer{}

	st.beginTranslate(buf)

	ingresses := st.k8sClient.getIngresses(st.ingressClass)

	if st.httpSvrCfg.HTTPS {
		st.translateTLSConfig(buf, ingresses)
	}

	for _, ingress := range ingresses {
		if ingress.Spec.DefaultBackend != nil {
			st.translateDefaultPipeline(ingress)
		}
		st.translateIngressRules(buf, ingress)
	}

	if p := st.pipelines[defaultPipelineName]; p != nil {
		buf.WriteString("  - paths:\n")
		fmt.Fprintf(buf, "    - backend: %s\n", defaultPipelineName)
		buf.WriteString("      pathPrefix: /\n")
	}

	return st.endTranslate(buf)
}
