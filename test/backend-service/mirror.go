package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {
	mirrorHandler := func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(10 * time.Millisecond)
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			body = []byte(fmt.Sprintf("<read failed: %v>", err))
		}

		url := req.URL.Path
		if req.URL.Query().Encode() != "" {
			url += "?" + req.URL.Query().Encode()
		}

		content := fmt.Sprintf(`Your Request
===============
Method: %s
URL   : %s
Header: %v
Body  : %s`, req.Method, url, req.Header, body)

		// fmt.Printf("%s: %d bytes body received, %d bytes body sent\n",
		// 	req.Host, len(body), len(content))

		io.WriteString(w, content)
	}

	http.HandleFunc("/", mirrorHandler)

	for _, port := range []int{9091, 9092, 9093, 9094, 9095, 9096, 9097} {
		go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}

	http.ListenAndServe(":9098", nil)
}
