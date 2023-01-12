# gRRC Service Proxy

- [gRPC Service Proxy](#grpc-service-proxy)
    - [Similar to Service Proxy](#similar-to-service-proxy)
    - [Configure gRPC Filter](#configure gRPC Filter)

Easegress gRPC servers as a forward or reverse proxy base on gRPC protocol. 


## Similar to Service Proxy
Its design and implementation largely refer to the original HTTP Server, so I highly recommended that you read the [Service-Proxy documentation](./service-proxy.md) first.


### Configure Whether to Use Connection Pool

create a gRPC forward proxy filter to handler traffic. if `connectTimeout` be specified, it will block until connection is established when create grpc client connection

``` yaml
filters:
  - kind: GRPCProxy
    pools:
      - loadBalance:
          # Using the forward load balancing strategy 
          policy: forward
          forwardKey: forward
        serviceName: "easegress-forward"
        connectTimeout: 300ms
    name: grpcforwardproxy
flow:
  - filter: grpcforwardproxy
    jumpIf: {}
kind: Pipeline
name: pipeline-grpc-forward
```

Note: Some gRPC servers manage physical connections by themselves, and they do not consider gRPC gateway scenarios, such as Nacos: when the actual client goes offline, the server will detect and actively close the server's Connection. because gRPC is based on HTTP2, it has the characteristics of request and response multiplexing, so there may be a connection from the same easegress used by multiple real clients to the real server. In this case, the server actively closes the connection, which will cause other normal clients to be affected. On the other hand, the server in grpc-go is designed to expect that business processing will not be aware of the underlying connection's status, so it does not provide related APIs for obtaining connection status. I have submitted a related [issue](https://github.com/grpc/grpc-go/issues/5835) in grpc-go to discuss that, you can check it out if you are interested.
