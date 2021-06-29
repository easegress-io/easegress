# Easegress Roadmap

- [Easegress Roadmap](#easegress-roadmap)
  - [Product Principles](#product-principles)
  - [Features](#features)
    - [Business Extensibility](#business-extensibility)
    - [Traffic Orchestration](#traffic-orchestration)
  - [Roadmap 2021](#roadmap-2021)
    - [Business Extensibility](#business-extensibility-1)
    - [Traffic Orchestration](#traffic-orchestration-1)

## Product Principles
1. **Traffic Orchestration**. It must be **traffic-specific**. It tries to solve users’ traffic-based requirements and solutions. Easegress can empower customer business capabilities, e.g., using Easegress to support high concurrent traffic scenarios(such as Flash-Sale, Black Friday, Double 11 event, etc).  And enhancing the APIs orchestration & management.
  
2. **Opening & Extensibility**.  It aims to be an **extensible-development** platform. Users can organize the existing filters into a pipeline, or completely customize a brand-new filter/controller for their specific business logic. With simple, clean, and flat software architecture, anyone can develop their own filter/controller/pipeline rapidly and easily. 
  
3. **Cloud Native**. It is designed to be **cloud-native** compliance. It's scalable, resilient, manageable, and observable. It's easy to integrate with cloud-native architecture, such as Spring Cloud, Service Discovery, Service Gateway, Tracing, Kubernetes, Serverless/FaaS, and so on.

## Features
Based on our product principles, we have made a classification of Easegress' features for powering users' business capabilities into two categories: business-specific and traffic-specific.
### Business Extensibility
* Dynamically load business code written in any language with the lowest performance costing.
* Easy to develop new features with Battery-included Easegress.
* Reusability, such as using Easegress as K8s ingress, Service Gateway, Traffic gateway.
* Easy to operate/easy to install.

### Traffic Orchestration 
* Supporting traffic management -  load balance, rate limiting, etc. 
* Supporting super-high concurrent requests scenario,e.g., Flash-sale, Black Friday, Double 11 event.
* Friendly API orchestration - API aggression, API pipeline 
* Protecting core APIs from high traffic load.
* Supporting canary development and traffic coloring.
* Filtering out invalid traffic for the backend.


## Roadmap 2021
### Business Extensibility

| Name                         | Issue                                                  | Description                                                                                                    |
| ---------------------------- | ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------- |
| WASM runtime embedding       | [#1](https://github.com/megaease/easegress/issues/1)   | Hot-loading customized business logic with WASM.                                                               |
| Operation Enhancement        |                                                        | For better supporting daily cluster operation routine,e.g., one-click installation supported, cluster scaling. |
| Traffic-controller           | [#20](https://github.com/megaease/easegress/issues/20) | Managing `pipeline` and `traffic gate` by Traffic-controller.                                                  |
| Controller/Filter versioning |                                                        | Configuring  `Controller/Filter` with specified versions.                                                      |
| Protobuf models generating   |                                                        | Generating Easegress inner models and related docs with pre-defined Protobuf                                   |



###  Traffic Orchestration 

| Name                          | Issue                                                  | Description                                                                                    |
| ----------------------------- | ------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| Traffic coloring              |                                                        | Supporting coloring ingress traffic by adding a special HTTP header according to users' model. |
| FaaS-controller               | [#22](https://github.com/megaease/easegress/issues/22) | Implementing Knative integrating, function life-cycle management inside a new controller.      |
| More protocol supporting      |                                                        | Such as MQTT, gRPC..                                                                           |
| Kubernetes Ingress controller | [#25](https://github.com/megaease/easegress/issues/25) | Adapting Easegress into a Kubernetes Ingress controller.                                       |
