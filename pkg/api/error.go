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

package api

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type (
	clusterErr string

	// Err is the standard return of error.
	Err struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

func (ce clusterErr) Error() string {
	return string(ce)
}

// ClusterPanic panics because of the cluster-level fault.
func ClusterPanic(err error) {
	panic(clusterErr(err.Error()))
}

// HandleAPIError handles api error.
func HandleAPIError(w http.ResponseWriter, r *http.Request, code int, err error) {
	w.WriteHeader(code)
	buff, err := codectool.MarshalJSON(Err{
		Code:    code,
		Message: err.Error(),
	})
	if err != nil {
		panic(err)
	}
	w.Write(buff)
}
