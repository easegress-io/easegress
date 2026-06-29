/*
 * Copyright (c) 2017, The Easegress Authors
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

package easegress

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readChartValues(t *testing.T) map[string]interface{} {
	t.Helper()

	data, err := os.ReadFile("values.yaml")
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}

	values := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse values.yaml: %v", err)
	}
	return values
}

func chartString(t *testing.T, values map[string]interface{}, keys ...string) string {
	t.Helper()

	value := chartValue(t, values, keys...)
	got, ok := value.(string)
	if !ok {
		t.Fatalf("%s must be a string, got %T", strings.Join(keys, "."), value)
	}
	return got
}

func chartBool(t *testing.T, values map[string]interface{}, keys ...string) bool {
	t.Helper()

	value := chartValue(t, values, keys...)
	got, ok := value.(bool)
	if !ok {
		t.Fatalf("%s must be a bool, got %T", strings.Join(keys, "."), value)
	}
	return got
}

func chartValue(t *testing.T, values map[string]interface{}, keys ...string) interface{} {
	t.Helper()

	var value interface{} = values
	for _, key := range keys {
		current, ok := value.(map[string]interface{})
		if !ok {
			t.Fatalf("%s parent must be a map, got %T", strings.Join(keys, "."), value)
		}
		value, ok = current[key]
		if !ok {
			t.Fatalf("missing chart value %s", strings.Join(keys, "."))
		}
	}
	return value
}

func readChartFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestDefaultChartDoesNotExposeAdminNodePort(t *testing.T) {
	values := readChartValues(t)

	if got := chartString(t, values, "admin", "apiAddr"); got != "127.0.0.1:2381" {
		t.Fatalf("default admin.apiAddr = %q, want local-only 127.0.0.1:2381", got)
	}
	if chartBool(t, values, "service", "admin", "enabled") {
		t.Fatal("default service.admin.enabled must be false")
	}
	if got := chartString(t, values, "service", "admin", "type"); got == "NodePort" || got == "LoadBalancer" {
		t.Fatalf("default service.admin.type = %q, want non-external service type", got)
	}

	serviceTemplate := readChartFile(t, "templates/Service.yaml")
	for _, required := range []string{
		"{{- if .Values.service.admin.enabled }}",
		"{{- if and .Values.service.admin.enabled $adminAPILoopback -}}",
		"admin service requires admin.apiAddr to listen on a non-loopback address",
		"admin service type NodePort/LoadBalancer requires admin.basicAuth or admin.allowUnsafeNoAuth=true",
		"nodePort: {{ default .Values.service.adminPort .Values.service.admin.nodePort }}",
	} {
		if !strings.Contains(serviceTemplate, required) {
			t.Fatalf("templates/Service.yaml missing admin-service guard %q", required)
		}
	}
	if strings.Contains(serviceTemplate, "nodePort: {{ .Values.service.adminPort }}") {
		t.Fatal("templates/Service.yaml must not unconditionally render the admin NodePort")
	}
}

func TestDefaultChartDoesNotExposeKubernetesSecrets(t *testing.T) {
	values := readChartValues(t)

	if chartBool(t, values, "rbac", "readSecrets") {
		t.Fatal("default rbac.readSecrets must be false")
	}
	if chartBool(t, values, "serviceAccount", "automountServiceAccountToken") {
		t.Fatal("default serviceAccount.automountServiceAccountToken must be false")
	}

	clusterRole := readChartFile(t, "templates/ClusterRole.yaml")
	if strings.Contains(clusterRole, `resources: ["services", "endpoints", "secrets"]`) {
		t.Fatal("ClusterRole must not always grant secrets access")
	}
	if !strings.Contains(clusterRole, "{{- if .Values.rbac.readSecrets }}") {
		t.Fatal("ClusterRole must gate secrets access on rbac.readSecrets")
	}

	for _, path := range []string{
		"templates/StatefulSet.yaml",
		"templates/Deployment.yaml",
	} {
		workload := readChartFile(t, path)
		if !strings.Contains(workload, "automountServiceAccountToken: {{ .Values.serviceAccount.automountServiceAccountToken }}") {
			t.Fatalf("%s must render serviceAccount.automountServiceAccountToken", path)
		}
	}
}
