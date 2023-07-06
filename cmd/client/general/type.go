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

// Package general provides the general utilities for the client.
package general

// CmdType is the type of command.
type CmdType string

const (
	GetCmd      CmdType = "get"
	CreateCmd   CmdType = "create"
	ApplyCmd    CmdType = "apply"
	DeleteCmd   CmdType = "delete"
	DescribeCmd CmdType = "describe"
	StartCmd    CmdType = "start"
	StopCmd     CmdType = "stop"
)

const (
	JSONFormat    string = "json"
	YamlFormat    string = "yaml"
	DefaultFormat string = "default"
)
