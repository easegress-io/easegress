# Changelog

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
