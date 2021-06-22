# FaaSController 

* A FaaSController is a business controller for handling Easegress and FaaS products integration purpose.  It abstracts `FaasFunction`, `FaaSStore` and, `FaaSProvder`. Currently, we only support `Knative` type FaaSProvider. The `FaaSFunction` describes the name, image URL, the resource and autoscaling type of this FaaS function instance. The `FaaSStore` is covered by Easegress' embed ETCD already. 
* FaaSController works closely with local FaaSProvider. Please make sure the they are running int the communicable environment. Flow this [doc](https://knative.dev/docs/install/install-serving-with-yaml/) to install Knative[1]'s serving component in K8s. It's betweer to have Easegress run in the same vm-instances with K8s for saving communication cost.

## Prerequest 
1. K8s : **v1.18+**
2. Knative Serving : **v0.23**

## Controller spec 
One FaaSController will manage on HTTP traffic gate and multiples pipeline automatically inside. The `httpserver` section is the configuration for the shared HTTP traffic gate. The `Knative` section is for `Knative` type FaaSProvider. 

```yaml
name: faascontroller
kind: FaaSController           
provider: knative             # FaaS provider kind, currently we only support Knative

syncInterval: 10s              

httpserver:
    http3: false               
    port: 10083                
    keepAlive: true            
    keepAliveTimeout: 60s      
    https: false
    certBase64:
    keyBase64:
    maxConnections: 10240      

knative:
   networkLayerURL: http://${knative_kourier_clusterIP}
   hostSuffix: ${knative_kourier_clusterIP}.xip.io  
```

## FaaSFunction spec
The FaaSFunction spec including `name`, `image` and other resource related configrations. The `image` is the HTTP microservice's image URL. When upgrading the FaaSfFunction's business logic. this filed can be helpful. Other fields are simaller with K8s or Knative's configuration.[2] The `requestAdaprot` is for customizing the HTTP request content routed from FaaSController's HTTP traffic gate to Knative's `kourier` gateway. 

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
There four types of function status, Pending, Active, InActive, and Failed[2]. Basically, they come from AWS Lambda's status. 
* Once the function has been created in Easegress, its original status is `pending`. After the function had been provisioned successfully by FaaSProvider(Knative), its status will become `active`. At last, FaaSController will add the routing rule in its HTTPServer for this function.  
* Easegress's FaaSFunction will be `active` not matter there are requests or not. Stop function execution by calling FaaSController's `stop` RESTful API, then it will turn function into `inactive`.  Updating function's spec for image URL or else fields, or deleting function also need to stop it first.
* **Easegress will only route use's ingress traffic to FaaSProvider when function is in `active` state.**
* After updating the function, it will run into `pending` status. (Once provision successfully, it will become `active` automatically) 
* FaaSController will remove the routing rule in its HTTPServer for rejecting one function's ingress traffic when it's in `inactive` status. (Then the client will receive HTTP 404 failure response. For becoming zero-downtime, deploy another new FaaSfunction in this FaaSController may be helpful) 

```                                                      
                                                         +-------------------+
                                                         | provision succ    |
  +---------------+   provision succ              +------+------+<-----------+
  |               |------------------------------>|             |
  |    pending    |             provision pending |   active    |<------+
  |               | <-----------------------------|             |       |
+-+---------------+ <--+     +--------------------+--------+----+       |   
|                      |     |                             |            |
provision faild        |     |                             |stop        | start
|                  update    provision failed              |            |
|  +---------------+   |     |        +-------------+      |            |
+->|               |   |     |        |             |      |            |
   |   failed      |<--+-----+--------+   inactive  |<-----+            |
   |               |                  |             +-------------------+
   |               |                  |             |
   +---------------+                  +-------------+
```

There are seven types of event: `UpdateEvent, DeleteEvent, StopEvent, StartEvent, ProvisionFailedEvent, ProvisionPendingEvent, ProvisionOKEvent`.
* Provision type's events are trigged by FaaSProvider. 
* Update/Start/Stop/Delete/Create are trigged by users.

### RESTful APIs
The RESTful API path obey this design `http://host/{version}/{namespace}/{scope}(optional)/{resource}/{action}`,

| Operation         | URL                                                                   | Method | Body          | Description                                                                                                                                 |
| ----------------- | --------------------------------------------------------------------- | ------ | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Create a function | http://eg-host/apis/v1/faas/${controller_name}                        | POST   | function spec | When there is not such a function in Easegress                                                                                              |
| Start a function  | http://eg-host/apis/v1/faas/${controller_name}/${function_name}/start | PUT    | empty         | When function is in `inactive` state only, it will turn-off accepting traffic for this function                                             |
| Stop a function   | http://eg-host/apis/v1/faas/${controller_name}/demo1/stop             | PUT    | empty         | When function is in `active` state only, it will turn-on accpeting traffic for this function                                                |
| Update a function | http://eg-host/apis/v1/faas/${controller_name}/${function_name}       | PUT    | function spec | When function is in `pending`, `inactive` or `failed` state. It can used to update your function or fix your function's deployment problem. |
| Delete a function | http://eg-host/apis/v1/faas/${controller_name}/${function_name}       | DELETE | empty         | When function is in `pending` or `failed` states.                                                                                           |
| Get a function    | http://eg-host/apis/v1/faas/${controller_name}/${function_name}       | GET    | empty         | No timing limitation.                                                                                                                       |
| Get function list | http://eg-host/apis/v1/faas/${controller_name}                        | GET    | empty         | No timing limitation.                                                                                                                       |


## Demoing 
1. Create the faasController in Easegress
```bash
$ cd ./easegress/example/writer-001 && ./start.sh

$ ./egctl.sh object create -f ./faascontroller.yaml

$ ./egctl.sh object get faascontroller
name: faascontroller
kind: FaaSController 
provider: knative             # FaaS provider kind, currently we only support Knative

syncInterval: 10s

httpserver:
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
   hostSuffix: 10.109.159.129.xip.io  
```
2. Create the function 
```bash

$ curl --data-binary @./function.yaml -X POST -H 'Content-Type: text/vnd.yaml' http://127.0.0.1:12381/apis/v1/faas/faascontroller
```

3. Waiting function be provision successfully. Comfired by using `Get` API for checking the `state` field 
```bash
$ curl http://127.0.0.1:12381/apis/v1/faas/faascontroller/demo10
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
  event: provisionsOK
  extData: {}
fsm: null
```

4. Visit function by HTTP traffic gate with `X-FaaS-Func-Name: demo10` in HTTP header.  
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
[1] knative website http://knative.dev
[2] resource quota https://kubernetes.io/docs/concepts/policy/resource-quotas/