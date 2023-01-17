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

package grpcprot

import (
	"fmt"
	"github.com/megaease/easegress/pkg/protocols"
	"google.golang.org/grpc"
)

// Protocol implements protocols.Protocol for HTTP.
type Protocol struct {
}

var _ protocols.Protocol = (*Protocol)(nil)

func (p *Protocol) CreateRequest(req interface{}) (protocols.Request, error) {
	if r, ok := req.(grpc.ServerStream); ok {
		return NewRequestWithServerStream(r), nil
	} else {
		return nil, fmt.Errorf("input param's type should be grpc.ServerStream")
	}
}

func (p *Protocol) CreateResponse(resp interface{}) (protocols.Response, error) {
	return NewResponse(), nil
}

func (p *Protocol) NewRequestInfo() interface{} {
	panic("implement me")
}

func (p *Protocol) BuildRequest(reqInfo interface{}) (protocols.Request, error) {
	panic("implement me")
}

func (p *Protocol) NewResponseInfo() interface{} {
	panic("implement me")
}

func (p *Protocol) BuildResponse(respInfo interface{}) (protocols.Response, error) {
	panic("implement me")
}

func (p *Protocol) BuildRequestWithRef(ref, reqInfo interface{}) (protocols.Request, error) {
	panic("not implemented")
}
