# 2021 Easegress Roadmap
This is what we hope to accomplish in 2021.

## Goal
* To be an extensible cloud-native traffic-specific platform.

### Breakdowns
1. Easegress works as a traffic orchestration system originally. It must be **traffic-specific**. It is implemented to solve users’ traffic dealing requirements and problems. With EG, users can empower their business capabilities,e.g., using EG to support Flash-sale activity, protecting their core APIs at peak time, and orchestrating their existing APIs for new business logic.
2. Easegress aims to become an **extensible** platform under the traffic-specific domain. Users can choose to orchestrate ingress/egress traffic via existing filters in a pipeline or customize a brand-new filter/controller for their special traffic-specific business logic. With stable, clean, flat software architecture, users can develop their own filter/controller rapidly and easily. 
3. Easegress is designed to be **cloud-native**. It's scalable, resilient, manageable, and observable. During rapid development in the future, these genes should be preserved inside.

## Features
Based on our product goal, we have made a classification of EG's features for powering users' business capabilities into two categories, extensibility and traffic-specific.
### Business features for extensibility
* Dynamically load business code written in any language with the lowest performance costing.
* Easy to develop new features with Battery-included EG.
* Reusability, such as using EG as K8s ingress, Service Gateway, Traffic gateway.
* Easy to operate/easy to install.

### Business features for traffic-specific
* Supporting Faster/safer application delivering.
* Providing Rich traffic-related metrics.
* Flash Sale without changing any code.
* Friendly API orchestration.
* Protecting core APIs under high pressure.
* Filtering out invalid traffic for backend with IP filter, HMAC validating...
* Mocking backend for testing.


## Roadmap 
### Extensibility

| Name                         | Priority | Status    | Description                                                                                                                         |
| ---------------------------- | -------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| WASM runtime embedding       | High     | In design | Supporting users' customized business logic hot-loading by source code with the help of  WASM.                                      |
| Operation Enhancement        | High     | In design | For better supporting daily cluster operation routine,e.g., one-click installation supported, cluster scaling.                      |
| Traffic-controller           | High     | Planning  | Managing `pipeline` and `traffic gate` by this new added controller. Keeping this low-level resource management logic in one place. |
| Controller/Filter versioning | Middle   | Planning  | Providing version in `Controller/Filter`. The user can specified the desired version to use them.                                   |
| Protobuf models generating   | Low      | Planning  | Using `Protobuf` to generate EG inner objects and related docs.                                                                     |



###  Traffic-specific

| Name                          | Priority | Status    | Description                                                                                  |
| ----------------------------- | -------- | --------- | -------------------------------------------------------------------------------------------- |
| Traffic coloring              | High     | In design | Supporting coloring ingress traffic by adding special HTTP header according to users' model. |
| FaaS-controller               | Middle   | Planning  | Implementing Knative integrating, function life-cycle management inside a new controller.    |
| More protocol supporting      | Middle   | Planning  | Such as MQTT, gRPC..                                                                         |
| Kubernetes Ingress controller | Low      | Planning  | Adapting EG into a Kubernetes Ingress controller.                                            |
