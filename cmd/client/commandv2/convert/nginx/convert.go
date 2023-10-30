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

package nginx

import (
	"github.com/megaease/easegress/v2/cmd/client/general"
	crossplane "github.com/nginxinc/nginx-go-crossplane"
)

const (
	DirectiveInclude  = "include"
	DirectiveHTTP     = "http"
	DirectiveServer   = "server"
	DirectiveLocation = "location"
)

// loadIncludes loads include files recursively.
// The result contains all directives in the include files and the original directives.
func loadIncludes(fileName string, directives crossplane.Directives, payload *crossplane.Payload) crossplane.Directives {
	res := crossplane.Directives{}
	for _, d := range directives {
		if d.Directive != DirectiveInclude {
			d.File = fileName
			res = append(res, d)
			continue
		}

		if len(d.Args) != 1 {
			continue
		}
		name := d.Args[0]
		var include crossplane.Directives
		for _, config := range payload.Config {
			if config.File == name {
				include = config.Parsed
				break
			}
		}
		if include == nil {
			general.Warnf("can't find include file %s in line %d of file %s", name, d.Line, fileName)
			continue
		}
		include = loadIncludes(name, include, payload)
		res = append(res, include...)
	}
	return res
}
