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

package autocertmanager

import (
	"crypto/tls"
	"fmt"
	"net/http"
)

// GetCertificate implement `GetCertificate` for tls.Config
func GetCertificate(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, fmt.Errorf("not implemented")
}

// HandleHTTP01Challenge handles HTTP-01 challenge
func HandleHTTP01Challenge(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
