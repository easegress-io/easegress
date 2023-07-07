# Kubernetes Ingress Controller

- [Kubernetes Ingress Controller](#kubernetes-ingress-controller)
  - [Why Use an Ingress Controller](#why-use-an-ingress-controller)
  - [Cookbook](#cookbook)
    - [Basic: Handle Ingresses from All K8s namespaces](#basic-handle-ingresses-from-all-k8s-namespaces)
    - [Handle Ingresses within Specified K8s Namespaces](#handle-ingresses-within-specified-k8s-namespaces)
    - [Use a Customized Ingress Class](#use-a-customized-ingress-class)
    - [Deploy Outside of a K8s Cluster](#deploy-outside-of-a-k8s-cluster)

The IngressController is an implementation of [Kubernetes ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/), it watches Kubernetes Ingress, Service, Endpoints, and Secrets then translates them to Easegress HTTP server and pipelines.

This document list example configurations for typical scenarios, more details could be found at [the guide of ingress controller](../reference/ingresscontroller.md).

## Why Use an Ingress Controller

- **Cuts Down Infrastructure Costs**: Without an ingress controller, when we wanted to expose 20 services to the public internet, we have to pay for 20 cloud load balancers; but with an ingress controller, we only need to pay for one cloud load balancer.
- **Scalable Layer 7 Load Balancer**
- **Manage Load Balancer Configuration in a Distributed Fashion**

## Cookbook

### Basic: Handle Ingresses from All K8s namespaces

Create an ingress controller, it handles ingresses from all K8s namespaces when `namespaces` is an empty array.

```bash
echo '
kind: IngressController
name: ingress-controller-example
namespaces: []                             # Keep the value an empty array
httpServer:
  port: 8080
  https: false
  keepAlive: true
  keepAliveTimeout: 60s
  maxConnections: 10240
' | egctl create -f -
```

Create two versions of `hello` service in Kubernetes:

```bash
echo '
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-deployment
spec:
  selector:
    matchLabels:
      app: products
      department: sales
  replicas: 2
  template:
    metadata:
      labels:
        app: products
        department: sales
    spec:
      containers:
      - name: hello-v1
        image: "us-docker.pkg.dev/google-samples/containers/gke/hello-app:1.0"
        env:
        - name: "PORT"
          value: "50001"
      - name: hello-v2
        image: "us-docker.pkg.dev/google-samples/containers/gke/hello-app:2.0"
        env:
        - name: "PORT"
          value: "50002"

---
apiVersion: v1
kind: Service
metadata:
  name: hello-service
spec:
  type: NodePort
  selector:
    app: products
    department: sales
  ports:
  - name: port-v1
    protocol: TCP
    port: 60001
    targetPort: 50001
  - name: port-v2
    protocol: TCP
    port: 60002
    targetPort: 50002
' | kubectl apply
```

Create a Kubernetes ingress for the two services, note the `ingressClassName` is `easegress`:

```bash
echo '
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-example
spec:
  ingressClassName: easegress
  rules:
  - host: "www.example.com"
    http:
      paths:
      - pathType: Prefix
        path: /
        backend:
          service:
            name: hello-service
            port:
              number: 60001
  - host: "*.megaease.com"
    http:
      paths:
      - pathType: Prefix
        path: /
        backend:
          service:
            name: hello-service
            port:
              number: 60002
' | kubectl apply
```

After a while, we can leverage the below command to access both versions of the `hello` application:

```bash
$ curl http://{NODE_IP}/ -HHost:www.megaease.com
Hello, world!
Version: 2.0.0
Hostname: hello-deployment-6cbf765985-r6242

$ curl http://{NODE_IP}/ -HHost:www.example.com
Hello, world!
Version: 1.0.0
Hostname: hello-deployment-6cbf765985-r6242
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
