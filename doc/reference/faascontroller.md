# FaaSController
- [FaaSController](#faascontroller)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration)
    - [Controller spec](#controller-spec)
    - [FaaSFunction spec](#faasfunction-spec)
    - [Lifecycle](#lifecycle)
    - [RESTful APIs](#restful-apis)
  - [Demoing](#demoing)
  - [Reference](#reference)

* A FaaSController is a business controller for handling Easegress and FaaS products integration purposes.  It abstracts `FaasFunction`, `FaaSStore` and, `FaasProvider`. Currently, we only support `Knative` type `FaaSProvider`. The `FaaSFunction` describes the name, image URL, the resource, and autoscaling type of this FaaS function instance. The `FaaSStore` is covered by Easegress' embed Etcd already.
* FaaSController works closely with local `FaaSProvider`. Please make sure they are running in a communicable environment. Follow this [knative doc](https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/) to install `Knative`[1]'s serving component in K8s. It's better to have Easegress run in the same VM instances with K8s for saving communication costs.


## Prerequisites
1. K8s cluster : **v1.23+**
2. `Knative` Serving : **v1.3+** (with kourier type of network layer)

## Configuration
### Controller spec
* One FaaSController will manage one shared HTTP traffic gate and multiple pipelines according to the functions it has.
* The `httpserver` section in spec is the configuration for the shared HTTP traffic gate.
* The `Knative` section is for `Knative` type of `FaaSProvider`. Depending your Kubernetes cluster, you can use either Magic DNS or Temporary DNS ([see](https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/#configure-dns)). Here's how you fill the `Knative` section for each one of them:
  * **Temporary DNS**: The value of `networkLayerURL` can be found using the following command

  ``` bash
  $ kubectl get svc -n kourier-system
  NAME               TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)                      AGE
  kourier            LoadBalancer   10.109.159.129   <pending>     80:31731/TCP,443:30571/TCP   250dk
  ```

  The `CLUSTER-IP` with value `10.109.159.129` is your kourier's K8s service's address. Use it as the value for `networkLayerURL` in the YAML below.
  * `hostSuffix`'s value should be `example.com` [2], like described in `Knative` serving's `Temporary DNS`.
* **Magic DNS**: For `networkLayerURL`, use the `EXTERNAL-IP` of the loadbalancer:

  ```bash
   $ kubectl get svc -n kourier-system
  NAME               TYPE           CLUSTER-IP       EXTERNAL-IP                                                                    PORT(S)                      AGE
  kourier            LoadBalancer   1.2.3.4   some-external-ip-of-my-cloud-provider.com   80:31060/TCP,443:30384/TCP   12m
  ```
  * For `hostSuffix`, use `kn service list` to see the suffix of your functions: For url `http://demo.default.4.5.6.7.sslip.io` the `hostSuffix` is `4.5.6.7.sslip.io` (basically the IP `x.x.x.x` + `sslip.io`). **Note:** If you don't have any functions yet deployed, you first use any value for `hostSuffix` (for example `example.com`), then deploy a function and use `kn service list` to find out the value of `hostSuffix`. Update it to your configuration and re-create FaaSController.

```yaml
name: faascontroller
kind: FaaSController
provider: knative             # FaaS provider kind, currently we only support Knative

syncInterval: 10s

httpServer:
    http3: false
    port: 10083
    keepAlive: true
    keepAliveTimeout: 60s
    https: false
    certBase64:
    keyBase64:
    maxConnections: 10240

knative:
   networkLayerURL: http://{knative_kourier_clusterIP} # or http://{knative_kourier_externalIP}
   hostSuffix: example.com # or x.x.x.x.sslip.com for Magic DNS
```

### FaaSFunction spec
* The FaaSFunction spec including `name`, `image`, and other resource-related configurations.
* The `image` is the HTTP microservice's image URL. When upgrading the FaaSfFunction's business logic. this field can be helpful.
* The `resource` and `autoscaling` fields are similar to K8s or `Knative`'s resource management configuration.[3]
* The `requestAdaptor` is for customizing the way how HTTP request content will be routed to `Knative`'s `kourier` gateway.

```yaml
name:           "demo10"
image:          "dev.local/colordeploy:17.0"
port:           8089
autoScaleType:  "rps"
autoScaleValue: "111"
minReplica:     1
maxReplica:     3
limitCPU:       "180m"
limitMemory:    "100Mi"
requestCPU:     "80m"
requestMemory:  "20Mi"
requestAdaptor:
  header:
    set:
      X-Func1: func-demo-10              # add one HTTP header
```

### Lifecycle
There four types of function state: Initial, Active, InActive, and Failed[4]. Basically, they come from AWS Lambda's status.

* `Initial`: Once the function has been created in Easegress, its original state is `initial`. After checking FaaSProvider(Knative)'s status successfully, it will become `active` automatically. And the function is ready for handling traffic.

* `Active`: Easegress's FaaSFunction will be `active` not matter there are requests or not.   **Easegress will only route ingress traffic to FaaSProvider when the function is in the `active` state.**

* `Inactive`: Stopping function execution by calling FaaSController's `stop` RESTful API and it will run into `inactive`. Updating function's spec for image URL or other fields, or deleting function also need to stop it first.

* `Failed`: The function will be turned into `failed` states during the runtime checking. If it's about some configuration error, e.g., wrong docker image URL, we can detect this failure by function's `status` message and then update the function's spec by calling RESTful API. If it's about some temporal failure caused by FaaSProvider, the function will turn into the `initial` state after FaaSProvider is recovered.

```
                  Provision

                      │
                      │
                      │
                ┌─────▼──────┐      Start     ┌────────────┐
                │            │     Success    │            │
       ┌────────┤  Initial   ├────────────────►   Active   │
       │        │            │                │            ├──────┐
       │        └───┬───▲────┘                └────┬───▲───┘      │
       │            │   │                          │   │          │
       │            │   │                          │   │          │
       │      Errors│   │ Update             Stop  │   │ Start    │
       │            │   ├───────────┐              │   │Success   │
       │            │   │           │              │   │          │
       │        ┌───▼───┴────┐      │         ┌────▼───┴───┐      │
       │        │            │      └─────────┤            │      │
Delete │        │   Failed   │                │  Inactive  │      │
       │        │            ◄────────────────┤            │      │
       │        └───┬───▲────┘  Start Failed  └──────┬─────┘      │
       │            │   │                            │            │
       │            │   │              Errors        │            │
       │    Delete  │   └────────────────────────────┼────────────┘
       │            │                                │
       │            │                                │
       │        ┌───▼────────┐                       │
       │        │            │          Delete       │
       └────────►  Destory   ◄───────────────────────┘
                │            │
                └────────────┘
```

| Original State | Event                                                                                                                                                                                 | New State |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| initial        | Checking the status in FaaSProvider by FaaSController automatically, and it's ready                                                                                                   | active    |
| initial        | Checking the status in FaaSProvider by FaaSController automatically, and it's has some faults                                                                                         | failed    |
| initial        | Checking the status in FaaSProvider by FaaSController automatically, and it's waiting on all resources to become ready                                                                | initial   |
| initial        | Deleting the function by RESTful API                                                                                                                                                  | destroyed |
| active         | Checking the status of instance in FaaSProvider by FaaSController automatically, and it has some faults                                                                               | failed    |
| active         | Checking the status of instance in FaaSProvider by FaaSController automatically, and for something reason, some resources are missing or pending                                      | failed    |
| active         | Stoping the function by RESTful API                                                                                                                                                   | inactive  |
| active         | Checking the status of instance in FaaSProvider by FaaSController automatically, and it's healthy                                                                                     | active    |
| inactive       | Updating the function by RESTful API                                                                                                                                                  | initial   |
| inactive       | Deleting the function by RESTful API                                                                                                                                                  | destroyed |
| inactive       | Staring the function by RESTful API and after successfully checking the status in FaaSProvider by FaaSController automatically                                                        | active    |
| inactive       | Staring the function by RESTful API but failing at checking status in FaaSProvider by FaaSController automatically                                                                    | failed    |
| inactive       | Staring the function by RESTful API, Checking the status of instance in FaaSProvider by FaaSController automatically, and for something reason, some resources are missing or pending | failed    |
| failed         | Updating the function by RESTful API                                                                                                                                                  | initial   |
| failed         | Deleting the function by RESTful API                                                                                                                                                  | destroyed |
| failed         | Checking the status in FaaSProvider by FaaSController automatically, and it's ready again                                                                                             | initial   |
| failed         | Checking the status of instance in FaaSProvider by FaaSController automatically, and for something reason, some resources are missing or pending                                      | failed    |
| failed         | Checking the status in FaaSProvider by FaaSController automatically, and it has some faults                                                                                           | failed    |


### RESTful APIs
The RESTful API path obey this design `http://host/{version}/{namespace}/{scope}(optional)/{resource}/{action}`,

| Operation         | URL                                                                 | Method | Body          | Description                                                                                                                                 |
| ----------------- | ------------------------------------------------------------------- | ------ | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Create a function | http://eg-host/apis/v2/faas/{controller_name}                       | POST   | function spec | When there is not such a function in Easegress                                                                                              |
| Start a function  | http://eg-host/apis/v2/faas/{controller_name}/{function_name}/start | PUT    | empty         | When function is in `inactive` state only, it will turn-on accepting traffic for this function                                              |
| Stop a function   | http://eg-host/apis/v2/faas/{controller_name}/demo1/stop            | PUT    | empty         | When function is in `active` state only, it will turn-off accpeting traffic for this function                                               |
| Update a function | http://eg-host/apis/v2/faas/{controller_name}/{function_name}       | PUT    | function spec | When function is in `initial`, `inactive` or `failed` state. It can used to update your function or fix your function's deployment problem. |
| Delete a function | http://eg-host/apis/v2/faas/{controller_name}/{function_name}       | DELETE | empty         | When function is in `initial`, `inactive` or `failed` states.                                                                               |
| Get a function    | http://eg-host/apis/v2/faas/{controller_name}/{function_name}       | GET    | empty         | No timing limitation.                                                                                                                       |
| Get function list | http://eg-host/apis/v2/faas/{controller_name}                       | GET    | empty         | No timing limitation.                                                                                                                       |


## Demoing
1. Creating the FaasController in Easegress

```bash
$ cd ./easegress/example/primary-001 && ./start.sh

$ ./egctl.sh object create -f ./faascontroller.yaml

$ ./egctl.sh object get faascontroller
name: faascontroller
kind: FaaSController
provider: knative             # FaaS provider kind, currently we only support Knative

syncInterval: 10s

httpServer:
    http3: false
    port: 10083
    keepAlive: true
    keepAliveTimeout: 60s
    https: false
    certBase64:
    keyBase64:
    maxConnections: 10240

knative:
   networkLayerURL: http://10.109.159.129
   hostSuffix: example.com
```
2. Creating the function

```bash

$ curl --data-binary @./function.yaml -X POST -H 'Content-Type: text/vnd.yaml' http://127.0.0.1:12381/apis/v2/faas/faascontroller
```

3. Waiting for the function provisioned successfully. Confirmed by using `Get` API for checking the `state` field

```bash
$ curl http://127.0.0.1:12381/apis/v2/faas/faascontroller/demo10
spec:
  name: demo10
  image: dev.local/colordeploy:17.0
  port: 8089
  autoScaleType: rps
  autoScaleValue: "111"
  minReplica: 1
  maxReplica: 3
  limitCPU: 180m
  limitMemory: 100Mi
  requestCPU: 80m
  requestMemory: 20Mi
  requestAdaptor:
    host: ""
    method: ""
    header:
      del: []
      set:
        X-Func: func-demo
        X-Func1: func-demo-10
      add: {}
    body: ""
status:
  name: demo10
  state: active
  event: ready
  extData: {}
fsm: null
```

4. Visiting function by HTTP traffic gate with `X-FaaS-Func-Name: demo10` in HTTP header.

```bash
$ curl http://127.0.0.1:10083/tomcat/job/api -H "X-FaaS-Func-Name: demo10" -X POST -d ‘{"megaease":"Hello Easegress+Knative"}’
V3 Body is
‘{megaease:Hello Easegress+Knative}’%

$ curl http://127.0.0.1:10083/tomcat/job/api -H "X-FaaS-Func-Name: demo10" -X POST -d ‘{"FaaS":"Cool"}’
V3 Body is
‘{FaaS:Cool}’%
```
The function's API is serving in `/tomcat/job/api` path and its logic is displaying "V3 body is" with the contents u post.

## Reference
1. knative website http://knative.dev
2. Install knative serving via YAML https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/
3. resource quota https://kubernetes.io/docs/concepts/policy/resource-quotas/
4. AWS Lambda state https://aws.amazon.com/blogs/compute/tracking-the-state-of-lambda-functions/
