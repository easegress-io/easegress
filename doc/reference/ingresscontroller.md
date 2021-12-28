# IngressController

- [IngressController](#ingresscontroller)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration)
    - [Controller spec](#controller-spec)
  - [Getting Started](#getting-started)
    - [Role Based Access Control configuration](#role-based-access-control-configuration)
    - [Configurations to ConfigMap](#configurations-to-configmap)
    - [Deploy Easegress IngressController](#deploy-easegress-ingresscontroller)
    - [Create backend service & Kubernetes ingress](#create-backend-service--kubernetes-ingress)
  - [Multi-instance IngressController](#multi-instance-ingresscontroller)

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

- IngressController uses `kubeConfig` and `masterURL` to connect to Kubernetes, at least one of them must be specified when deployed outside of a Kubernetes cluster, and both are optional when deployed inside a cluster.

- The `namespaces` is an array of Kubernetes namespaces which the IngressController needs to watch, all namespaces are watched if left empty.
- IngressController only handles `Ingresses` with `ingressClassName` set to `ingressClass`, the default value of `ingressClass` is `easegress`.
- One IngressController manages a shared HTTP traffic gate and multiple pipelines according to the Kubernetes ingress. The `httpServer` section in the spec is the basic configuration for the shared HTTP traffic gate. The routing part of the HTTP server and pipeline configurations will be generated dynamically according to Kubernetes ingresses.

## Getting Started

### Role Based Access Control configuration

If your cluster is configured with RBAC, first you will need to authorize Easegress IngressController for using the Kubernetes API. Below is an example configuration:

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

### Configurations to ConfigMap

Let's use ConfigMap to store Easegress server configuration and Easegress ingress configuration. This ConfigMap is used later in Deployment.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: easegress-cm
  namespace: default
data:
  easegress-server.yaml: |
    name: ingress-easegress
    cluster-name: easegress-ingress-controller
    cluster-role: primary
    api-addr: 0.0.0.0:2381
    data-dir: /opt/easegress/data
    log-dir: /opt/easegress/log
    debug: false
  controller.yaml: |
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

The `easegress-serverl` creates Easegress instance named *easegress-ingress-controller* and `controller.yaml` defines IngressController object for Easegress.

### Deploy Easegress IngressController

To deploy the IngressController, we will create a Deployment and a Service as below:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: easegress-ingress
  name: easegress
  namespace: default
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
      - args:
        - -c
        - |-
          /opt/easegress/bin/easegress-server \
            -f /opt/eg-config/easegress-server.yaml \
            --initial-object-config-files /opt/eg-config/controller.yaml \
            --initial-cluster $(EG_NAME)=http://localhost:2380
        command:
        - /bin/sh
        env:
        - name: EG_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        image: megaease/easegress:latest
        imagePullPolicy: IfNotPresent
        name: easegress-primary
        resources:
          limits:
            cpu: 1200m
            memory: 2Gi
          requests:
            cpu: 100m
            memory: 256Mi
        volumeMounts:
        - mountPath: /opt/eg-config/easegress-server.yaml
          name: easegress-cm
          subPath: easegress-server.yaml
        - mountPath: /opt/eg-config/controller.yaml
          name: easegress-cm
          subPath: controller.yaml
        - mountPath: /opt/easegress/data
          name: ingress-data-volume
        - mountPath: /opt/easegress/log
          name: ingress-data-volume
      restartPolicy: Always
      volumes:
      - emptyDir: {}
        name: ingress-data-volume
      - configMap:
          defaultMode: 420
          items:
          - key: easegress-server.yaml
            path: easegress-server.yaml
          - key: controller.yaml
            path: controller.yaml
          name: easegress-cm
        name: easegress-cm
```

The IngressController is created via the command line argument `initial-object-config-files` of `easegress-server`. Notice how Easegress logs and data are stored to *emptyDir* called `ingress-data-volume` inside the pod. IngressController is stateless so we can restart new pods without preserving previous state.

Last but not least let's create service for forwarding the Ingress traffic to Easegress.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: easegress-public
  namespace: default
spec:
  ports:
  - name: web
    protocol: TCP
    port: 8080
    nodePort: 30080
  selector:
    app: easegress-ingress
  type: NodePort
```

The port `web` is to receive external HTTP requests from port 30080 and forward them to the HTTP server in Easegress.

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

Once all pods are up and running, we can leverage the command below to access both versions of the `hello` application:

```bash
$ curl http://{NODE_IP}:30080/ -HHost:www.megaease.com
Hello, world!
Version: 2.0.0
Hostname: hello-deployment-6cbf765985-r6242

$ curl http://{NODE_IP}:30080/ -HHost:www.example.com
Hello, world!
Version: 1.0.0
Hostname: hello-deployment-6cbf765985-r6242
```

And we can see Easegress IngressController has forwarded requests to the correct application version according to Kubernetes ingress.

## Multi-instance IngressController

In previous chapters we created IngressController with one instance running. To support high-availability scenarios, you can increase the number of replicas in the Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: easegress-ingress
  name: easegress
  namespace: default
spec:
  replicas: 2 # number of IngressController instances running
  ...
```
