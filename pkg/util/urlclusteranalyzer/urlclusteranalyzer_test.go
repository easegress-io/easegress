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

package urlclusteranalyzer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

var wg sync.WaitGroup

func TestURLClusterAnalyzer(t *testing.T) {
	urlClusterAnalyzer := New()

	p := urlClusterAnalyzer.GetPattern("")
	fmt.Println(p)
	if p != "" {
		t.Fatal("")
	}

	p = urlClusterAnalyzer.GetPattern("city/address/1/order/3")
	fmt.Println(p)
	if p != "/city/address/1/order/3" {
		t.Fatal("")
	}

	for i := 0; i < maxValues; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/%d", i))
	}
	fmt.Println(p)
	if p != fmt.Sprintf("/%d", maxValues-1) {
		t.Fatal("")
	}

	for i := 0; i < maxValues+1; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/%d", i))
	}
	fmt.Println(p)
	if p != "/*" {
		t.Fatal("")
	}

	for i := 0; i < maxValues+10; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/orders/%d", i))
	}
	fmt.Println(p)
	if p != "/orders/*" {
		t.Fatal("")
	}

	for i := 0; i < maxValues+10; i++ {
		p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/com/megaease/users/%d/orders/%d/details", 1, i))
	}
	fmt.Println(p)
	if p != "/com/megaease/users/1/orders/*/details" {
		t.Fatal("")
	}

	begin := time.Now()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pat := ""
			for i := 0; i < 10000; i++ {
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/com%d/abc/users/%d/orders/%d/details", i%10, i, i))
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com%d/merchant/%d/sail2/%d/details", i%10, i, i))
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/merchant%d/%d/sail3/%d/details", i%10, i, i))
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/users/%d/orders/%d/details%d", i, i, i%10))
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/merchant/%d/sail/50/details", i))
				pat = urlClusterAnalyzer.GetPattern(fmt.Sprintf("prefix%d/abc/com/merchant/%d/sail15/%d/details", i%10, i, i))
			}
			fmt.Println(pat)
			if pat != "/prefix9/abc/com/merchant/*/sail15/*/details" {
				t.Fatal("")
			}
		}()
	}
	wg.Wait()
	duration := time.Since(begin)
	fmt.Println(duration)

	p = urlClusterAnalyzer.GetPattern(fmt.Sprintf("/abc/com/merchant/other/other/%d/details", 30))
	fmt.Println(p)
	if p != "/abc/com/merchant/*/other/30/details" {
		t.Fatal("")
	}
}
