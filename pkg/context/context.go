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

package context

import (
	stdcontext "context"
)

type (
	// Protocol is type of protocol that context support
	Protocol string

	// Context is general context for HTTPContext, MQTTContext, TCPContext
	Context interface {
		stdcontext.Context
		Protocol() Protocol
	}
)

const (
	// HTTP is HTTP protocol
	HTTP Protocol = "HTTP"

	// MQTT is MQTT protocol
	MQTT Protocol = "MQTT"

	// TCP is TCP protocol
	TCP Protocol = "TCP"

	// UDP is UDP protocol
	UDP Protocol = "UDP"

	// NilErr defines nil error string
	NilErr = ""
)
