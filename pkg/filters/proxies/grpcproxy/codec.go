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

package grpcproxy

import (
	"google.golang.org/protobuf/proto"
)

const (
	codecName = "easegress-raw"
)

// GrpcCodec impl grpc.Codec instead of encoding.Codec, because encoding.Codec would changed grpc.header
// eg: if use encoding.Codec, then header's content-type=application/grpc -> content-type=application/grpc+xxx
// xxx from encoding.Codec.Name()
type GrpcCodec struct{}

type frame struct {
	payload []byte
}

// Marshal object to []byte
func (GrpcCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*frame)
	if !ok {
		return proto.Marshal(v.(proto.Message))
	}
	return out.payload, nil
}

// Unmarshal []byte to object
func (GrpcCodec) Unmarshal(data []byte, v interface{}) error {
	if s, ok := v.(*frame); ok {
		(*s).payload = data
		return nil
	}
	return proto.Unmarshal(data, v.(proto.Message))
}

// Name return codec name
func (GrpcCodec) Name() string {
	return codecName
}
