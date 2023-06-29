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

package command

import "github.com/megaease/easegress/cmd/client/general"

var makeURL = general.MakeURL

const (
	apiURL = general.ApiURL

	healthURL = general.HealthURL

	membersURL = general.MembersURL
	memberURL  = general.MemberItemURL

	objectKindsURL = general.ObjectKindsURL
	objectsURL     = general.ObjectsURL
	objectURL      = general.ObjectItemURL

	statusObjectURL  = general.StatusObjectItemURL
	statusObjectsURL = general.StatusObjectsURL

	wasmCodeURL = general.WasmCodeURL
	wasmDataURL = general.WasmDataURL

	customDataKindURL     = general.CustomDataKindURL
	customDataKindItemURL = general.CustomDataKindItemURL
	customDataURL         = general.CustomDataURL
	customDataItemURL     = general.CustomDataItemURL

	profileURL      = general.ProfileURL
	profileStartURL = general.ProfileStartURL
	profileStopURL  = general.ProfileStopURL
)
