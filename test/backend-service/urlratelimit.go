package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type hitStruct struct {
	lastHitCount int64
	hitCount     int64
}
type urlHitMap struct {
	m     map[string]hitStruct
	mutex *sync.RWMutex
}

func (this *urlHitMap) Increase(key string) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	i, ok := this.m[key]
	if !ok {
		this.m[key] = hitStruct{0, 1}
	}
	i.hitCount += 1
	this.m[key] = i
}
func (this *urlHitMap) Display() {
	nowtime := time.Now().Format("20060102150405")
	for k, v := range this.m {
		this.mutex.Lock()
		intvHitCount := v.hitCount - v.lastHitCount
		v.lastHitCount = v.hitCount
		this.m[k] = v
		this.mutex.Unlock()
		fmt.Printf("%s	%d	%d	%s\n", nowtime, intvHitCount, v.hitCount, k)
	}
}
func main() {
	umap := &urlHitMap{make(map[string]hitStruct), new(sync.RWMutex)}
	rateHander := func(w http.ResponseWriter, req *http.Request) {
		urlpath := req.URL.Path
		umap.Increase(urlpath)
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}
	go func() {
		fmt.Printf("CURRTIME	SHIT	TOTHIT	PATH\n")
		for ; ;
		{
			time.Sleep(1 * time.Second)
			umap.Display()
		}
	}()
	http.HandleFunc("/", rateHander)
	http.ListenAndServe(":9098", nil)
}
