# Changelog

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
- Fix double free etcd [\#87](https://github.com/megaease/easegress/pull/87) ([tg123](https://github.com/tg123))


## [v1.0.0](https://github.com/megaease/easegress/tree/v1.0.0) (2021-06-01)
