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

// Package main is the entry point of the simple mirror server.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TeeWriter is an io.Writer wrapper.
type TeeWriter struct {
	writers []io.Writer
}

// NewTeeWriter returns a TeeWriter.
func NewTeeWriter(writers ...io.Writer) *TeeWriter {
	return &TeeWriter{writers: writers}
}

// Write writes the data.
func (tw *TeeWriter) Write(p []byte) (n int, err error) {
	for _, w := range tw.writers {
		w.Write(p)
	}
	return len(p), nil
}

func main() {
	helloHandler := func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "hello")
	}
	mirrorHandler := func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(10 * time.Millisecond)
		body, err := io.ReadAll(req.Body)
		if err != nil {
			body = []byte(fmt.Sprintf("<read failed: %v>", err))
		}

		tw := NewTeeWriter(w, os.Stdout)

		url := req.URL.Path
		if req.URL.Query().Encode() != "" {
			url += "?" + req.URL.Query().Encode()
		}

		fmt.Fprintln(tw, "Your Request")
		fmt.Fprintln(tw, "==============")
		fmt.Fprintln(tw, "Method:", req.Method)
		fmt.Fprintln(tw, "URL   :", url)

		fmt.Fprintln(tw, "Header:")
		for k, v := range req.Header {
			fmt.Fprintf(tw, "    %s: %v\n", k, v)
		}

		fmt.Fprintln(tw, "Body  :", string(body))
	}

	http.HandleFunc("/", mirrorHandler)
	http.HandleFunc("/pipeline/activity/1", helloHandler)
	http.HandleFunc("/pipeline/activity/2", helloHandler)

	for _, port := range []int{9091, 9092, 9093, 9094, 9095, 9096, 9097} {
		go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}

	http.ListenAndServe(":9098", nil)
}
