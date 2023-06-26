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
	// ApiURL is the prefix of all API URLs.
	ApiURL = "/apis/v2"

	// HealthURL is the URL of health check.
	HealthURL = ApiURL + "/healthz"

	// MembersURL is the URL of members.
	MembersURL = ApiURL + "/status/members"
	// MemberItemURL is the URL of a member.
	MemberItemURL = ApiURL + "/status/members/%s"

	// ObjectsURL is the URL of objects.
	ObjectsURL = ApiURL + "/objects"
	// ObjectItemURL is the URL of a object.
	ObjectItemURL = ApiURL + "/objects/%s"
	// ObjectKindsURL is the URL of object kinds.
	ObjectKindsURL = ApiURL + "/object-kinds"
	// ObjectTemplateURL is the URL of object template.
	ObjectTemplateURL = ApiURL + "/objects-yaml/%s/%s"

	// StatusObjectsURL is the URL of status objects.
	StatusObjectsURL = ApiURL + "/status/objects"
	// StatusObjectItemURL is the URL of a status object.
	StatusObjectItemURL = ApiURL + "/status/objects/%s"

	// WasmCodeURL is the URL of wasm code.
	WasmCodeURL = ApiURL + "/wasm/code"
	// WasmDataURL is the URL of wasm data.
	WasmDataURL = ApiURL + "/wasm/data/%s/%s"

	// CustomDataKindURL is the URL of custom data kinds.
	CustomDataKindURL = ApiURL + "/customdatakinds"
	// CustomDataKindItemURL is the URL of a custom data kind.
	CustomDataKindItemURL = ApiURL + "/customdatakinds/%s"
	// CustomDataURL is the URL of custom data.
	CustomDataURL = ApiURL + "/customdata/%s"
	// CustomDataItemURL is the URL of a custom data.
	CustomDataItemURL = ApiURL + "/customdata/%s/%s"

	// ProfileURL is the URL of profile.
	ProfileURL = ApiURL + "/profile"
	// ProfileStartURL is the URL of start profile.
	ProfileStartURL = ApiURL + "/profile/start/%s"
	// ProfileStopURL is the URL of stop profile.
	ProfileStopURL = ApiURL + "/profile/stop"

	// HTTPProtocol is prefix for HTTP protocol
	HTTPProtocol = "http://"
	// HTTPSProtocol is prefix for HTTPS protocol
	HTTPSProtocol = "https://"
)
