package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type (
	contextEntity struct {
		Request  requestEntity  `json:"request"`
		Response responseEntity `json:"response"`
	}

	requestEntity struct {
		RealIP string `json:"realIP"`

		Method string `json:"method"`

		Scheme   string `json:"scheme"`
		Host     string `json:"host"`
		Path     string `json:"path"`
		Query    string `json:"query"`
		Fragment string `json:"fragment"`

		Proto string `json:"proto"`

		Header http.Header `json:"header"`

		Body []byte `json:"body"`
	}

	responseEntity struct {
		StatusCode int         `json:"statusCode"`
		Header     http.Header `json:"header"`
		Body       []byte      `json:"body"`
	}
)

func main() {
	largeBodySize := 64 * 1024
	largeBody := bytes.Repeat([]byte(`-`), largeBodySize)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctxEntity := &contextEntity{}
		err = json.Unmarshal(body, ctxEntity)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctxEntity.Response.StatusCode = 200
		ctxEntity.Response.Header.Add("X-Remote-Name", "G.O.O.D")
		ctxEntity.Response.Body = largeBody

		buff, err := json.Marshal(ctxEntity)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(buff)
	})

	http.ListenAndServe(":10000", nil)
}
