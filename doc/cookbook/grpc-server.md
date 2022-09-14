# gRRC Service Proxy

- [gRPC Service Proxy](#grpc-service-proxy)
    - [Similar to Service Proxy](#similar-to-service-proxy)
    - [Configure Whether to Use Connection Pool](#configure whether to use connection pool)
       - [When Not to Use Connection Pool](#When-not-to-use-connection-pool)

Easegress gRPC servers as a forward or reverse proxy base on gRPC protocol. 


## Similar to Service Proxy
Its design and implementation largely refer to the original HTTP Server, so it is strongly recommended that you read 
the [Service-Proxy documentation](./service-proxy.md) first.


### Configure Whether to Use Connection Pool

create a gRPC forward proxy filter to handler traffic

``` yaml
filters:
  - kind: GRPCProxy
    port: 8081
    useConnectionPool: true
    initConnNum: 2
    maxConnsPerHost: 1024
    connectTimeout: 200ms
    borrowTimeout: 1000ms
    pools:
      - loadBalance:
          #使用正向代理负载均衡策略
          policy: forward
          forwardKey: forward
        serviceName: "easegress-forward"
    name: grpcforwardproxy
flow:
  - filter: grpcforwardproxy
    jumpIf: {}
kind: Pipeline
name: pipeline-grpc-forward
```

If you don't want to use connection pool , then set `useConnectionPool` to false or not specify. by the way,
parameters such as `initConnNum`,`maxConnsPerHost`,`connectTimeout`,`borrowTimeout` are valid when `useConnectionPool` is
true, otherwise, these parameters would be ignored.

### When Not to Use Connection Pool
Some gRPC servers manage physical connections by themselves, and they are not designed with gRPC gateway scenarios, such 
as Nacos: when the actual client goes offline, the server will detect and actively close the server's Connection. 
because gRPC is based on HTT2, it has the characteristics of request and response multiplexing, so there may be a 
connection from the same easegress used by multiple real clients to the real server.In this case, the server actively 
closes the connection, which will cause other normal clients to be affected.
