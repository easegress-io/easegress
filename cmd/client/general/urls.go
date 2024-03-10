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

// Package general provides the general utilities for the client.
package general

const (
	// APIURL is the prefix of all API URLs.
	APIURL = "/apis/v2"

	// HealthURL is the URL of health check.
	HealthURL = APIURL + "/healthz"

	// MembersURL is the URL of members.
	MembersURL = APIURL + "/status/members"
	// MemberItemURL is the URL of a member.
	MemberItemURL = APIURL + "/status/members/%s"

	// ObjectsURL is the URL of objects.
	ObjectsURL = APIURL + "/objects"
	// ObjectItemURL is the URL of a object.
	ObjectItemURL = APIURL + "/objects/%s"
	// ObjectKindsURL is the URL of object kinds.
	ObjectKindsURL = APIURL + "/object-kinds"
	// ObjectTemplateURL is the URL of object template.
	ObjectTemplateURL = APIURL + "/objects-yaml/%s/%s"
	// ObjectAPIResources is the URL of object api resources.
	ObjectAPIResources = APIURL + "/object-api-resources"

	// StatusObjectsURL is the URL of status objects.
	StatusObjectsURL = APIURL + "/status/objects"
	// StatusObjectItemURL is the URL of a status object.
	StatusObjectItemURL = APIURL + "/status/objects/%s"

	// WasmCodeURL is the URL of wasm code.
	WasmCodeURL = APIURL + "/wasm/code"
	// WasmDataURL is the URL of wasm data.
	WasmDataURL = APIURL + "/wasm/data/%s/%s"

	// CustomDataKindURL is the URL of custom data kinds.
	CustomDataKindURL = APIURL + "/customdatakinds"
	// CustomDataKindItemURL is the URL of a custom data kind.
	CustomDataKindItemURL = APIURL + "/customdatakinds/%s"
	// CustomDataURL is the URL of custom data.
	CustomDataURL = APIURL + "/customdata/%s"
	// CustomDataItemURL is the URL of a custom data.
	CustomDataItemURL = APIURL + "/customdata/%s/%s"

	// ProfileURL is the URL of profile.
	ProfileURL = APIURL + "/profile"
	// ProfileStartURL is the URL of start profile.
	ProfileStartURL = APIURL + "/profile/start/%s"
	// ProfileStopURL is the URL of stop profile.
	ProfileStopURL = APIURL + "/profile/stop"

	// LogsURL is the URL of logs.
	LogsURL = APIURL + "/logs"
	// LogsLevelURL is the URL of logs level.
	LogsLevelURL = APIURL + "/logs/level"

	// MetricsURL is the URL of metrics.
	MetricsURL = APIURL + "/metrics"

	// HTTPProtocol is prefix for HTTP protocol
	HTTPProtocol = "http://"
	// HTTPSProtocol is prefix for HTTPS protocol
	HTTPSProtocol = "https://"

	defaultServer = "localhost:2381"
)
