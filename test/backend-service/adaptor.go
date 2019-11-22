package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

var port = flag.Int("p", 0, "listen port")
var runtype = flag.String("t", "client", "runtype: [server|client]")
var host = flag.String("h", "localhost", "server host")

func main() {
	flag.Parse()
	if *runtype == "server" {
		runServer()
	}else{
		ret:=runClient()
		if !ret {
			os.Exit(1)
		}
	}
}
func runServer(){
	httpHander := func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(10 * time.Millisecond)
		v := req.Header.Get("forAddHeader")
		if v != "AddByGateway" {
			fmt.Println("Not found gateway add Header forAddHeader")
		}
		v = req.Header.Get("forSetHeader")
		if v != "SetByGateway" {
			fmt.Println("Not found gateway add Header forSetHeader or not set by gateway")
		}
		v = req.Header.Get("forDelHeader")
		if v != "" {
			fmt.Println("Not del header by gateway")
		}

		w.Header().Set("retDelHearder", "test")
		w.Header().Set("retSetHearder", "test")
		w.WriteHeader(http.StatusOK)
	}
	http.HandleFunc("/", httpHander)
	portstr := ":9098"
	if *port != 0 {
		portstr = fmt.Sprintf(":%d", *port)
	}
	http.ListenAndServe(portstr, nil)
}
func runClient() bool{
	client := &http.Client{}
	url := fmt.Sprintf("%s:%d", *host,*port)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		fmt.Printf("create request error: %v \n" , err )
	}
	req.Header.Set("forDelHearder", "test")
	req.Header.Set("forSetHearder", "test")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("connect server error: %v \n" , err )
	}
	defer resp.Body.Close()
	v := resp.Header.Get("retAddHeader")
	if v != "AddByGateway" {
		fmt.Println("Not found gateway add Header retAddHeader")
		return false
	}
	v = resp.Header.Get("retSetHeader")
	if v != "SetByGateway" {
		fmt.Println("Not found gateway add Header retSetHeader or not set by gateway")
		return false
	}
	v = resp.Header.Get("retDelHeader")
	if v != "" {
		fmt.Println("Not del header by gateway")
		return false
	}
	return true
}
