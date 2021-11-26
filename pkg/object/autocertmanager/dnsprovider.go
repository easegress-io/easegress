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

package autocertmanager

import (
	"fmt"

	"github.com/libdns/alidns"
	"github.com/libdns/libdns"
)

type dnsProvider interface {
	libdns.RecordAppender
	libdns.RecordGetter
	libdns.RecordSetter
	libdns.RecordDeleter
}

var dnsProviders = map[string]func(domain *DomainSpec) (dnsProvider, error){
	"alidns": func(domain *DomainSpec) (dnsProvider, error) {
		return &alidns.Provider{
			AccKeyID:     domain.AccessKeyID,
			AccKeySecret: domain.AccessKeySecret,
		}, nil
	},
}

func newDNSProvider(domain *DomainSpec) (dnsProvider, error) {
	fn := dnsProviders[domain.DNSProvider]
	if fn == nil {
		return nil, fmt.Errorf("unknown DNS provider: %s", domain.DNSProvider)
	}
	return fn(domain)
}
