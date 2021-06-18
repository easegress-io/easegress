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

package api

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/megaease/easegress/pkg/logger"
)

func (s *Server) newAPILogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		t1 := time.Now()
		defer func() {
			logger.APIAccess(r.Method, r.RemoteAddr, r.URL.Path, ww.Status(),
				r.ContentLength, int64(ww.BytesWritten()),
				t1, time.Since(t1))
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) newRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {
				logger.Errorf("recover from %s, err: %v, stack trace:\n%s\n",
					r.URL.Path, rvr, debug.Stack())

				if ce, ok := rvr.(clusterErr); ok {
					HandleAPIError(w, r, http.StatusServiceUnavailable, ce)
				} else {
					HandleAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("%v", rvr))
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) newConfigVersionAttacher(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NOTE: It needs to add the header before the next handlers
		// write the body to the network.
		version := s._getVersion()
		w.Header().Set(ConfigVersionKey, fmt.Sprintf("%d", version))
		next.ServeHTTP(w, r)
	})
}
