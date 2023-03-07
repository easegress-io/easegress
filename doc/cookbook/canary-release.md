# Canary Release With Cloudflare

- [Canary Release With Cloudflare](#Canary-Release-With-Cloudflare)
  - [Introduction to Canary Release](#Introduction-to-Canary-Release)
  - [Key Points of Canary Release](#Key-Points-of-Canary-Release)
  - [How to implement canary release with Easegress and Cloudflare](#How-to-implement-canary-release-with-Easegress-and-Cloudflare)
    - [Scenario 1: Implementing Cancary Release based on geographic location](#Implementing-Cancary-Release-based-on-geographic-location)
      - [Add location header to HTTP request through Cloudflare's Transform](#add-location-header-to-http-request-through-cloudflares-transform)
      - [Config traffic scheduling through Easegress](#config-traffic-scheduling-through-easegress)
    - [Scenario 2: Implementing Cancary Release based on user devices](#Implementing-Cancary-Release-based-on-user-devices)
      - [Implementation through Cloudflare's Worker](#implementation-through-cloudflares-worker)
        - [Deploy Worker](#deploy-worker)
        - [Config traffic scheduling through Easegress](#config-traffic-scheduling-through-easegress-1)
      - [Implementation through Easegress's WASM](#implementation-through-easegresss-wasm)
        - [Deploy WASM](#deploy-wasm)
        - [Config traffic scheduling through Easegress](#config-traffic-scheduling-through-easegress-1)

## Introduction to Canary Release

Canary release is a software deployment technique that allows for a gradual and controlled rollout of new features or updates to a small subset of users or servers before
being released to the entire user base. The name "canary" comes from the use of canaries in coal mines to detect poisonous gases. In the same way, canary releases are used
to detect petential problems or bugs in new code before it reaches a wider audience.

In a canary release, a small percentage of users or servers are selected to receive the new code, while the majority of users or servers continue to use the previous version.
This allows for real-world testing and monitoring of the new code in a controlled environment, and any issues or bugs can be detected and addressed before rolling out the new code to the entire user base.

Canary releases are often used in large-scale web applications, where a bug or downtime can have significant consequences. By testing new features on a small scale before releasing them to a wider audience,
the risk of negative impact is reduced, and users can enjoy a smoother, more stable experience.

## Key points of Canary Release

To achieve canary release, we need to consider the following points:

* Traffic scheduling: Traffic scheduling is a network management technique used to direct network traffic to different servers based on a set of rules.
* Traffic tagging: Traffic tagging is a network management technique used to add tags or identifiers to network packets to identify and differentiate different types of traffic. 
It allows network administrators to have more granular control over network traffic, including prioritization and restriction.

## How to implement canary release with Easegress and Cloudflare

* [Cloudflare](https://www.cloudflare.com/) is a content delivery network (CDN) and DNS provider. It provides a variety of services, including DNS, CDN, DDoS protection, and security.
Cloudflare's [Transform](https://developers.cloudflare.com/rules/transform/) feature allows you to modify HTTP requests and responses. It can be used to implement Canary Release.
* Easegress provides a variety of traffic scheduling and tagging features, including header filter and header hash. It can be used to implement Canary Release.

Let's take a look at how to implement canary release with Easegress and Cloudflare. Following are two scenarios:

### Implementing Cancary Release based on geographic location

Upgrade the service `/old-demo` to `/new-demo`. After the upgrade, according to geographic location, 
the traffic from Beijing and 20% of non-Beijing locations should be directed to `/new-demo`. 
The geographic location judgment is based on the `CF-IPCity` field in Cloudflare's HTTP header.

#### Add location header to HTTP request through Cloudflare's Transform

Cloudflare's Transform feature can be enabled through either of the following two methods:

* Cloudflare Dashboard -> Rules -> Transform Rules -> Managed Transforms -> [enabled “Add visitor location headers”](https://developers.cloudflare.com/rules/transform/managed-transforms/configure/)
* Call [API](https://developers.cloudflare.com/api/operations/managed-transforms-update-status-of-managed-transforms)

```bash
curl --request PATCH \
--url https://api.cloudflare.com/client/v4/zones/<zone-id>/managed_headers \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer 1234' \
--data '{
  "managed_request_headers": [
    {
      "enabled": true,
      "id": "add_visitor_location_headers"
    }
  ]
}'
```

#### Config traffic scheduling through Easegress

```yaml
kind: Proxy
name: canaryrelease-example
pools:
- servers:
  - url: http://<old-demo>
- filter:
    headers:
      cf-ipcity:
        exact: Beijing
  servers:
  - url: http://new-demo
- filter:
    permil: 200
    policy: headerHash
    headerHashKey: cf-ipcity
  servers:
  - url: http://<new-demo>
```

### Implementing Cancary Release based on user devices

Upgrade the service /old-demo to /new-demo. After the upgrade, the requests from Mac devices should be directed to /new-demo. 
Easegress provides the [user-agent parser](https://github.com/megaease/easegress-rust-uaparser), which can parse user devices and os information.

We have two ways to get the user device information: One is to use Cloudflare's Worker, and the other is to use Easegress's WASM.

#### Implementation through Cloudflare's Worker

##### Deploy Worker

1. Download `easegress-rust-uaparser`

```bash
git clone https://github.com/megaease/easegress-rust-uaparser.git
```

2. Enter the `easegress-rust-uaparser/cloudflare` directory and deploy the worker using the wrangler command

```bash
npx wrangler publish
```

3. Enter the Cloudflare website configuration page and configure Workers Routes as you needed.

##### Configure traffic scheduling through Easegress

* Create HTTPServer

```yaml
kind: HTTPServer
name: http-server-cloudflare
port: 80
keepAlive: true
https: false
rules:
- paths:
  - pathPrefix: /cloudflare
    backend: cloudflare-pipeline
```

* Create Pipeline

```yaml
name: cloudflare-pipeline
kind: Pipeline
flow:
- filter: proxy-demo

filters:
- name: proxy-demo
  kind: Proxy
  pools:
  - servers:
    - url: http://<old-demo>
  - filter:
      headers:
        x-ua-device:
          exact: Mac
    servers:
    - url: http://<new-demo>
```

#### Implementation through Easegress's WASM

##### Deploy WASM

1. Download `easegress-rust-uaparser`

```bash
git clone https://github.com/megaease/easegress-rust-uaparser.git
```

2. Navigate to the `easegress-rust-uaparser/binary` directory and use the `easegress.wasm` file directly, 
or If you want to customize the parser's behavior, you can modify `regexes.yaml` and then compile it by yourself:

  - Navigate to the `easegress-rust-uaparser/easegress directory` and compile easegress.wasm:

     ```bash
    cargo build --release --target wasm32-unknown-unknown
    ```

##### Config traffic scheduling through Easegress

* Create HTTPServer

```yaml
kind: HTTPServer
name: http-server-cloudflare
port: 80
keepAlive: true
https: false
rules:
- paths:
  - pathPrefix: /easegress
    backend: easegress-pipeline
```

* Create Pipeline

```yaml
name: easegress-pipeline
kind: Pipeline
flow:
- filter: wasm
- filter: proxy-demo

filters:
- name: wasm
  kind: WasmHost
  maxConcurrency: 2
  code: <easegress.wasm path>
  timeout: 100ms
- name: proxy-demo
  kind: Proxy
  pools:
  - servers:
    - url: http://<old-demo>
  - filter:
      headers:
        x-ua-device:
          exact: Mac
    servers:
    - url: http://<new-demo>
```
