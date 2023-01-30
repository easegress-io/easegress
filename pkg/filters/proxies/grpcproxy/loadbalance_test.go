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

package grpcproxy

import (
	"context"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestForwardLB(t *testing.T) {
	assertions := assert.New(t)
	key := "forward-target"
	balancer := NewLoadBalancer(&LoadBalanceSpec{Policy: "forward", ForwardKey: key}, nil)

	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(context.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)

	target := "127.0.0.1:8849"
	assertions.Nil(balancer.ChooseServer(req))

	req.Header().Set(key, target)

	lb, ok := balancer.(ReusableServerLB)
	assertions.True(ok)
	assertions.Equal(target, lb.ChooseServer(req).URL)

}
