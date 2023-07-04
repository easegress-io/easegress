# gRRC Service Proxy

- [gRPC Service Proxy](#grpc-service-proxy)
    - [Simple Proxy Configuration](#simple-proxy-configuration)
    - [Configuration](#configuration)

Easegress gRPC servers is a proxy server base on gRPC protocol.

## Simple Proxy Configuration
Below is one of the simplest gRPC Proxy Server, and it will listen on port `8080` and handle the requests that includes kv `content-type:application/grpc` in headers by use backend `pipeline-grpc-forward`
``` yaml
kind: GRPCServer
port: 8080
name: server-grpc
#路由规则
rules:
  - methods:
      - backend: pipeline-grpc
        headers:
          - key: "Content-Type"
            values: 
             - "application/grpc"
xForwardedFor: true
```

## Configuration
The below parameters will help manage connections better

| Name | Type | Description | Required |
|------|------|-------------|----------|
| maxConnections | uint32 | The maximum number of connections allowed by gRPC Server , default value 10240, min is 1 | No |
| minTimeClientSendPing | duration | The minimum amount of time a client should wait before sending a keepalive ping, default value is 5 minutes | No |
| permitClintSendPingWithoutStream | duration | If true, server allows keepalive pings even when there are no active streams(RPCs). If false, and client sends ping when there are no active streams, server will send GOAWAY and close the connection. default false | No |
| maxConnectionIdle | duration | A duration for the amount of time after which an idle connection would be closed by sending a GoAway. Idleness duration is defined since the most recent time the number of outstanding RPCs became zero or the connection establishment. default value is infinity | No |
| maxConnectionAge | duration | A duration for the maximum amount of time a connection may exist before it will be closed by sending a GoAway. A random jitter of +/-10% will be added to MaxConnectionAge to spread out connection storms. default value is infinity | No |
| maxConnectionAgeGrace | duration | An additive period after MaxConnectionAge after which the connection will be forcibly closed. default value is infinity | No |
| keepaliveTime | duration | After a duration of this time if the server doesn't see any activity it pings the client to see if the transport is still alive. If set below 1s, a minimum value of 1s will be used instead. default value is 2 hours. | No |
| keepaliveTimeout | duration | After having pinged for keepalive check, the server waits for a duration of Timeout and if no activity is seen even after that the connection is closed. default value is 20 seconds |No |
