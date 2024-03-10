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
	"context"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestForwardLB(t *testing.T) {
	key := "forward-target"
	lb := newForwardLoadBalancer(&LoadBalanceSpec{Policy: "forward", ForwardKey: key})
	assert.Panics(t, func() {
		req, _ := httpprot.NewRequest(nil)
		lb.ChooseServer(req)
	})

	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(context.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)
	assert.Nil(t, lb.ChooseServer(req))

	target := "127.0.0.1:8849"
	req.Header().Set(key, target)
	svr := lb.ChooseServer(req)
	assert.Equal(t, target, svr.URL)

	lb.ReturnServer(svr, req, nil)
	lb.Close()
}
