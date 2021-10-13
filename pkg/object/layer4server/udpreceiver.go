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

package layer4server

import (
	"strings"
	"sync"
)

var (
	ProxyMap = sync.Map{}
)

// GetProxyMapKey construct udp session key
func GetProxyMapKey(raddr, laddr string) string {
	var builder strings.Builder
	builder.WriteString(raddr)
	builder.WriteString(":")
	builder.WriteString(laddr)
	return builder.String()
}

// SetUDPProxyMap set udp session by udp server listener
func SetUDPProxyMap(key string, clientConn interface{}) {
	ProxyMap.Store(key, clientConn)
}

// DelUDPProxyMap delete udp session
func DelUDPProxyMap(key string) {
	ProxyMap.Delete(key)
}
