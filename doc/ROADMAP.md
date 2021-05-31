# 2021 Easegress Roadmap
This is what we hope to accomplish in 2021.

## Goal
* To be an extensible cloud-native traffic-specific platform.

### Breakdowns
1. Easegress works as a traffic orchestration system originally. It must be **traffic-specific**. It is implemented to solve users’ traffic dealing requirements and problems. With Easegress, users can empower their business capabilities,e.g., using Easegress to support Flash-sale activity, protecting their core APIs at peak time, and orchestrating their existing APIs for new business logic.
2. Easegress aims to become an **extensible** platform under the traffic-specific domain. Users can choose to orchestrate ingress/egress traffic via existing filters in a pipeline or customize a brand-new filter/controller for their special traffic-specific business logic. With stable, clean, flat software architecture, users can develop their own filter/controller rapidly and easily. 
3. Easegress is designed to be **cloud-native**. It's scalable, resilient, manageable, and observable. These genes should be kept in it for the rapid future development process. 

## Features
Based on our product goal, we have made a classification of Easegress's features for powering users' business capabilities into two categories, extensibility and traffic-specific.
### Business features for extensibility
* Dynamically load business code written in any language with the lowest performance costing.
* Easy to develop new features with Battery-included Easegress.
* Reusability, such as using Easegress as K8s ingress, Service Gateway, Traffic gateway.
* Easy to operate/easy to install.

### Business features for traffic-specific
* Supporting faster/safer application delivering.
* Providing rich traffic-related metrics.
* Supporting super high concurrent requests scenario,e.g., Flash-sale, Black Friday, Double 11 event.
* Friendly API orchestration.
* Protecting users' core APIs under high traffic pressure.
* Filtering out invalid traffic for backend with IP filter, HMAC validating...
* Mocking backend for testing.


## Roadmap 
### Extensibility

| Name                         | Issue                                                | Description                                                                                                    |
| ---------------------------- | ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| WASM runtime embedding       | [#1](https://github.com/megaease/easegress/issues/1) | Hot-loading customized business logic with WASM.                                                               |
| Operation Enhancement        |                                                      | For better supporting daily cluster operation routine,e.g., one-click installation supported, cluster scaling. |
| Traffic-controller           |                                                      | Managing `pipeline` and `traffic gate` by Traffic-controller.                                                  |
| Controller/Filter versioning |                                                      | Configuring  `Controller/Filter` with specified versions.                                                      |
| Protobuf models generating   |                                                      | Generating Easegress inner models and related docs with pre-defined Protobuf                                   |



###  Traffic-specific

| Name                          | Issue | Description                                                                                  |
| ----------------------------- | ----- | -------------------------------------------------------------------------------------------- |
| Traffic coloring              |       | Supporting coloring ingress traffic by adding special HTTP header according to users' model. |
| FaaS-controller               |       | Implementing Knative integrating, function life-cycle management inside a new controller.    |
| More protocol supporting      |       | Such as MQTT, gRPC..                                                                         |
| Kubernetes Ingress controller |       | Adapting Easegress into a Kubernetes Ingress controller.                                     |
