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

package autocertmanager

import (
	"fmt"

	"github.com/libdns/alidns"
	"github.com/libdns/azure"
	"github.com/libdns/cloudflare"
	"github.com/libdns/digitalocean"
	"github.com/libdns/dnspod"
	"github.com/libdns/duckdns"

	// "github.com/libdns/googleclouddns"
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

type dnsProviderCreator struct {
	requiredFields []string
	creatorFn      func(d *DomainSpec) (dnsProvider, error)
}

func newDNSProvider(d *DomainSpec) (dnsProvider, error) {
	if len(d.DNSProvider) == 0 {
		return nil, fmt.Errorf("DNS provider is not configured for domain: %s", d.Name)
	}

	name := d.DNSProvider["name"]
	if len(name) == 0 {
		return nil, fmt.Errorf("DNS provider name is required for domain: %s", d.Name)
	}

	if _, ok := d.DNSProvider["zone"]; !ok {
		return nil, fmt.Errorf("DNS provider field 'zone' is required for domain: %s", d.Name)
	}

	creator := dnsProviderCreators[name]
	if creator == nil {
		return nil, fmt.Errorf("unknown DNS provider %q for domain: %s", name, d.Name)
	}

	for _, f := range creator.requiredFields {
		if _, ok := d.DNSProvider[f]; !ok {
			return nil, fmt.Errorf("DNS provider field %q is required for domain: %s", f, d.Name)
		}
	}

	return creator.creatorFn(d)
}

var dnsProviderCreators = map[string]*dnsProviderCreator{
	"alidns": {
		requiredFields: []string{"accessKeyId", "accessKeySecret"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &alidns.Provider{
				AccKeyID:     d.DNSProvider["accessKeyId"],
				AccKeySecret: d.DNSProvider["accessKeySecret"],
			}, nil
		},
	},

	"azure": {
		requiredFields: []string{"tenantId", "clientId", "clientSecret", "subscriptionId", "resourceGroupName"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &azure.Provider{
				TenantId:          d.DNSProvider["tenantId"],
				ClientId:          d.DNSProvider["clientId"],
				ClientSecret:      d.DNSProvider["clientSecret"],
				SubscriptionId:    d.DNSProvider["subscriptionId"],
				ResourceGroupName: d.DNSProvider["resourceGroupName"],
			}, nil
		},
	},

	"cloudflare": {
		requiredFields: []string{"apiToken"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &cloudflare.Provider{APIToken: d.DNSProvider["apiToken"]}, nil
		},
	},

	"digitalocean": {
		requiredFields: []string{"apiToken"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &digitalocean.Provider{APIToken: d.DNSProvider["apiToken"]}, nil
		},
	},

	"dnspod": {
		requiredFields: []string{"apiToken"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &dnspod.Provider{APIToken: d.DNSProvider["apiToken"]}, nil
		},
	},

	"duckdns": {
		requiredFields: []string{"apiToken"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &duckdns.Provider{APIToken: d.DNSProvider["apiToken"]}, nil
		},
	},

	/*
		"google": {
			requiredFields: []string{"project"},
			creatorFn: func(d *DomainSpec) (dnsProvider, error) {
				return &googleclouddns.Provider{Project: d.DNSProvider["project"]}, nil
			},
		},
	*/

	"hetzner": {
		requiredFields: []string{"authApiToken"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &hetzner.Provider{AuthAPIToken: d.DNSProvider["authApiToken"]}, nil
		},
	},

	"route53": {
		requiredFields: []string{"accessKeyId", "secretAccessKey", "awsProfile"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &route53.Provider{
				AccessKeyId:     d.DNSProvider["accessKeyId"],
				SecretAccessKey: d.DNSProvider["secretAccessKey"],
				AWSProfile:      d.DNSProvider["awsProfile"],
			}, nil
		},
	},

	"vultr": {
		requiredFields: []string{"apiToken"},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &vultr.Provider{APIToken: d.DNSProvider["apiToken"]}, nil
		},
	},
}
