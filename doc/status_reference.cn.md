## 概述
1. object的key均为非空string类型。
2. 时间类型兼容不同的后缀单位：`ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`。

## 节点
| 监控指标                      	| 类型   	| 值可定范围                         	|
|-------------------------------	|--------	|------------------------------------	|
| name                          	| string 	|                                    	|
| cluster-name                  	| string 	|                                    	|
| cluster-role                  	| string 	| writer/reader                      	|
| force-new-cluster             	| bool   	|                                    	|
| cluster-client-url            	| string 	|                                    	|
| cluster-peer-url              	| string 	|                                    	|
| cluster-join-urls             	| string 	| 逗号`,`为分隔符                    	|
| api-addr                      	| string 	|                                    	|
| data-dir                      	| string 	|                                    	|
| wal-dir                       	| string 	|                                    	|
| log-dir                       	| string 	|                                    	|
| conf-dir                      	| string 	|                                    	|
| debug                         	| bool   	|                                    	|
| cpu-profile-file              	| string 	|                                    	|
| memory-profile-file           	| string 	|                                    	|
| etcd-request-timeout-in-milli 	| int    	|                                    	|
| status                        	| object 	|                                    	|
| status.name                   	| string 	|                                    	|
| status.id                     	| int    	|                                    	|
| status.role                   	| string 	|                                    	|
| status.etcdstatus             	| string 	| leader/follower/subscriber/offline 	|
| lastheartbeattime             	| int    	|                                    	|

## HTTPServer
| 监控指标 	| 类型   	| 值可定范围                              	|
|----------	|--------	|-----------------------------------------	|
| state    	| string 	| nil/failed/running/closed               	|
| error    	| string 	| 运行错误原因，当`state`为failed，为非空 	|

## HTTPProxy
| 监控指标             	| 类型   	| 值可定范围                                                  	|
|----------------------	|--------	|-------------------------------------------------------------	|
| tps                  	| int    	| 最小0                                                       	|
| p50                  	| string 	| 时间类型                                                    	|
| p90                  	| string 	| 时间类型                                                    	|
| p95                  	| string 	| 时间类型                                                    	|
| p99                  	| string 	| 时间类型                                                    	|
| rateLimiter          	| object 	| 非空当rateLimiter被启用                                     	|
| rateLimiter.tps      	| int    	| 最小0                                                       	|
| circuitBreaker       	| object 	| 非空当circuitBreaker被启用                                  	|
| circuitBreaker.state 	| string 	| closed/halfOpen/open                                        	|
| *[Bb]ackend          	| object 	| 非空当mirrorBackend/candidateBackend被启动，backend必为非空 	|
| *[Bb]ackend.codes    	| object 	| key为string类型（server url），value为object                	|
| *[Bb]ackend.codes[]  	| object 	| key为int类型（HTTP Code），value为int类型（对应次数）       	|