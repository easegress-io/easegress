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

package urlclusteranalyzer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var wg sync.WaitGroup

func TestURLClusterAnalyzer(t *testing.T) {
	assert := assert.New(t)

	urlClusterAnalyzer := New()

	p := urlClusterAnalyzer.GetPattern("")
	assert.Equal("", p)

	p = urlClusterAnalyzer.GetPattern("city/address/1/order/3")
	assert.Equal("/city/address/1/order/3", p)

	for i := 0; i < maxValues; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/%d", i))
	}
	assert.Equal(fmt.Sprintf("/%d", maxValues-1), p)

	for i := 0; i < maxValues+1; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/%d", i))
	}
	assert.Equal("/*", p)

	for i := 0; i < maxValues+10; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/orders/%d", i))
	}
	assert.Equal("/orders/*", p)

	for i := 0; i < maxValues+10; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/com/megaease/users/%d/orders/%d/details", 1, i))
	}
	assert.Equal("/com/megaease/users/1/orders/*/details", p)

	begin := time.Now()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pat := ""
			for i := 0; i < 10000; i++ {
				urlClusterAnalyzer.GetPattern(fmt.Sprintf("/com%d/abc/users/%d/orders/%d/details", i%10, i, i))
				urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com%d/merchant/%d/sail2/%d/details", i%10, i, i))
				urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/merchant%d/%d/sail3/%d/details", i%10, i, i))
				urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/users/%d/orders/%d/details%d", i, i, i%10))
				urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/merchant/%d/sail/50/details", i))
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("prefix%d/abc/com/merchant/%d/sail15/%d/details", i%10, i, i))
			}
			assert.Equal("/prefix9/abc/com/merchant/*/sail15/*/details", pat)
		}()
	}
	wg.Wait()
	duration := time.Since(begin)
	fmt.Println(duration)

	p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/merchant/other/other/%d/details", 30))
	assert.Equal("/abc/com/merchant/*/other/30/details", p)
}
