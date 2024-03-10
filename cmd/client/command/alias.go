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

// Package command provides the commands.
package command

import "github.com/megaease/easegress/v2/cmd/client/general"

var makePath = general.MakePath

const (
	apiURL = general.APIURL

	membersURL = general.MembersURL
	memberURL  = general.MemberItemURL

	objectKindsURL = general.ObjectKindsURL
	objectsURL     = general.ObjectsURL
	objectURL      = general.ObjectItemURL

	statusObjectURL  = general.StatusObjectItemURL
	statusObjectsURL = general.StatusObjectsURL

	customDataKindURL     = general.CustomDataKindURL
	customDataKindItemURL = general.CustomDataKindItemURL
	customDataURL         = general.CustomDataURL
	customDataItemURL     = general.CustomDataItemURL
)
