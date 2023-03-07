# Canary Release With Cloudflare

- [Canary Release With Cloudflare](#Canary-Release-With-Cloudflare)
  - [Scenario 1](#scenario-1)
    - [Configure Cloudflare](#configure-cloudflare)
    - [Configure Easegress](#configure-easegress)
  - [Scenario 2](#scenario-2)
    - [Implementation through Cloudflare's Worker](#implementation-through-cloudflares-worker)
      - [Deploy Worker](#deploy-worker)
      - [Configure Easegress](#configure-easegress-1)
    - [Implementation through Easegress's WASM](#implementation-through-easegresss-wasm)
      - [Deploy WASM](#deploy-wasm)
      - [Configure Easegress](#configure-easegress-2)

## Scenario 1

Upgrade the service `/old-demo` to `/new-demo`. After the upgrade, according to geographic location, 
the traffic from Beijing and 20% of non-Beijing locations should be directed to `/new-demo`. 
The geographic location judgment is based on the `CF-IPCity` field in Cloudflare's HTTP header.

### Configure Cloudflare

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

### Configure Easegress

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

## Scenario 2
Upgrade the service /old-demo to /new-demo. After the upgrade, the requests from Mac devices should be directed to /new-demo. 
Easegress provides the [user-agent parser](https://github.com/megaease/easegress-rust-uaparser), which can parse user devices and os information.

### Implementation through Cloudflare's Worker

#### Deploy Worker

1. Download `easegress-rust-uaparser`

```bash
git clone https://github.com/megaease/easegress-rust-uaparser.git
```

2. Enter the `easegress-rust-uaparser/cloudflare` directory and deploy the worker using the wrangler command

```bash
npx wrangler publish
```

3. Enter the Cloudflare website configuration page and configure Workers Routes as you needed.

#### Configure Easegress

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

### Implementation through Easegress's WASM

#### Deploy WASM

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

#### Configure Easegress

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
