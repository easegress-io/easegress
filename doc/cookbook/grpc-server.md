# gRRC Service Proxy

- [gRPC Service Proxy](#grpc-service-proxy)
    - [Similar to Service Proxy](#similar-to-service-proxy)
    - [Configure gRPC Filter](#configure gRPC Filter)

Easegress gRPC servers as a forward or reverse proxy base on gRPC protocol. 


## Similar to Service Proxy
Its design and implementation largely refer to the HTTP Server, so I highly recommended that you read the [Service-Proxy documentation](./service-proxy.md) first.

We can create a simple forward gRPC proxy server according to the following configuration.

``` yaml
kind: GRPCServer
maxConnections: 2048
maxConnectionIdle: 60s
name: server-grpc
#路由规则
rules:
  - paths:
      #指向pipeline的name，表明符合此规则的请求由对应的pipeline处理
      - backend: pipeline-grpc-forward
        #表示请求头中应包含x-masa-forward-port且值满足正则表达式
        headers:
          - key: "targetAddress"
            regexp: "\\d{4,5}"
#是否为代理，true的话，会在X-Forwarded-For中追加客户端的真实IP
xForwardedFor: true
```
