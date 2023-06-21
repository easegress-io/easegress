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

// Package command implements commands of Easegress client.
package general

type (
	// GlobalFlags is the global flags for the whole client.
	GlobalFlags struct {
		Server             string
		ForceTLS           bool
		InsecureSkipVerify bool
		OutputFormat       string
	}

	// APIErr is the standard return of error.
	APIErr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

func (g *GlobalFlags) DefaultFormat() bool {
	return g.OutputFormat == DefaultFormat
}

// CmdGlobalFlags is the singleton of GlobalFlags.
var CmdGlobalFlags GlobalFlags

const (
	ApiURL = "/apis/v2"

	HealthURL = ApiURL + "/healthz"

	MembersURL    = ApiURL + "/status/members"
	MemberItemURL = ApiURL + "/status/members/%s"

	ObjectsURL        = ApiURL + "/objects"
	ObjectItemURL     = ApiURL + "/objects/%s"
	ObjectKindsURL    = ApiURL + "/object-kinds"
	ObjectTemplateURL = ApiURL + "/objects-yaml/%s/%s"

	StatusObjectsURL    = ApiURL + "/status/objects"
	StatusObjectItemURL = ApiURL + "/status/objects/%s"

	WasmCodeURL = ApiURL + "/wasm/code"
	WasmDataURL = ApiURL + "/wasm/data/%s/%s"

	CustomDataKindURL     = ApiURL + "/customdatakinds"
	CustomDataKindItemURL = ApiURL + "/customdatakinds/%s"
	CustomDataURL         = ApiURL + "/customdata/%s"
	CustomDataItemURL     = ApiURL + "/customdata/%s/%s"

	ProfileURL      = ApiURL + "/profile"
	ProfileStartURL = ApiURL + "/profile/start/%s"
	ProfileStopURL  = ApiURL + "/profile/stop"

	// HTTPProtocol is prefix for HTTP protocol
	HTTPProtocol = "http://"
	// HTTPSProtocol is prefix for HTTPS protocol
	HTTPSProtocol = "https://"
)
