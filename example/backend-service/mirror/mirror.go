package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TeeWriter is an io.Writer wapper.
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
