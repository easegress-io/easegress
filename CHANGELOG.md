# Changelog

## [v2.7.3](https://github.com/megaease/easegress/tree/v2.7.3) (2024-03-26) 

[Full Changelog](https://github.com/megaease/easegress/compare/v2.7.2...v2.7.3) 

**Implemented enhancements:**
* Kafka filter supports synchronous producer and message key.
* host and port Functions added for template of request-response builder. 
* Proxy support set upstream host for outgoing requests.

## [v2.7.2](https://github.com/megaease/easegress/tree/v2.7.2) (2024-03-11)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.7.1...v2.7.2)

**Community works**
* Replace Github related megaease links to easegress-io

**Implemented enhancements:**
* Support global filter fallthrough when meet error

**Fixed bugs:**
* Fix empty proxy-timeout failure


## [v2.7.1](https://github.com/megaease/easegress/tree/v2.7.1) (2024-02-22)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.7.0...v2.7.1)

**Implemented enhancements:**
* Upgraded to Golang version 1.21.
* Enabled `egctl create httpproxy` command to support automatic updates for `AutoCertManager`.
* Introduced an exportable custom plain logger.
* Expanded `egctl` command utility across various namespaces.
* Optimized proxy filter to immediately flush responses for Server-Sent Events (SSE).
* Enhanced template functionality with additional functions in `RequestBuilder` and `ResponseBuilder`.
* Updated documentation and adhered to the latest CNCF Code of Conduct for community guidelines.

**Fixed bugs:**
* Resolved the `AutoCertManager` did not remove duplicate DNS records issue.
* Corrected proxy filter behavior to properly escape paths.


## [v2.7.0](https://github.com/megaease/easegress/tree/v2.7.0) (2023-12-29)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.6.4...v2.7.0)

**Significant changes:**
* Support updating system controllers.

**Implemented enhancements:**
* Enabled unlimited single message capability for WebsocketProxy.
* Add a hook in AutoCertManager for singular instance management.

**Fixed bugs:**
* Resolved WebsocketProxy keepHost issue.
* Corrected the object status prefix in some special cases.
* Fixed creation error in egbuilder filter.

## [v2.6.4](https://github.com/megaease/easegress/tree/v2.6.4) (2023-12-04)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.6.3...v2.6.4)

**Significant changes:**
* Support health check for Proxy and WebSocketProxy filter.

## [v2.6.3](https://github.com/megaease/easegress/tree/v2.6.3) (2023-11-23)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.6.2...v2.6.3)

**Significant changes:**
* Support Kubernetes Gateway API v1.
* Converts Nginx configurations into Easegress YAMLs.
* Integrates all Easegress filters and resilience policies with Kubernetes Gateway API.

**Implemented enhancements:**
* New RedirectorV2 filter added.
* Runtime log level adjustment enabled.
* Updated omitempty jsonschema in Specs.
* Introduced cookie hash support.

## [v2.6.2](https://github.com/megaease/easegress/tree/v2.6.2) (2023-10-19)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.6.1...v2.6.2)

**Significant changes:**
* Introduced a new user-friendly document.

**Implemented enhancements:**
* Added support for additional annotations in Kubernetes ingress.
* Enabled CORS support in the easegress-server APIs.
* Updated the helm chart.

## [v2.6.1](https://github.com/megaease/easegress/tree/v2.6.1) (2023-09-01)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.6.0...v2.6.1)

**Significant changes:**
* Improved egctl with new commands: 'egctl logs' and 'egctl create httpproxy'.



**Implemented enhancements:**
* Extended Kafka headers with additional MQTT information.
* Config easegress-server using environment variables.
* Restricted to a single running object file.

## [v2.6.0](https://github.com/megaease/easegress/tree/v2.6.0) (2023-08-16)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.5.3...v2.6.0)

**Significant changes:**
* Introduced egbuilder to build Easegress with custom plugins.
* Added support for Kubernetes Gateway API.

**Implemented enhancements:**
* Rewrite integration tests with egctl commands.

## [v2.5.3](https://github.com/megaease/easegress/tree/v2.5.3) (2023-08-02)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.5.0...v2.5.3)

**Implemented enhancements:**
* Add documentation for the GRPCServer.
* Update the redirect policy for the Proxy filter.
* Correct typos and optimize the code.

## [v2.5.0](https://github.com/megaease/easegress/tree/v2.5.0) (2023-07-07)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.4.3...v2.5.0)

**Significant changes:**
* Enhanced the command line tool egctl.

**Implemented enhancements:**
* Introduced new WebSocket options to the ingress controller.
* Expanded multiplexing rules to support multiple hosts.

## [v2.4.3](https://github.com/megaease/easegress/tree/v2.4.3) (2023-06-30)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.4.2...v2.4.3)

* Websocket supports cross origin requests (#1021)

## [v2.4.2](https://github.com/megaease/easegress/tree/v2.4.2) (2023-06-08)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.4.1...v2.4.2)

**Implemented enhancements:**
* GRPCProxy use connection pool to manage connections (#977).
* Enhance SimpleHTTPProxy to support forward proxy (#1016).

**Fixed bugs:**
* Websocket does not work (#998).
* Fix typo and other problems in the documentation (#999).

## [v2.4.1](https://github.com/megaease/easegress/tree/v2.4.1) (2023-04-21)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.4.0...v2.4.1)

**Implemented enhancements:**
* Add a new filter: Simple HTTP Proxy.
* Add a Canary Release cookbook.
* Add a ChatGPT Bot cookbook.

**Fixed bugs:**
* Fix the Sampler's incorrect result bug
* Fix OIDCAdaptor bug.
* Fix typo and other minor problems in the documentation.

## [v2.4.0](https://github.com/megaease/easegress/tree/v2.4.0) (2023-03-03)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.3.1...v2.4.0)

**Significant changes:**
* Support GRPC protocol.

**Implemented enhancements:**
* Enhancement to access log.
* Enlarge charset allowed in object names.
* Enhance tracing to support Cloudflare.
* RequestAdaptor and ResponseAdaptor support template.

**Fixed bugs:**
* WASM apply data command not working.
* install.sh not work as expectation on some platforms.
* Fix typo and other minor problems in the documentation.


## [v2.3.1](https://github.com/megaease/easegress/tree/v2.3.1) (2022-12-28)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.3.0...v2.3.1)

**Significant changes:**
* Support OpenTelemetry for tracing.
* Add new filter Redirector to handle HTTP 3xx redirects.
* Add prometheus support for Proxy and HTTPServer.

**Implemented enhancements:**
* Add LDAP mode for basic authentication.
* Add Broker mode for MQTTProxy.
* Support HTTPS for egctl and easegress-server.
* Enhance egctl delete.

## [v2.3.0](https://github.com/megaease/easegress/tree/v2.3.0) (2022-11-28)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.2.0...v2.3.0)

**Significant changes:**
* HTTPServer: support radix tree router and routing by client IP.
* Add new filter OPAFilter for Open Policy Agent.

**Implemented enhancements:**
* Load Balance: support session and server health check.
* Making running objects dump interval configurable.
* Simplify CORSAdaptor.

**Fixed bugs:**
* Fix unexpected EOF when compression is enabled.
* Remove duplicated code in codectool.
* Update keyfunc for compatibility and bug fixes.
* Fix mqtt test random fail.


## [v2.2.0](https://github.com/megaease/easegress/tree/v2.2.0) (2022-10-18)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.1.1...v2.2.0)

**Significant changes:**
* Add new filter WebSocketProxy and remove controller WebSocketServer.
* Add new filter OIDCAdaptor for OpenID Connect 1.0 authorization.
* Add new filter DataBuilder.

**Implemented enhancements:**
* Support routing by query string.
* Update wasmtime-go.
* Add more signing algorithm for JWT validator.
* Support websocket for ingress controller.

**Fixed bugs:**
* Fix cluster test random fail.
* Fix EaseMonitorMetrics status error.
* Fix GlobalFilter jsonschema validator error.

## [v2.1.1](https://github.com/megaease/easegress/tree/v2.1.1) (2022-09-09)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.1.0...v2.1.1)


**Implemented enhancements:**

* Add ResultBuilder filter
* Speed up DNS01 challenge
* Add EaseTranslateBot example (code-free workflow by using Easegress pipeline)
* Speed up MQTTProxy client connection
* Check Kubernetes version for ingress controller

**Fixed bugs:**
* Fix default logger bug
* Fix HTTP runtime ticker bug
* Fix ingress translation error


## [v2.1.0](https://github.com/megaease/easegress/tree/v2.1.0) (2022-08-09)

[Full Changelog](https://github.com/megaease/easegress/compare/v2.0.0...v2.1.0)

**Significant changes:**

* Define user data in pipeline spec.
* `jumpIf` support jumping on an empty result.
* Bump API version to v2 (v1 APIs are kept for compatibility).

**Implemented enhancements:**

* RequestAdaptor support signing the request (experimental).
* RequestBuilder support form data (HTTP only).
* Add `disableReport` option to tracing.
* Logs go to stdout/stderr by default.

**Fixed bugs:**

* HTTPServer does not work as expected when match all header is enabled.
* `jumpIf` not working with global filter.
* Response headers set by filters before a Proxy are all lost.
* Nacos registry doesn't work.
* Panic in StatusSyncController.
* MQTT client is not closed as expected.

## [v2.0.0](https://github.com/megaease/easegress/tree/v2.0.0) (2022-07-07)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.5.3...v2.0.0)

**Significant changes:**

- Pipeline
  * Pipeline is protocol-independent now.
  * Add `RequestBuilder` filter, `ResponseBuilder` filter and built-in filter `END`.
  * Add the support of `namespace`.
  * Add the support of filter alias.
  * Filter `Retryer`, `CircuitBreaker` and `TimeLimiter` are removed, resilience
    policies are now defined on pipeline and injected into filters that support
    resilience.
  * Filter `APIAggregator` is removed.

**Implemented enhancements:**
- Tracing is now using Zipkin B3 format.
- Cluster
  * Drop the support of dynamic cluster management.
  * Depreciated configuration method is removed.

## [v1.5.3](https://github.com/megaease/easegress/tree/v1.5.3) (2022-06-28)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.5.2...v1.5.3)

**Implemented enhancements:**
- Remove HTTP hop headers [\#650](https://github.com/megaease/easegress/pull/650)
- Optimize Kubernetes IngressController rule route policy [\#651](https://github.com/megaease/easegress/pull/651)

**Fixed bugs:**
- Wrong HTTP request scheme if the `X-Forwarded-Proto` header contains two or more items[\#634](https://github.com/megaease/easegress/pull/634)
- Fix request "Content-Length" header missing bug [\#649](https://github.com/megaease/easegress/pull/649)

## [v1.5.2](https://github.com/megaease/easegress/tree/v1.5.2) (2022-05-10)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.5.1...v1.5.2)

**Significant changes:**

- Support external standalone etcd [\#595](https://github.com/megaease/easegress/pull/595)

**Implemented enhancements:**

- Support Easegress ingress rewrite target [\#617](https://github.com/megaease/easegress/pull/617)
- Support the AND-OR header strategy in HTTPServer [\#613](https://github.com/megaease/easegress/pull/613)
- Support proxy to send zipkin b3 headers [\#579](https://github.com/megaease/easegress/pull/579)

**Fixed bugs:**
- Fix proxy pool bug [\#614](https://github.com/megaease/easegress/pull/614)
- Fix FaasController request host field [\#586](https://github.com/megaease/easegress/pull/586)
- Fix Easemonitor metrics status convert error [\#583](https://github.com/megaease/easegress/pull/583)
- Fix Status sync controller pointer error [\#582](https://github.com/megaease/easegress/pull/582)
- Fix HTTPPipeline creation fail [\#577](https://github.com/megaease/easegress/pull/577)



## [v1.5.1](https://github.com/megaease/easegress/tree/v1.5.1) (2022-04-06)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.5.0...v1.5.1)

**Significant changes:**

- Turn profiling on/off runtime [\#543](https://github.com/megaease/easegress/pull/543)

**Implemented enhancements:**

- Change the way StatusSyncController stores statuses to reduce memory usage [\#542](https://github.com/megaease/easegress/pull/542)
- Support custom image name [\#545](https://github.com/megaease/easegress/pull/545)
- HTTPServer prefix and fullpath support rewrite_target [\#553](https://github.com/megaease/easegress/pull/553)
- Refactor the Docker entrypoint.sh [\#569](https://github.com/megaease/easegress/pull/569)

**Fixed bugs:**

- Fix proxy fallback [\#537](https://github.com/megaease/easegress/pull/537)
- Resolve inconsistent path selection [\#536](https://github.com/megaease/easegress/pull/536)
- Validate CircuitBreaker filter [\#551](https://github.com/megaease/easegress/pull/551)



## [v1.5.0](https://github.com/megaease/easegress/tree/v1.5.0) (2022-03-03)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.4.1...v1.5.0)

**Significant changes:**

- HTTP basic auth filter [\#454](https://github.com/megaease/easegress/pull/454)
- HeaderLookup filter [\#454](https://github.com/megaease/easegress/pull/454)
- HeaderToJSON filter [\#458](https://github.com/megaease/easegress/pull/458)
- CertExtractor filter [\#474](https://github.com/megaease/easegress/pull/474)
- Custom data management [\#456](https://github.com/megaease/easegress/pull/456), [\#500](https://github.com/megaease/easegress/pull/500)

  **NOTE:** The dynamic cluster management feature (e.g. adding a new primary node without stopping the cluster) is deprecated and will be removed in the next release, please switch to static cluster management. You can refer to this cookbook [chapter](https://github.com/megaease/easegress/blob/main/doc/cookbook/multi-node-cluster.md) or follow [Helm example](https://github.com/megaease/easegress/tree/main/helm-charts/easegress) for more info on how to define a static cluster.

**Implemented enhancements:**

- Add pipeline route for mqttproxy [\#453](https://github.com/megaease/easegress/pull/453)
- Install script to install the systemd service [\#461](https://github.com/megaease/easegress/pull/461), [\#463](https://github.com/megaease/easegress/pull/463)
- RequestAdaptor support compress/decompress request body [\#497](https://github.com/megaease/easegress/pull/497)
- CorsAdaptor support origin [\#498](https://github.com/megaease/easegress/pull/498)
- Filter out 'TLS handshake error' [\#533](https://github.com/megaease/easegress/pull/533)
- Test [\#469](https://github.com/megaease/easegress/pull/469), [\#507](https://github.com/megaease/easegress/pull/507)
- Documentation [\#464](https://github.com/megaease/easegress/pull/464), [\#465](https://github.com/megaease/easegress/pull/465), [\#475](https://github.com/megaease/easegress/pull/475), [\#499](https://github.com/megaease/easegress/pull/499)

**Fixed bugs:**

- Fix empty http request body read panic [\#457](https://github.com/megaease/easegress/pull/457)
- Fix paging in query nacos service registry [\#478](https://github.com/megaease/easegress/pull/478)
- Fix duplicate response header [\#482](https://github.com/megaease/easegress/pull/482)
- Add close option to httpRequest.SetBody [\#502](https://github.com/megaease/easegress/pull/502)
- Fix wrong service name in nacos service registry [\#504](https://github.com/megaease/easegress/pull/504)
- Remove method & path from httpRequest [\#524](https://github.com/megaease/easegress/pull/524)
- Make cluster.Mutex to be goroutine same [\#527](https://github.com/megaease/easegress/pull/527)
- Fix empty request scheme [\#529](https://github.com/megaease/easegress/pull/529)
- Fix typo [\#471](https://github.com/megaease/easegress/pull/471), [\#479](https://github.com/megaease/easegress/pull/479),


## [v1.4.1](https://github.com/megaease/easegress/tree/v1.4.1) (2022-01-07)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.4.0...v1.4.1)

**Implemented enhancements:**

- Improve performance of Proxy filter [\#414](https://github.com/megaease/easegress/pull/414)
- Ingress Controller tutorial and Helm Charts to support multi nodes [\#395](https://github.com/megaease/easegress/pull/395)

**Fixed bugs:**

- Reduce etcd server memory usage [\#439](https://github.com/megaease/easegress/pull/439)
- Support PEM in both base64 and plain text [\#425](https://github.com/megaease/easegress/pull/425)
- Only set host when server addr is IP (close #447) [\#451](https://github.com/megaease/easegress/pull/451)
- Copy backend http response header to httpresponse [\#449](https://github.com/megaease/easegress/pull/449)
- Fix httpserver httppipeline status not show error [\#441](https://github.com/megaease/easegress/pull/441)
- Mock filter to support header match [\#409](https://github.com/megaease/easegress/pull/409)
- Fix wrong content-type [\#430](https://github.com/megaease/easegress/pull/430)
- Fix easegress-server yaml validation [\#432](https://github.com/megaease/easegress/pull/432)
- Change status code 403 to 401 when jwt signer and oauth2 validation fail [\#426](https://github.com/megaease/easegress/pull/426)
- Fix typos [\#423](https://github.com/megaease/easegress/pull/423) [\#417](https://github.com/megaease/easegress/pull/417) [\#416](https://github.com/megaease/easegress/pull/416) [\#410](https://github.com/megaease/easegress/pull/410)
- Expose max-sync-message-size in options [\#419](https://github.com/megaease/easegress/pull/419)
- Enable calling mirrorPool reader Read after EOF [\#411](https://github.com/megaease/easegress/pull/411)
- Use ClusterJoinURLs for secondary members [\#403](https://github.com/megaease/easegress/pull/403)

## [v1.4.0](https://github.com/megaease/easegress/tree/v1.4.0) (2021-12-07)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.3.1...v1.4.0)


**Significant changes:**

- **Support automated certificate management with ACME (Let's Encrypt)** [\#391](https://github.com/megaease/easegress/pull/391)
- **Cluster configuration made simpler** [\#382](https://github.com/megaease/easegress/pull/382)
- **WebAssembly SDK for Golang** [#370](https://github.com/megaease/easegress/pull/370)


**Implemented enhancements:**

- **New feature `GlobalFilters` to HTTPPipeline**[\#376](https://github.com/megaease/easegress/pull/376)
- **Linux kernel tuning guide**[\#361](https://github.com/megaease/easegress/pull/361)
- **WebSocket proxy user guide**[\#385](https://github.com/megaease/easegress/pull/385)
- **Cluster deployment guide**[\#369](https://github.com/megaease/easegress/pull/369)


**Fixed bugs:**

- **Add more tracing information**[\#316](https://github.com/megaease/easegress/issues/316)
- **Improve robustness of WebSocket proxy**[\#379](https://github.com/megaease/easegress/issues/379)


## [v1.3.2](https://github.com/megaease/easegress/tree/v1.3.2) (2021-11-08)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.3.1...v1.3.2)

**Implemented enhancements:**

- **Performance improvement**[\#277](https://github.com/megaease/easegress/pull/277)

**Fixed bugs:**

- **Wrong host function name in WasmHost**[\#317](https://github.com/megaease/easegress/pull/317)
- **Wrong name in WebSocket spec**[\#328](https://github.com/megaease/easegress/pull/328)
- **Response body corrupt after pass ResponseAdapter**[\#345](https://github.com/megaease/easegress/pull/345)
- **Failed to create pipeline when filters are not sorted as flow**[\#348](https://github.com/megaease/easegress/pull/348)

## [v1.3.1](https://github.com/megaease/easegress/tree/v1.3.1) (2021-10-20)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.3.0...v1.3.1)

**Implemented enhancements:**

- **Improve docker image build speed**[\#306](https://github.com/megaease/easegress/pull/306)
- **Add doc for MQTT proxy**[\#300](https://github.com/megaease/easegress/pull/300)
- **MQTT proxy performance optimization**[\#304](https://github.com/megaease/easegress/pull/304)

**Fixed bugs:**

- **Fix MQTT closed client related bugs**[\#302](https://github.com/megaease/easegress/pull/302)
- **Change logger level from Error to Warn**[\#296](https://github.com/megaease/easegress/pull/307)
- **Fix Eureka port number**[\#309](https://github.com/megaease/easegress/pull/309)

## [v1.3.0](https://github.com/megaease/easegress/tree/v1.3.0) (2021-10-13)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.2.1...v1.3.0)

**Significant changes:**

- **MQTT support** [\#203](https://github.com/megaease/easegress/pull/203)

**Implemented enhancements:**

- **Add chart files for helm-install** [\#254](https://github.com/megaease/easegress/pull/254)
- **Add license checker** [\#247](https://github.com/megaease/easegress/pull/247)
- **Improve egctl to support sending multi configs at once** [\#230](https://github.com/megaease/easegress/pull/230)
- **Create objects from spec files at startup** [\#202](https://github.com/megaease/easegress/pull/202)
- **Support bi-directional service registry controllers** [#171](https://github.com/megaease/easegress/pull/171)

**Fixed bugs:**

- **Fix typo**
- **Make `docker run` with default config without parameter** [\#248](https://github.com/megaease/easegress/pull/248)
- **Fix random failure of jmx test cases** [\#290](https://github.com/megaease/easegress/pull/290)
- **Correctly handle empty object name** [\#289](https://github.com/megaease/easegress/pull/289)
- **Update doc/developer-guide.md** [\#250](https://github.com/megaease/easegress/pull/250)
- **Update cluster example, no std-log-level in option** [\#278](https://github.com/megaease/easegress/pull/278)
- **Add readme in cn** [\#267](https://github.com/megaease/easegress/pull/267)
- **Correctly update restfulapi in ctl** [\#249](https://github.com/megaease/easegress/pull/249)
- **Update func match() in pkg/object/httpserver/mux.go** [\#245](https://github.com/megaease/easegress/pull/245)
- **Refine the flash sale document** [\#243](https://github.com/megaease/easegress/pull/243)
- **Fix lint warning** [\#242](https://github.com/megaease/easegress/pull/242)
- **Fix wrong link for ingress controller guide** [\#232](https://github.com/megaease/easegress/pull/232)
- **Update ingresscontroller\.md to fix permission issue** [#227](https://github.com/megaease/easegress/pull/227)
- **Fix test fail caused by random order of map iteration** [\#224](https://github.com/megaease/easegress/pull/224)
- **Add host functions for cluster data (flash sale support)** [\#188](https://github.com/megaease/easegress/pull/188)
- **Avoid send duplicated metrics data** [\#217](https://github.com/megaease/easegress/pull/217)

## [v1.2.1](https://github.com/megaease/easegress/tree/v1.2.1) (2021-09-08)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.2.0...v1.2.1)

**Significant changes:**

- **Add documentation of user cases** [\#155](https://github.com/megaease/easegress/pull/155)
- **Add UnitTest and raise coverage rate** [\#170](https://github.com/megaease/easegress/pull/170)

**Implemented enhancements:**

- Trafficcontroller copy mutex by pointer [\#211](https://github.com/megaease/easegress/pull/211)
- Move limitlistener to util for other protocol proxy use[\#210](https://github.com/megaease/easegress/pull/210)
- Skip invalid ip cidr parse result [\#184](https://github.com/megaease/easegress/pull/184)

**Fixed bugs:**

- Fix interface nil convert panic [\#164](https://github.com/megaease/easegress/pull/164)

## [v1.2.0](https://github.com/megaease/easegress/tree/v1.2.0) (2021-08-23)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.1.0...v1.2.0)

**Significant changes:**

- **WASM AssemblyScript SDK** [SDK](https://github.com/megaease/easegress-assemblyscript-sdk)
- **Windows Supporting** [\#74](https://github.com/megaease/easegress/pull/74)
- **Websockt Proxy**  [\#99](https://github.com/megaease/easegress/issues/99)

**Implemented enhancements:**

- Add user-guide for WasmHost filter [\#138](https://github.com/megaease/easegress/pull/138)
- Refactor APIAggregator filter [\#153](https://github.com/megaease/easegress/pull/153)
- Add PR deployment testing [\#130](https://github.com/megaease/easegress/pull/130)
- Format and correct code according to golint [\#124](https://github.com/megaease/easegress/pull/124)[\#131](https://github.com/megaease/easegress/pull/131)

**Fixed bugs:**

- Fix http template dictory empty error [\#163](https://github.com/megaease/easegress/pull/163)
- Fix initTrafficGate panic when service not found [\#144](https://github.com/megaease/easegress/pull/144)

## [v1.1.0](https://github.com/megaease/easegress/tree/v1.1.0) (2021-07-16)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.0.1...v1.1.0)

**Significant changes:**

- **Kubernetes Ingress Controller** [\#25](https://github.com/megaease/easegress/issues/25)
- **WASM**:The new plugin mechanism for the Easegress [\#1](https://github.com/megaease/easegress/issues/1)(**Still working on AssemblyScript SDK, coming soon...**)

**Implemented enhancements:**

- CI: Add codecov into github action [\#121](https://github.com/megaease/easegress/pull/121)
- Mesh: convert service instance API to `pb/json` [\#109](https://github.com/megaease/easegress/pull/109)
- Support dynamic admin API && correct syncer && make interface cleaner [\#96](https://github.com/megaease/easegress/pull/96)
- CI: check unnecessary dependencies [\#77](https://github.com/megaease/easegress/pull/77)

**Fixed bugs:**

- Github Action failed due to the unit test of TestCluster  [\#111](https://github.com/megaease/easegress/issues/111)
- Fix unit test failure of semaphore and cluster [\#110](https://github.com/megaease/easegress/pull/110)
- Use default spec correctly & fix httppipeline.Validate without recursively checking [\#100](https://github.com/megaease/easegress/pull/100)

## [v1.0.1](https://github.com/megaease/easegress/tree/v1.0.1) (2021-06-29)

[Full Changelog](https://github.com/megaease/easegress/compare/v1.0.0...v1.0.1)

**Significant changes:**

- Use traffic controller to manage TrafficGate and Pipeline [\#20](https://github.com/megaease/easegress/issues/20)
- FaaSController [\#59](https://github.com/megaease/easegress/pull/59)
- Upgrade MeshController to use TrafficController [\#79](https://github.com/megaease/easegress/pull/79)
- Replace the Iris with Go-Chi Framework [\#24](https://github.com/megaease/easegress/issues/24)

**Implemented enhancements:**

- Golang/protobuf warning [\#36](https://github.com/megaease/easegress/issues/36)
- Support an HTTPServer bind multiple certs [\#31](https://github.com/megaease/easegress/issues/31)
- HTTP `Host` header can't be written properly into headers for backend request [\#27](https://github.com/megaease/easegress/issues/27)
- Support strip trailing slash [\#85](https://github.com/megaease/easegress/pull/85)
- Add multiple certs support, close \#31 [\#48](https://github.com/megaease/easegress/pull/48)
- Docker: add tzdata package for latest alpine [\#45](https://github.com/megaease/easegress/pull/45)
- Deps: use dependabot to update dependencies [\#43](https://github.com/megaease/easegress/pull/43)

**Fixed bugs:**

- Possible regression introduce by FaaSController  [\#73](https://github.com/megaease/easegress/issues/73)
- HTTPPipeline loadBalance is required but causes panic if missing [\#63](https://github.com/megaease/easegress/issues/63)
- Server help messages print multiple times [\#37](https://github.com/megaease/easegress/issues/37)
- Fix double free etcd [\#87](https://github.com/megaease/easegress/pull/87)

## [v1.0.0](https://github.com/megaease/easegress/tree/v1.0.0) (2021-06-01)
