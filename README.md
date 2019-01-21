# EaseGateway [![Build Status](https://travis-ci.com/megaease/easegateway.svg?token=bgrenfvQpzZ8JoKbi6uX&branch=master)](https://travis-ci.com/megaease/easegateway) [![codecov](https://codecov.io/gh/megaease/easegateway/branch/master/graph/badge.svg?token=HAR3ZmYoQG)](https://codecov.io/gh/megaease/easegateway)

# Introduction

`EaseGateway` is a generic gateway system, support standalone and cluster both running modes.
For more design details, please see [easegateway design](./doc/easegateway_design.md)

## Build

### Requirements

- Go version >= 1.11 which needs go module.

### Make

```shell
make
make vendor_from_mod # Copy source code from module to traditional vendor.
```

### Run

Makefile generates binaray `easegateway-server` and `egwctl` at `bin/`.

### Security

You can use [generate_cert.go](https://golang.org/src/crypto/tls/generate_cert.go) to generate certificate files for testing.

```shell
    go run generate_cert.go -ca -host localhost,127.0.0.1 -start-date 'Jan 1 15:04:05 2016' -duration 87600h
```
