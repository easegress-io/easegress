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
	"github.com/libdns/azure"
	"github.com/libdns/cloudflare"
	"github.com/libdns/digitalocean"
	"github.com/libdns/dnspod"
	"github.com/libdns/duckdns"
	"github.com/libdns/googleclouddns"
	"github.com/libdns/hetzner"
	"github.com/libdns/libdns"
	"github.com/libdns/route53"
	"github.com/libdns/vultr"
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

	"azure": func(domain *DomainSpec) (dnsProvider, error) {
		return &azure.Provider{
			TenantId:          domain.TenantID,
			ClientId:          domain.ClientID,
			ClientSecret:      domain.ClientSecret,
			SubscriptionId:    domain.SubscriptionID,
			ResourceGroupName: domain.ResourceGroupName,
		}, nil
	},

	"cloudflare": func(domain *DomainSpec) (dnsProvider, error) {
		return &cloudflare.Provider{APIToken: domain.APIToken}, nil
	},

	"digitalocean": func(domain *DomainSpec) (dnsProvider, error) {
		return &digitalocean.Provider{APIToken: domain.APIToken}, nil
	},

	"dnspod": func(domain *DomainSpec) (dnsProvider, error) {
		return &dnspod.Provider{APIToken: domain.APIToken}, nil
	},

	"duckdns": func(domain *DomainSpec) (dnsProvider, error) {
		return &duckdns.Provider{APIToken: domain.APIToken}, nil
	},

	"google": func(domain *DomainSpec) (dnsProvider, error) {
		return &googleclouddns.Provider{Project: domain.Project}, nil
	},

	"hetzner": func(domain *DomainSpec) (dnsProvider, error) {
		return &hetzner.Provider{AuthAPIToken: domain.APIToken}, nil
	},

	"route53": func(domain *DomainSpec) (dnsProvider, error) {
		return &route53.Provider{
			AccessKeyId:     domain.AccessKeyID,
			SecretAccessKey: domain.AccessKeySecret,
			AWSProfile:      domain.AWSProfile,
		}, nil
	},

	"vultr": func(domain *DomainSpec) (dnsProvider, error) {
		return &vultr.Provider{APIToken: domain.APIToken}, nil
	},
}

func newDNSProvider(domain *DomainSpec) (dnsProvider, error) {
	fn := dnsProviders[domain.DNSProvider]
	if fn == nil {
		return nil, fmt.Errorf("unknown DNS provider: %s", domain.DNSProvider)
	}
	return fn(domain)
}
