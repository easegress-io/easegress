# IngressController 


The IngressController is an implementation of [Kubernetes ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/), it watches Kubernetes Ingress, Service, Endpoints, and Secrets then translates them to Easegress HTTP server and pipelines.

## Prerequisites
1. K8s cluster : **v1.18+**

## Configuration

### Controller spec

```yaml
kind: IngressController
name: ingress-controller-example
kubeConfig:
masterURL:
namespaces: ["default"]
ingressClass: easegress
httpServer:
  port: 8080
  https: false
  keepAlive: true            
  keepAliveTimeout: 60s      
  maxConnections: 10240
```
* IngressController uses `kubeConfig` and `masterURL` to connect to Kubernetes, at least one of them must be specified when deployed outside of a Kubernetes cluster, and both are optional when deployed inside a cluster.
* The `namespaces` is an array of Kubernetes namespaces which the IngressController needs to watch, all namespaces are watched if left empty.
* IngressController only handles `Ingresses` with `ingressClassName` set to `ingressClass`, the default value of `ingressClass` is `easegress`.
* One IngressController manages a shared HTTP traffic gate and multiple pipelines according to the Kubernetes ingress. The `httpServer` section in the spec is the basic configuration for the shared HTTP traffic gate. The routing part of the HTTP server and pipeline configurations will be generated dynamically according to Kubernetes ingresses.

## Getting Started

### Role Based Access Control configuration

If your cluster is configured with RBAC, you will need to authorize Easegress IngressController for using the Kubernetes API firstly. Below is an example configuration:

```yaml
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: easegress-ingress-controller
rules:
- apiGroups: [""] # "" indicates the core API group
  resources: ["services", "endpoints", "secrets"]
  verbs: ["get", "watch", "list"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "watch", "list"]

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: easegress-ingress-controller
  namespace: default

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: easegress-ingress-controller
subjects:
- kind: ServiceAccount
  name: easegress-ingress-controller
  namespace: default
roleRef:
  kind: ClusterRole
  name: easegress-ingress-controller
  apiGroup: rbac.authorization.k8s.io
```

Note the name of the ServiceAccount we just created is `easegress-ingress-controller`, it will be used later.

### Deploy Easegress IngressController

To deploy the IngressController, we will create a Deployment and a Service as below:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    name: easegress-ingress
  name: easegress-ingress
spec:
  replicas: 1
  selector:
    matchLabels:
      app: easegress-ingress
  template:
    metadata:
      labels:
        app: easegress-ingress
    spec:
      serviceAccountName: easegress-ingress-controller
      containers:
      - command:
        - /bin/sh
        - -c
        - |-
          echo name: $POD_NAME > /easegress-ingress/config.yaml && echo '
          cluster-request-timeout: 10s
          cluster-role: writer
          api-addr: 0.0.0.0:2381
          cluster-name: easegress-ingress-controller
          ' >> /easegress-ingress/config.yaml && echo '
          kind: IngressController
          name: ingress-controller-example
          kubeConfig:
          masterURL:
          namespaces: ["default"]
          ingressClass: easegress
          httpServer:
            port: 8080
            https: false
            keepAlive: true            
            keepAliveTimeout: 60s      
            maxConnections: 10240
          ' >> /easegress-ingress/controller.yaml && /opt/easegress/bin/easegress-server -f /easegress-ingress/config.yaml --initial-object-config-files /easegress-ingress/controller.yaml
        image: megaease/easegress:latest
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        name: easegress-ingress
        volumeMounts:
        - mountPath: /easegress-ingress
          name: ingress-params-volume
      volumes:
      - emptyDir: {}
        name: ingress-params-volume

---
kind: Service
apiVersion: v1
metadata:
  name: easegress-ingress-service
  namespace: default
spec:
  selector:
    app: easegress-ingress
  ports:
    - name: web
      protocol: TCP
      port: 8080
      nodePort: 80
  type: NodePort
```

The IngressController is created via the command line argument `initial-object-config-files` of `easegress-server`. And the exported port `web` is to receive external HTTP requests and forward them to the HTTP server in Easegress.

### Create backend service & Kubernetes ingress

Apply below YAML configuration to Kubernetes:

```yaml
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
        image: "gcr.io/google-samples/node-hello:1.0"
        env:
        - name: "PORT"
          value: "50001"
      - name: hello-v2
        image: "gcr.io/google-samples/hello-app:2.0"
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

---
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
```

After a while, we can leverage the below command to access both versions of the `hello` application:

```bash
$ curl http://{NODE_IP}/ -HHost:www.megaease.com
Hello, world!
Version: 2.0.0
Hostname: hello-deployment-6cbf765985-r6242

$ curl http://{NODE_IP}/ -HHost:www.example.com
Hello Kubernetes!
```

And we can see Easegress IngressController has forwarded requests to the correct application version according to Kubernetes ingress.
