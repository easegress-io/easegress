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

// CmdType is the type of command.
type CmdType string

const (
	// GetCmd is the get command.
	GetCmd CmdType = "get"
	// EditCmd is the edit command.
	EditCmd CmdType = "edit"
	// CreateCmd is the create command.
	CreateCmd CmdType = "create"
	// ApplyCmd is the apply command.
	ApplyCmd CmdType = "apply"
	// DeleteCmd is the delete command.
	DeleteCmd CmdType = "delete"
	// DescribeCmd is the describe command.
	DescribeCmd CmdType = "describe"
	// StartCmd is the start command.
	StartCmd CmdType = "start"
	// StopCmd is the stop command.
	StopCmd CmdType = "stop"
)

const (
	// JSONFormat is the json format.
	JSONFormat string = "json"
	// YamlFormat is the yaml format.
	YamlFormat string = "yaml"
	// DefaultFormat is the default format.
	DefaultFormat string = "default"
)
