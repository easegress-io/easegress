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

package jmxtool

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

func httpServer(finished chan bool) {
	m := http.NewServeMux()
	s := http.Server{Addr: ":8181", Handler: m}
	m.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Goobey, %q", html.EscapeString(r.URL.Path))
		s.Shutdown(context.Background())
	})
	m.HandleFunc(serviceConfigURL, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
	}
	fmt.Println("Finished")
	finished <- true
}
func TestAgentClient(t *testing.T) {

	finished := make(chan bool)
	go httpServer(finished)

	agent := NewAgentClient("127.0.0.1", "8181")
	fmt.Printf("%v\n", agent)

	service := &spec.Service{
		Name: "agent",
		LoadBalance: &spec.LoadBalance{
			Policy: proxy.PolicyRandom,
		},
		Sidecar: &spec.Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	agent.UpdateService(service, 1)
	header := &spec.GlobalCanaryHeaders{
		ServiceHeaders: map[string][]string{},
	}
	agent.UpdateCanary(header, 1)

	var client = &http.Client{
		Timeout: time.Second,
	}
	client.Get("http://127.0.0.1:8181/shutdown")

	<-finished
}
