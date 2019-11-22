package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var p = flag.Int("p", 0, "listen port")

func main() {
	flag.Parse()
	var lastHitCount int64 = 0
	var hitCount int64 = 0
	var trigger bool = false
	rateHander := func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt64(&hitCount, 1)
		time.Sleep(50 * time.Millisecond)
		if !trigger {
			w.WriteHeader(http.StatusServiceUnavailable)
		}else {
			w.WriteHeader(http.StatusOK)
		}
	}
	go func() {
		for ; ;
		{
			trigger = !trigger
			fmt.Printf("trigger service retok %v\n", trigger)
			time.Sleep(30 * time.Second)
		}
	}()
	go func() {
		fmt.Printf("CURRTIME	10SHIT	TOTHIT\n")
		for ; ;
		{
			intvHitCount := hitCount - lastHitCount
			lastHitCount = hitCount
			fmt.Printf("%s	%d	%d\n", time.Now().Format("20060102150405"), intvHitCount, hitCount)
			time.Sleep(1 * time.Second)
		}
	}()
	http.HandleFunc("/", rateHander)
	portstr := ":9098"
	if *p != 0 {
		portstr = fmt.Sprintf(":%d", *p)
	}
	http.ListenAndServe(portstr, nil)
}
