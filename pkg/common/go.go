package common

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

func GoID() (int, error) {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		return -1, fmt.Errorf("get goroutine id faild: %v", err)
	}
	return id, nil
}

func CloseChan(c interface{}) (ret bool) {
	val := reflect.ValueOf(c)
	if val.IsNil() {
		ret = false
		return
	}

	channel := val
	if val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		channel = val.Elem()
	}

	defer func() {
		ret = recover() == nil
	}()

	channel.Close()

	if channel.CanSet() {
		channel.Set(reflect.Zero(channel.Type()))
	}

	return
}
