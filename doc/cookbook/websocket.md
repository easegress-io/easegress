# WebSocket

- [WebSocket](#websocket)
  - [Background](#background)
  - [Design](#design)
  - [References](#references)

## Background

- Reverse proxy is one of most popular features of Easegress and it's suitable for many production and development scenarios.

- In reversed proxy use cases, `WebSocket` is a widely used protocol for a full-duplex communication solution between client and server. WebSocket relies on TCP.[1]

- Many reverse proxy support `WebSocket`,e.g., NGINX[2], Traefik, and so on.

## Design

- WebSocket server of Easegress is called `WebSocketServer` and it belongs to the group of BusinessControllers.

- Easegress uses  `github.com/gorilla/websocket` to implement `WebSocket` client since it has rich features supported and a quite active community (15k star/2.5k fork). Gorilla supports some useful features (`receive fragmented message` and `send close message`) that do not exist in Go's standard WebSocket library `golang.org/x/net/websocket` [3], which make it a natural choice for Easegress WebSocketServer.

1. Spec

    Example1: `http` and `ws`

    ```yaml
    kind: WebSocketServer
    name: websocketSvr
    https: false                  # client need to use http/https firstly for connection upgrade      
    certBase64:
    keyBase64:
    port: 10020                   # proxy servers listening port

    backend: ws://localhost:3001  # the reserved proxy target
                                  #  Easegress will exame the backend URL's scheme, If it starts with `wss`,
                                  #  then `wssCerBase64` and `wssKeyBase64` must not be empty

    wssCertBase64:                # wss backend certificate in base64 format
    wssKeyBase64:                 # wss backend key in base64 format
    ```

    Example2: `https` and `wss`

    ```yaml
    kind: WebSocketServer
    name: websocketSvr
    https: true     
    certBase64: your-cert-base64
    keyBase64: your-key-base64
    port: 10020
    backend: wss://localhost:3001
    wssCertBase64: your-cert-wss-base64
    wssKeyBase64: your-key-wss-base64
    ```

2. Request sequence

    ```none
    +--------------+                +--------------+                +--------------+  
    |              |  1             |              |   2            |              | 
    |   client     +--------------->|  Easegress   +--------------->|  websocket   |
    |              |<---------------+(WebSocketSvr)|<---------------+  backend     | 
    |              |  4             |              |   3            |              |
    +--------------+                +--------------+                +--------------+
    ```

3. Headers

    We copy all headers from your HTTP request to websocket backend, except ones used by `gorilla` package to build connection. Based on [4], we also add `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto` to http headers that send to websocket backend.

    > note: `gorilla` use `Upgrade`, `Connection`, `Sec-Websocket-Key`, `Sec-Websocket-Version`, `Sec-Websocket-Extensions` and `Sec-Websocket-Protocol` in http headers to set connection.

4. Demos

    1. Create a WebSocket proxy for Easegress: `egctl object create -f websocket.yaml`. Here we use `Example1` as example, which will transfer requests from `easegress-ip:10020` to `ws://localhost:3001`.

    2. Send request

        ```bash
        curl --include \
            --no-buffer \
            --header "Connection: Upgrade" \
            --header "Upgrade: websocket" \
            --header "Host: 127.0.0.1:10020" \
            --header "Sec-WebSocket-Key: your-key-here" \
            --header "Sec-WebSocket-Version: 13" \
            http://127.0.0.1:10081/
        ```

    3. You send request to `WebSocketServer` and it will transfer it to true backend `ws://localhost:3001`.

## References

1. <https://datatracker.ietf.org/doc/html/rfc6455>
2. <https://www.nginx.com/blog/websocket-nginx/>
3. <https://github.com/gorilla/websocket>
4. <https://docs.oracle.com/en-us/iaas/Content/Balance/Reference/httpheaders.htm>
