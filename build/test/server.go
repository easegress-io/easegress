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

// Package test is for integration testing.
package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func startServer(port int, handler http.Handler) *http.Server {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: handler,
	}
	go server.ListenAndServe()
	return server
}

func checkServerStart(t *testing.T, checkReq func() *http.Request) bool {
	for i := 0; i < 10; i++ {
		req := checkReq()
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
