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

// Package mqttprot implements the MQTT protocol.
package mqttprot

import "github.com/megaease/easegress/v2/pkg/protocols"

func init() {
	protocols.Register("mqtt", &Protocol{})
}

// Protocol implements protocols.Protocol for MQTT.
type Protocol struct {
}

var _ protocols.Protocol = (*Protocol)(nil)

// CreateRequest creates a new MQTT request.
func (p *Protocol) CreateRequest(req interface{}) (protocols.Request, error) {
	panic("not implemented")
}

// CreateResponse creates a new MQTT response.
func (p *Protocol) CreateResponse(resp interface{}) (protocols.Response, error) {
	panic("not implemented")
}
