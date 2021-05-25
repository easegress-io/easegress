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
