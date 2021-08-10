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

package common

// Signal is cross-platform abstract type of os.Signal
type Signal string

const (
	// SignalInt represents quit in Easegress
	SignalInt Signal = "int"
	// SignalTerm represents force quit in Easegress
	SignalTerm Signal = "term"

	// SignalUsr2 represents reload signal in Easegress
	SignalUsr2 Signal = "usr2"
)
