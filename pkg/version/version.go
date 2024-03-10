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

// Package version defines the version of Easegress.
package version

import "fmt"

var (
	// RELEASE returns the release version
	RELEASE = "UNKNOWN"
	// REPO returns the git repository URL
	REPO = "UNKNOWN"
	// COMMIT returns the short sha from git
	COMMIT = "UNKNOWN"

	// API return the API version
	API = "v1"
	// Short return the short version
	Short = fmt.Sprintf("Easegress %s", RELEASE)
	// Long return the long version
	Long = fmt.Sprintf("Easegress release: %s, repo: %s, commit: %s", RELEASE, REPO, COMMIT)
)
