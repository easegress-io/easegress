package common

import (
	"fmt"
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
		return -1, fmt.Errorf("get goroutine id faild: %s", err)
	}
	return id, nil
}
