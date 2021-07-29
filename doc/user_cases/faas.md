- [Background](#background)
- [Easegress works with FaaS](#easegress-works-with-faas)
- [Examples](#examples)
  - [Scenario 1: Run a FaaS function beside Easegress](#scenario-1-run-a-faas-function-beside-easegress)
  - [Scenario 2: Limit FaaS function resources using](#scenario-2-limit-faas-function-resources-using)
  - [Scenario 3: Longlife FaaS function](#scenario-3-longlife-faas-function)
  - [Scenario 4: Autoscaling FaaS Function according to rps](#scenario-4-autoscaling-faas-function-according-to-rps)
- [References](#references)

## Background
* Function As Services is a recently popular cloud computing solution. It provides a platform for customer to develop, run and manage application functionalities without the complexity of building and maintaining the infrastructure.[1]
* Easegress provides a business controller for implemetating this zero-infrastructure-mantaining and auto-scaling requirements.

## Easegress works with FaaS
* Original near traffic, easier to integrate 
* Reusing machine resources. 
* Reducing small customize features' maintenance.

## Examples
### Scenario 1: Run a FaaS function beside Easegress
After implementing your business 
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
image:          "${image_url}"    
port:           8089
autoScaleType:  "concurrency"          
autoScaleValue: "100" 
```
Save it into `/home/easegress/function.yaml`, using command to deploy it in Easegress:
**Note** this command should be run in Easegress' instance environment and ${eg_apiport} should be replace with the real working port, ${image_url} should be replaced with pullable image URL. 

``` bash
curl --data-binary @/home/easegress/function.yaml -X POST -H 'Content-Type: text/vnd.yaml' http://127.0.0.1:${eg_apiport}/apis/v1/faas/faascontroller
```
3. Get the function's status, make sure it is in `active` status
``` bash
$ curl http://127.0.0.1:${eg_apiport}/apis/v1/faas/faascontroller/demo
spec:
  name: demo
  image: dev.local/demo:1.0
  port: 8089
  autoScaleType: rps
  autoScaleValue: "1000"
  minReplica: 0 
  maxReplica: 0  
  limitCPU:  
  limitMemory: 
  requestCPU: 
  requestMemory: 
  requestAdaptor:
    host: ""
    method: ""
    header:
      del: {}
      set: {}
      add: {}
    body: ""
status:
  name: demo10
  state: active
  event: ready 
  extData: {}
fsm: null
```

4. Request the faas function by Easegress HTTP traffic gate with `X-FaaS-Func-Name: demo` in HTTP header.
**Note**: this example HTTP backend's API works on `/tomcat/job/api` path and its business logic is echoing back your request body and with `V3 Body is ` content.

``` bash
$ curl http://127.0.0.1:10083/tomcat/job/api -H "X-FaaS-Func-Name: demo" -X POST -d ‘{"megaease":"Hello Easegress+Knative"}’ 
V3 Body is 
‘{megaease:Hello Easegress+Knative}’% 
```


### Scenario 2: Limit FaaS function resources using  
You want to make sure at the maximum instance number can only under 50, and it can only "180m" CPU and "100Mi" memory useage maximum allowed per instance. In order to providing meaningful resources amount for the function, you also want to make sure one instacne has at least "100m" CPU and "50mi" memory provision. (The CPU and memory limitation usage value comes form Kubernetes resource). 

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

Add the configuration above in #Scenario 1's `/home/easegress/function.yaml` 

1. Stop the function execution by using command 

``` bash
curl http://127.0.0.1:${eg_apiport}/apis/v1/faas/faascontroller/demo/stop -X PUT
```

The function will become `inactive` then we can update the resource limitation safely.

2. Update the function's spec 

``` bash
curl --data-binary @/home/easegress/function.yaml -X PUT -H 'Content-Type: text/vnd.yaml' http://127.0.0.1:${eg_apiport}/apis/v1/faas/faascontroller/demo
```

3. Verify the update 
* Waiting for the function starts successfully and becomes `active`
* Request the function with step4 in Scenario 1.

### Scenario 3: Longlife FaaS function 
In same special cases, you may want your FaaS function has at least one instance running beside Easegress.  

``` yaml
name: demo
#...
minReplica:  1  
#...
```

1. Modifying the `minReplica` above in #Scenario 1's `/home/easegress/function.yaml` 

2. Update the function spec and verify it as in Scenario 2's step 2 - 3.

### Scenario 4: Autoscaling FaaS Function according to rps 
If your don't need to controll the function's allowed request precisely, `RPS` based autoscaling is a good choice. 

``` yaml
name:demo
#...
autoScaleType:  "rps"          
autoScaleValue: "6000" 
#...
```

1. Modifying the `autoScaleType`  and `autoScaleValue" above in #Scenario 1's `/home/easegress/function.yaml` 

2. Update the function spec and verify it as in Scenario 2's step 2 - 3.

## References
1. https://en.wikipedia.org/wiki/Function_as_a_service
2. https://github.com/megaease/easegress/blob/main/doc/faascontroller.md
