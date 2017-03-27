# Introduction

`easegateway` aims to hide kafka from public network by changing https to kafka protocol.

# Build

```shell
make build
```

# Run
```shell
sudo make run
```

# Security
You can use [generate_cert.go](https://golang.org/src/crypto/tls/generate_cert.go) to generate certificate files.

```shell
    go run generate_cert.go -ca -host localhost,127.0.0.1 -start-date 'Jan 1 15:04:05 2016' -duration 87600h
```
