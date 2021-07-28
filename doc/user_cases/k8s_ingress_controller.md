# Kubernetes Ingress Controller

The IngressController is an implementation of [Kubernetes ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/), it watches Kubernetes Ingress, Service, Endpoints, and Secrets then translates them to Easegress HTTP server and pipelines.

This document list example configurations for typical scenarios, more details could be found at [the guide of ingress controller](./ingresscontroller.md).

## Why Use an Ingress Controller

* **Cuts Down Infrastructure Costs**: Without an ingress controller, when we wanted to expose 20 services to public internet, we have to pay for 20 cloud load balancers; but with an ingress controller, we only need to pay for one cloud load balancer.
* **Scalable Layer 7 Load Balancer**
* **Manage Load Balancer Configuration in a Distributed Fashion**

## Cookbook

### Basic: Handle Ingresses from All K8s namespaces

The ingress controller handles ingresses from all K8s namespaces when `namespaces` is an empty array.

```yaml
kind: IngressController
name: ingress-controller-example
namespaces: []                             # Keep the value an empty array
httpServer:
  port: 8080
  https: false
  keepAlive: true            
  keepAliveTimeout: 60s      
  maxConnections: 10240
```

### Handle Ingresses within Specified K8s Namespaces

When one or more K8s namespaces are listed in `namespaces`, the ingress controller only handles ingresses within these namespaces.

```yaml
kind: IngressController
name: ingress-controller-example
namespaces: ['sales', 'customer']          # List namespaces here
httpServer:
  port: 8080
  https: false
  keepAlive: true            
  keepAliveTimeout: 60s      
  maxConnections: 10240
```

### Use a Customized Ingress Class

By default, the ingress controller handles ingresses with `IngressClassName` set to `easegress`, this can be changed by specifying the `ingressClass` in configuration.

```yaml
kind: IngressController
name: ingress-controller-example
namespaces: []
ingressClass: myingress                    # specify the target ingress class
httpServer:
  port: 8080
  https: false
  keepAlive: true            
  keepAliveTimeout: 60s      
  maxConnections: 10240
```

### Deploy Outside of a K8s Cluster

When deployed outside of a K8s Cluster, we must specify `kubeConfig` or `masterURL` or both of them.

```yaml
kind: IngressController
name: ingress-controller-example
kubeConfig: /home/megaease/.kube/config    # Path of the K8s configuration file
masterURL: http://localhost:8080/api/      # URL of the K8s API server
namespaces: []
httpServer:
  port: 8080
  https: false
  keepAlive: true            
  keepAliveTimeout: 60s      
  maxConnections: 10240
```
