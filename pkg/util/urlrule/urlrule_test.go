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

package urlrule

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

func TestURLRULEMatch(t *testing.T) {
	u := &URLRule{
		id: "TestURLRULEMatch",
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Prefix: "/",
		},
	}

	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}
	if u.ID() != u.id {
		t.Error("Rule ID is wrong")
	}

	u.Init()

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/user/api", nil)

	if !u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req is %#v, urlrule is %#v", req, u)
	}
}

func TestURLRegxMatch(t *testing.T) {
	u := &URLRule{
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			RegEx: "^\\/app\\/.+$",
		},
	}
	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}
	u.URL.Init()

	u.Init()

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/app/api", nil)

	if !u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req path is %s, urlrule is regEx: %s", req.URL.Path, u.URL.RegEx)
	}
}

func TestURLExactMatch(t *testing.T) {
	u := &URLRule{
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact: "/app/v2/user",
		},
	}
	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}

	u.Init()

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/app/v2/user", nil)

	if !u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req path is %s, urlrule is exact : %s", req.URL.Path, u.URL.Exact)
	}
}

func TestURLExactNotMatch(t *testing.T) {
	u := &URLRule{
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact: "/app/v2/user",
		},
	}
	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}

	u.Init()

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/app/v3/user", nil)

	if u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req path is %s, urlrule is exact : %s", req.URL.Path, u.URL.Exact)
	}
}

func TestURLPrefixNotMatch(t *testing.T) {
	u := &URLRule{
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Prefix: "/app/v3",
		},
	}
	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}

	u.Init()

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/app/v2/user", nil)

	if u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req path is %s, urlrule is exact : %s", req.URL.Path, u.URL.Exact)
	}
}

func TestURLRULENoMatchMethod(t *testing.T) {
	u := &URLRule{
		Methods: []string{
			"DELETE",
		},
		URL: stringtool.StringMatcher{
			Prefix: "/",
		},
	}
	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/user/api", nil)

	if u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req mehtod is %#v, urlrule method is required: %v", req.Method, u.Methods)
	}
}

func TestURLRULENoMatchURL(t *testing.T) {
	u := &URLRule{
		Methods: []string{
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact: "/user",
		},
	}
	if err := u.URL.Validate(); err != nil {
		t.Error(err)
	}

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/user/api", nil)

	if u.Match(req) {
		t.Errorf("match HTTP URL and Method failed, req path is %s, urlrule path is required exact : %s", req.URL.Path, u.URL.Exact)
	}
}

func TestFailStringMatch(t *testing.T) {
	sm := stringtool.StringMatcher{}

	err := sm.Validate()
	if err == nil {
		t.Error("string matcher should be invalid")
	}
}

func TestDeepEqual(t *testing.T) {
	u1 := &URLRule{
		id: "TestDeepEqual1",
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact: "/app/v2/user",
		},
	}

	u2 := &URLRule{
		id: "TestDeepEqual2",
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact: "/app/v2/user",
		},
	}

	u3 := &URLRule{
		id: "TestDeepEqual3",
		Methods: []string{
			"GET",
			"POST",
			"DELETE",
		},
		URL: stringtool.StringMatcher{
			RegEx: "^/app/v2",
		},
	}
	u4 := &URLRule{
		id: "TestDeepEqual4",
		Methods: []string{
			"GET",
			"POST",
			"PATCH",
		},
		URL: stringtool.StringMatcher{
			Prefix: "/app/v3/user",
		},
	}

	u5 := &URLRule{
		id: "TestDeepEqual1",
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact: "/app/v3/user",
		},
	}

	u6 := &URLRule{
		id: "TestDeepEqual2",
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact:  "/app/v2/user",
			Prefix: "/app",
		},
	}

	u7 := &URLRule{
		id: "TestDeepEqual2",
		Methods: []string{
			"GET",
			"POST",
		},
		URL: stringtool.StringMatcher{
			Exact:  "/app/v2/user",
			Prefix: "/app",
			RegEx:  "^.*$",
		},
	}

	if !u1.DeepEqual(u2) {
		t.Errorf("%v and %v should be equal\n", u1, u2)
	}
	if u1.DeepEqual(u3) {
		t.Errorf("%v and %v should not be equal\n", u1, u3)
	}
	if u3.DeepEqual(u4) {
		t.Errorf("%v and %v should not be equal\n", u3, u4)
	}

	if u1.DeepEqual(u5) {
		t.Errorf("%v and %v should not be equal\n", u1, u5)
	}

	if u1.DeepEqual(u6) {
		t.Errorf("%v and %v should not be equal\n", u1, u6)
	}

	if u6.DeepEqual(u7) {
		t.Errorf("%v and %v should not be equal\n", u6, u7)
	}
}
