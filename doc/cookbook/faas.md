# FaaS

- [FaaS](#faas)
  - [Background](#background)
  - [Easegress works with FaaS](#easegress-works-with-faas)
  - [Examples](#examples)
    - [Scenario 1: Run a FaaS function beside Easegress](#scenario-1-run-a-faas-function-beside-easegress)
    - [Scenario 2: Limit FaaS function resources using](#scenario-2-limit-faas-function-resources-using)
    - [Scenario 3: Longlife FaaS function](#scenario-3-longlife-faas-function)
    - [Scenario 4: Autoscaling FaaS Function according to rps](#scenario-4-autoscaling-faas-function-according-to-rps)
  - [References](#references)
    - [Resource limiter](#resource-limiter)
    - [Long life function](#long-life-function)
    - [RPS autoscaling](#rps-autoscaling)

## Background

* Function As Services is a recently popular cloud computing solution. It provides a platform for users to develop, run and manage application functionalities without the complexity of building and maintaining the infrastructure.[1]
* Easegress provides a business controller for implementing these zero-infrastructure-maintaining and auto-scaling requirements.

## Easegress works with FaaS

* Isolation: separate Control logic and Business logic
* Traffic originated: Original near traffic, easier to integrate
* Resource saving: reusing Easegress+K8s machine resources.
* Pay what you used: reducing small customize features' developing and maintenance cost.

## Examples


### Scenario 1: Run a FaaS function beside Easegress

* After implementing your business logic and having Knative installed to your Kubernetes cluster. You can refer to [FaaSController](../reference/faascontroller.md) for more info

1. Create a FaaSController[2]

``` bash
echo 'name: faascontroller
kind: FaaSController
provider: knative             # FaaS provider kind, currently we only support Knative
syncInterval: 10s

httpServer:
    http3: false
    port: 10083
    keepAlive: true
    keepAliveTimeout: 60s
    maxConnections: 10240

knative:
   networkLayerURL: http://${knative_kourier_clusterIP}
   hostSuffix: example.com '| egctl object create
```

2. Deploy a function into Easegress and Knative, prepare a YAML content as below:

``` yaml
name:           "demo"
image:          "gcr.io/knative-samples/helloworld-go" # you can change this to any pullable image
port:           8080 # image exposes port 8080
autoScaleType:  "concurrency"
autoScaleValue: "100"
minReplica:     0
maxReplica:     1
limitCPU:       "1000m"
limitMemory:    "1000Mi"
requestCPU:     "80m"
requestMemory:  "20Mi"
requestAdaptor:
  header:
    set:
      X-Func: demo
```

* Save it into `/home/easegress/function.yaml`, using command to deploy it in Easegress:

``` bash
$ curl --data-binary @/home/easegress/function.yaml -X POST -H 'Content-Type: text/vnd.yaml' http://127.0.0.1:2381/apis/v2/faas/faascontroller
```
**Note** this command should be run in Easegress' instance environment and 2381 is the default admin traffic port. If your Easegress instance uses different port, please change 2381 to the correct port.


1. Get the function's status, make sure it is in `active` status

``` bash
$ curl http://127.0.0.1:2381/apis/v2/faas/faascontroller/demo
spec:
  name: demo
  image: gcr.io/knative-samples/helloworld-go
  port: 8080
  autoScaleType: concurrency
  autoScaleValue: "100"
  minReplica: 0
  maxReplica: 1
  limitCPU: 1000m
  limitMemory: 1000Mi
  requestCPU: 80m
  requestMemory: 20Mi
  requestAdaptor:
    host: ""
    method: ""
    header:
      del: []
      set:
        X-Func: demo
      add: {}
    body: ""
    compress: ""
    decompress: ""
status:
  name: demo
  state: active
  event: ready
  extData: {}
fsm: null
```

4. Request the FaaS function by Easegress HTTP traffic gate with `X-FaaS-Func-Name: demo` in the HTTP header.
**Note**: this example's container image `gcr.io/knative-samples/helloworld-go` serves on path `/`. For different function images, the path might differ.

``` bash
$ curl http://127.0.0.1:10083 -H "X-FaaS-Func-Name: demo"
Hello World!
```

### Scenario 2: Limit FaaS function resources using
* You want to make sure at the maximum instance number can only be under 50, and it can only "180m" CPU and "100Mi" memory usage maximum allowed per instance. To providing meaningful resources amount for the function, you also want to make sure one instance has at least a "100m" CPU and "50mi" memory provision. (The CPU and memory limitation usage value comes from Kubernetes resource).

``` yaml
name: demo
#...

limitedMemory: "200Mi"
limitedCPU: "180m"
requireMemory: "100Mi"
requireCPU: "100m"
minReplica: 0
maxReplica: 50

```

* For the full YAML, see [here](#resource-limiter)

* Add the configuration above in #Scenario 1's `/home/easegress/function.yaml`

1. Stop the function execution by using command

``` bash
$ curl http://127.0.0.1:2381/apis/v2/faas/faascontroller/demo/stop -X PUT
```

* The function will become `inactive` then we can update the resource limitation safely.

2. Update the function's spec

``` bash
$ curl --data-binary @/home/easegress/function.yaml -X PUT -H 'Content-Type: text/vnd.yaml' http://127.0.0.1:2381/apis/v2/faas/faascontroller/demo
```

3. Verify the update
* Waiting for the function starts successfully and becomes `active`
* Request the function with step4 in Scenario 1.

### Scenario 3: Longlife FaaS function
* In the same special cases, you may want your FaaS function to have at least one instance running beside Easegress.

``` yaml
name: demo
#...
minReplica:  1
#...
```

* For the full YAML, see [here](#long-life-function)

1. Modifying the `minReplica` above in #Scenario 1's `/home/easegress/function.yaml`

2. Update the function spec and verify it as in Scenario 2's steps 2 - 3.

### Scenario 4: Autoscaling FaaS Function according to rps

* If you don't need to control the function's allowed request precisely, `RPS` based autoscaling is a good choice.

``` yaml
name: demo
#...
autoScaleType:  "rps"
autoScaleValue: "6000"
#...
```

* For the full YAML, see [here](#rps-autoscaling)

1. Modifying the `autoScaleType`  and `autoScaleValue" above in #Scenario 1's `/home/easegress/function.yaml`

2. Update the function spec and verify it as in Scenario 2's step 2 - 3.

## References

1. https://en.wikipedia.org/wiki/Function_as_a_service
2. https://github.com/megaease/easegress/blob/main/doc/faascontroller.md

### Resource limiter

```yaml
name:           demo
image:          "${image_url}"
port:           8089
autoScaleType:  "concurrency"
autoScaleValue: "100
limitedMemory: "200Mi"
limitedCPU:    "180m"
requireMemory: "100Mi"
requireCPU:    "100m"
minReplica:    0
maxReplica:    50
```

### Long life function

``` yaml
name:           demo
image:          "${image_url}"
port:           8089
autoScaleType:  "concurrency"
autoScaleValue: "100"
limitedMemory: "200Mi"
limitedCPU:    "180m"
requireMemory: "100Mi"
requireCPU:    "100m"
minReplica:    1
maxReplica:    50
```

### RPS autoscaling

``` yaml
name:           demo
image:          "${image_url}"
port:           8089
autoScaleType:  "rps"
autoScaleValue: "6000"
limitedMemory: "200Mi"
limitedCPU:    "180m"
requireMemory: "100Mi"
requireCPU:    "100m"
minReplica:    0
maxReplica:    50
```
