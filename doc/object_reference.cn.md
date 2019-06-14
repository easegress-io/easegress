## 概述
1. object的key均为非空string类型。
2. 时间类型兼容不同的后缀单位：`ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`。

## 基础配置
所有Object需提供如下基本配置

| 配置名                     	 | 类型     | 是否可以为空 	| 默认值 	| 正确性                               	|
|----------------------------	|--------	|:------------:|--------	|--------------------------------------	|
| name                       	| string 	|     非空     	| ""     	| regexp: `^[A-Za-z0-9\-_\.~]{1,253}$` 	|
| kind                       	| string 	|     非空     	| ""     	| `HTTPServer` 或者 `HTTPProxy` |

## HTTPServer
| 配置名                     	| 类型   	| 是否可以为空 	| 默认值 	| 正确性                               	|
|----------------------------	|--------	|:------------:	|--------	|--------------------------------------	|
| port                       	| int    	|     非空     	| 10080  	| 1-65535                              	|
| keepAlive                  	| bool   	|     可空     	| true   	|                                      	|
| keepAliveTimeout           	| string 	|     可空     	| 60s    	| 最小1s                               	|
| maxConnections             	| int    	|     可空     	| 10240  	| 最小1                                	|
| https                      	| bool   	|     可空     	|        	|                                      	|
| certBase64                 	| string 	|     可空     	|        	| 合法base64，且能解码为公钥           	|
| keyBase64                  	| string 	|     可空     	|        	| 合法base64，且能解码为私钥           	|
| cacheSize                  	| int    	|     可空     	|        	|                                      	|
| ipFilter                   	| object 	|     可空     	|        	|                                      	|
| rules                      	| array  	|     可空     	|        	|                                      	|
| rules[].ipFilter           	| object 	|     可空     	|        	|                                      	|
| rules[].host               	| string 	|     可空     	|        	|                                      	|
| rules[].hostRegexp         	| string 	|     可空     	|        	| regexp                               	|
| rules[].paths              	| array  	|     可空     	|        	|                                      	|
| rules[].paths[].ipFilter   	| object 	|     可空     	|        	|                                      	|
| rules[].paths[].path       	| string 	|     可空     	|        	| 要以`/`开头                          	|
| rules[].paths[].pathPrefix 	| string 	|     可空     	|        	| 要以`/`开头                          	|
| rules[].paths[].pathRegexp 	| string 	|     可空     	|        	| regexp                               	|
| rules[].paths[].Methods    	| array  	|     可空     	|        	|                                      	|
| rules[].paths[].Methods[]  	| string 	|     可空     	|        	| 合法HTTP方法                         	|
| rules[].paths[].Backend    	| string 	|     非空     	|        	|                                      	|
| *.ipFilter.blockByDefault  	| bool   	|     可空     	|        	|                                      	|
| *.ipFilter.allowIPs        	| array  	|     可空     	|        	|                                      	|
| *.ipFilter.allowIPs[]      	| string 	|     非空     	|        	| 合法IPv4、IPv4 CIDR、IPv6、IPv6 CIDR 	|
| *.ipFilter.blockIPs        	| array  	|     可空     	|        	|                                      	|
| *.ipFilter.blockIPs[]      	| string 	|     非空     	|        	| 合法IPv4、IPv4 CIDR、IPv6、IPv6 CIDR 	|

## HTTPProxy
| 配置名                                        	| 类型   	| 是否可以为空 	| 正确性                                          	|
|-----------------------------------------------	|--------	|:------------:	|-------------------------------------------------	|
| server                                        	| string 	|     非空     	|                                                 	|
| fallback                                      	| object 	|     可空     	|                                                 	|
| fallback.forRateLimiter                       	| bool   	|     可空     	|                                                 	|
| fallback.forCircuitBreaker                    	| bool   	|     可空     	|                                                 	|
| fallback.forCandidateBackendCodes             	| array  	|     可空     	| 元素需unique                                    	|
| fallback.forCandidateBackendCodes[]           	| int    	|     非空     	| 合法HTTP Code                                   	|
| fallback.forBackendCodes                      	| array  	|     可空     	| 元素需unique                                    	|
| fallback.forBackendCodes[]                    	| int    	|     非空     	| 合法HTTP Code                                   	|
| fallback.mockCode                             	| int    	|     非空     	| 合法HTTP Code                                   	|
| fallback.mockHeaders                          	| object 	|     可空     	|                                                 	|
| fallback.mockHeaders[]                        	| string 	|     非空     	|                                                 	|
| fallback.mockBody                             	| string 	|     可空     	|                                                 	|
| validator                                     	| object 	|     可空     	|                                                 	|
| validator.headers                             	| object 	|     可空     	|                                                 	|
| validator.headers[]                           	| object 	|     非空     	|                                                 	|
| validator.headers[].values                    	| array  	|     可空     	| 元素需unique                                    	|
| validator.headers[].values[]                  	| string 	|     可空     	|                                                 	|
| validator.headers[].regexp                    	| string 	|     可空     	| regexp                                          	|
| rateLimiter                                   	| object 	|     可空     	|                                                 	|
| rateLimiter.tps                               	| int    	|     非空     	| 最小1                                           	|
| rateLimiter.timeout                           	| string 	|     可空     	| 最小1ms                                         	|
| circuitBreaker                                	| object 	|     可空     	|                                                 	|
| circuitBreaker.failureCodes                   	| array  	|     非空     	| 元素需unique                                    	|
| circuitBreaker.failureCodes[]                 	| int    	|     非空     	| 合法HTTP Code                                   	|
| circuitBreaker.countPeriod                    	| string 	|     非空     	| 最小1s                                          	|
| circuitBreaker.toClosedConsecutiveCounts      	| int    	|     非空     	| 最小1                                           	|
| circuitBreaker.toHalfOpenTimeout              	| string 	|     非空     	| 最小1s                                          	|
| circuitBreaker.toOpenFailureCounts            	| int    	|     可空     	| 最小1                                           	|
| circuitBreaker.toOpenFailureConsecutiveCounts 	| int    	|     可空     	| 最小1                                           	|
| adaptor                                       	| object 	|     可空     	|                                                 	|
| adaptor.request                               	| object 	|     可空     	|                                                 	|
| adaptor.request.method                        	| string 	|     可空     	| 合法HTTP方法                                    	|
| adaptor.request.path                          	| object 	|     可空     	|                                                 	|
| adaptor.request.path.replace                  	| string 	|     可空     	|                                                 	|
| adaptor.request.path.addPrefix                	| string 	|     可空     	| 要以`/`开头                                     	|
| adaptor.request.path.trimPrefix               	| string 	|     可空     	| 要以`/`开头                                     	|
| adaptor.request.path.RegexpReplace            	| object 	|     可空     	|                                                 	|
| adaptor.request.path.RegexpReplace.Regexp     	| string 	|     非空     	| regexp                                          	|
| adaptor.request.path.RegexpReplace.Replace    	| string 	|     非空     	|                                                 	|
| adaptor.request.header                        	| object 	|     可空     	|                                                 	|
| adaptor.request.header.del                    	| array  	|     可空     	| 元素需unique                                    	|
| adaptor.request.header.del[]                  	| string 	|     非空     	|                                                 	|
| adaptor.request.header.set                    	| object 	|     可空     	|                                                 	|
| adaptor.request.header.set[]                  	| string 	|     可空     	|                                                 	|
| adaptor.request.header.add                    	| object 	|     可空     	|                                                 	|
| adaptor.request.header.add[]                  	| string 	|     可空     	|                                                 	|
| adaptor.response                              	| object 	|     可空     	|                                                 	|
| adaptor.response.header                       	| object 	|     可空     	| 同adaptor.request.adaptor                       	|
| mirrorBackend                                 	| object 	|     可空     	|                                                 	|
| mirrorBackend.filter                          	| object 	|     可空     	|                                                 	|
| mirrorBackend.filter[]                        	| object 	|     非空     	|                                                 	|
| mirrorBackend.filter[].values                 	| array  	|     可空     	| 元素需unique                                    	|
| mirrorBackend.filter[].values[]               	| string 	|     可空     	|                                                 	|
| mirrorBackend.filter[].regexp                 	| string 	|     可空     	| regexp                                          	|
| candidateBackend                              	| object 	|     可空     	|                                                 	|
| candidateBackend.filter                       	| object 	|     非空     	| 同mirrorBackend.filter                          	|
| backend                                       	| object 	|     非空     	|                                                 	|
| *[Bb]ackend.serversTags                       	| array  	|     可空     	|                                                 	|
| *[Bb]ackend.serversTags[]                     	| string 	|     非空     	|                                                 	|
| *[Bb]ackend.servers                           	| array  	|     非空     	|                                                 	|
| *[Bb]ackend.servers[]                         	| object 	|     非空     	|                                                 	|
| *[Bb]ackend.servers[].url                     	| string 	|     非空     	| 合法url                                         	|
| *[Bb]ackend.servers[].tags                    	| array  	|     可空     	|                                                 	|
| *[Bb]ackend.servers[].tags[]                  	| string 	|     非空     	|                                                 	|
| *[Bb]ackend.loadBalance                       	| object 	|     非空     	|                                                 	|
| *[Bb]ackend.loadBalance.policy                	| string 	|     非空     	| 为roundRobin random ipHash headerHash其一       	|
| *[Bb]ackend.loadBalance.headerHashKey         	| string 	|     可空     	| 当policy为headerHash，必须非空                  	|
| *[Bb]ackend.adaptor                           	| object 	|     可空     	| 同adaptor，mirrorBackend.adaptor.response必为空 	|
| *[Bb]ackend.memoryCache                       	| object 	|     可空     	|                                                 	|
| *[Bb]ackend.memoryCache.expiration            	| string 	|     非空     	| 最小1s                                          	|
| *[Bb]ackend.memoryCache.maxEntryBytes         	| int    	|     非空     	| 最小1                                           	|
| *[Bb]ackend.memoryCache.size                  	| int    	|     非空     	| 最小1                                           	|
| *[Bb]ackend.memoryCache.codes                 	| array  	|     非空     	| 元素需unique                                    	|
| *[Bb]ackend.memoryCache.codes[]               	| int    	|     非空     	| 合法HTTP Code                                   	|
| *[Bb]ackend.memoryCache.methods               	| array  	|     非空     	| 元素需unique                                    	|
| *[Bb]ackend.memoryCache.methods[]             	| string 	|     非空     	| 合法HTTP方法                                    	|
| compression                                   	| object 	|     可空     	|                                                 	|
| compression.minLenth                          	| int    	|     可空     	| 最小0                                           	|
