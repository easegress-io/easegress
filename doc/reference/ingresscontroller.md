# IngressController 
- [IngressController](#ingresscontroller)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration)
    - [Controller spec](#controller-spec)
  - [Getting Started](#getting-started)
    - [Role Based Access Control configuration](#role-based-access-control-configuration)
    - [Access to disk](#access-to-disk)
    - [Create Headless service and set ports](#create-headless-service-and-set-ports)
    - [Configurations to ConfigMap](#configurations-to-configmap)
    - [Deploy Easegress IngressController](#deploy-easegress-ingresscontroller)
    - [Create backend service & Kubernetes ingress](#create-backend-service--kubernetes-ingress)
  - [Multi-node IngressController](#multi-node-ingresscontroller)
  - [Add secondary Easegress instances](#add-secondary-easegress-instances)

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

### Access to disk

Easegress instances needs to access disk, to support failure of the pods. In this example we use local volume, but you could also mount any cloud providers volumes ([here](https://kubernetes.io/docs/concepts/storage/volumes/) are some of the options).

To mount local directory at host machine, we create PersistentVolume and StorageClass. Replace `<YOUR-HOSTNAME-HERE>` by hostname of the machine, where Easegress can use the disk. 

```yaml
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: easegress-pv-1
spec:
  capacity:
    storage: 4Gi
  volumeMode: Filesystem
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Delete
  storageClassName: easegress-storage
  hostPath:
    path: /opt/easegress
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - <YOUR-HOSTNAME-HERE>
---

apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: easegress-storage
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

### Create Headless service and set ports

Create ClusterIP [Headless](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services) service to facilitate Easegress server(s) communication.
```yaml
---

apiVersion: v1
kind: Service
metadata:
  name: easegress-hs
  namespace: default
spec:
  clusterIP: None
  ports:
  - name: admin-port
    port: 2381
    protocol: TCP
    targetPort: 2381
  - name: peer-port
    port: 2380
    protocol: TCP
    targetPort: 2380
  - name: client-port
    port: 2379
    protocol: TCP
    targetPort: 2379
  selector:
    app: easegress-ingress
  type: ClusterIP

---

apiVersion: v1
kind: Service
metadata:
  name: easegress-public
  namespace: default
spec:
  ports:
  - name: admin-port
    nodePort: 31255 # random port
    port: 2381
    protocol: TCP
    targetPort: 2381
  selector:
    app: easegress-ingress
  type: NodePort
```

### Configurations to ConfigMap

Let's use ConfigMap to store Easegress server configuration and Easegress ingress configuration. We will use this ConfigMap later in StatefulSet.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: easegress-cluster-cm
  namespace: default
data:
  eg-primary.yaml: |
    cluster-name: easegress-ingress-controller
    cluster-role: primary
    cluster:
      listen-client-urls:
      - http://0.0.0.0:2379
      listen-peer-urls:
      - http://0.0.0.0:2380
      initial-cluster:
      - easegress-0: http://easegress-0.easegress-hs.default:2380
    api-addr: 0.0.0.0:2381
    data-dir: /opt/easegress/data
    wal-dir: ""
    cpu-profile-file: ""
    memory-profile-file: ""
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

The `eg-primary.yaml` creates Easegress cluster with one member: `http://easegress-0.easegress-hs.default:2380`. Once we create a StatefulSet called `easegress`, using serviceName `easegress-hs` and inside namespace `default`, this URL will match Kubernetes DNS record. You can read more about stable network ids [here](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#stable-network-id).


### Deploy Easegress IngressController
To deploy the IngressController, we will create a StatefulSet and a Service as below:

```yaml
apiVersion: apps/v1
kind: StatefulSet
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
  serviceName: easegress-hs
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
          echo name: $EG_NAME > /opt/eg-config/config.yaml &&
          cat /opt/eg-config/eg-primary.yaml >> /opt/eg-config/config.yaml &&
          /opt/easegress/bin/easegress-server \
            -f /opt/eg-config/config.yaml \
            --advertise-client-urls http://$(EG_NAME).easegress-hs.default:2379 \
            --initial-advertise-peer-urls http://$(EG_NAME).easegress-hs.default:2380 \
            --initial-object-config-files /opt/eg-config/controller.yaml
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
        name: easegress-ingress
        ports:
        - containerPort: 2381
          name: admin-port
          protocol: TCP
        - containerPort: 2380
          name: peer-port
          protocol: TCP
        - containerPort: 2379
          name: client-port
          protocol: TCP
        resources:
          limits:
            cpu: 1200m
            memory: 2Gi
          requests:
            cpu: 100m
            memory: 256Mi
        volumeMounts:
        - mountPath: /opt/eg-config/eg-primary.yaml
          name: easegress-cluster-cm
          subPath: eg-primary.yaml
        - mountPath: /opt/eg-config/controller.yaml
          name: easegress-cluster-cm
          subPath: controller.yaml
        - mountPath: /opt/eg-data/
          name: easegress-pv
      restartPolicy: Always
      volumes:
      - configMap:
          defaultMode: 420
          items:
          - key: eg-primary.yaml
            path: eg-primary.yaml
          - key: controller.yaml
            path: controller.yaml
          name: easegress-cluster-cm
        name: easegress-cluster-cm
  updateStrategy:
    type: RollingUpdate
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: easegress-pv
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 3Gi
      storageClassName: easegress-storage
      volumeMode: Filesystem

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

This StatefulSet creates one replica (at runtime easegress-0), but we will see later in this tutorial, what changes are needed to be able to have 3,5 or 7 replicas.

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
$ curl http://{NODE_IP}/ -HHost:www.megaease.com
Hello, world!
Version: 2.0.0
Hostname: hello-deployment-6cbf765985-r6242

$ curl http://{NODE_IP}/ -HHost:www.example.com
Hello, world!
Version: 1.0.0
Hostname: hello-deployment-6cbf765985-r6242
```

And we can see Easegress IngressController has forwarded requests to the correct application version according to Kubernetes ingress.

## Multi-node IngressController

Until now we have only created IngressController with one instance running. To support high-availability scenarios, let's go through the configurations for creating a cluster of 3 *primary* Easegress instances.

Before applying the changes, let's remove the old resources first:
```yaml
kubectl delete statefulset easegress
kubectl delete pv easegress-pv-0
kubectl delete configmap easegress-cluster-cm
```
Also run `rm -rf /opt/easegress` on machine that `easegress-pv-0` was attached to.

Here are briefly the modifications we need in order to support 3 member cluster:
- in part [Access to disk](#access-to-disk): instead of creating only one *PersistentVolume* `easegress-pv-1`, create three of them `easegress-pv-1`, `easegress-pv-2`, `easegress-pv-3`
- in part [Configurations to ConfigMap](#configurations-to-configmap) add all three member urls to cluster configuration
- in part [Deploy Easegress IngressController](#deploy-easegress-ingresscontroller), change the *StatefulfulSet* replica count to 3


So the *PersistentVolume* creation would look something like this:

```yaml
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: easegress-pv-1
spec:
  capacity:
    storage: 4Gi
  volumeMode: Filesystem
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Delete
  storageClassName: easegress-storage
  hostPath:
    path: /opt/easegress
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - <YOUR-HOSTNAME-1-HERE>
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: easegress-pv-2
spec:
  capacity:
    storage: 4Gi
  volumeMode: Filesystem
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Delete
  storageClassName: easegress-storage
  hostPath:
    path: /opt/easegress
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - <YOUR-HOSTNAME-2-HERE>
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: easegress-pv-3
spec:
  capacity:
    storage: 4Gi
  volumeMode: Filesystem
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Delete
  storageClassName: easegress-storage
  hostPath:
    path: /opt/easegress
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - <YOUR-HOSTNAME-3-HERE>

---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: easegress-storage
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```
Now here's how we add all three cluster member URLs to *ConfigMap*:

```yaml
kind: ConfigMap
metadata:
  name: easegress-cluster-cm
  namespace: default
data:
  eg-primary.yaml: |
    cluster-name: easegress-ingress-controller
    cluster-role: primary
    cluster:
      listen-client-urls:
      - http://0.0.0.0:2379
      listen-peer-urls:
      - http://0.0.0.0:2380
      initial-cluster:
      - easegress-0: http://easegress-0.easegress-hs.default:2380
      - easegress-1: http://easegress-1.easegress-hs.default:2380
      - easegress-2: http://easegress-2.easegress-hs.default:2380
    api-addr: 0.0.0.0:2381
    data-dir: /opt/easegress/data
    wal-dir: ""
    cpu-profile-file: ""
    memory-profile-file: ""
    log-dir: /opt/easegress/log
    debug: false
  controller.yaml: |
    ...
```
And last but not least the *StatefulSet* with three replicas:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app: easegress-ingress
  name: easegress
  namespace: default
spec:
  replicas: 3 # changed from 1 to 3
  selector:
    matchLabels:
      app: easegress-ingress
  ...
```

Now you have configuration yaml files for bootstrapping three member Easegress Ingress Controller cluster. You can test ingress like before `curl http://{NODE_IP}/ -HHost:www.example.com`. To verify that the cluster is healthy, query admin port with `egctl`:
```bash
egctl --server {NODE_IP}:31255 member list |grep " name"
```
which should output:
```bash
#     name: easegress-0
#     name: easegress-1
#     name: easegress-2
```

You can read more multi-node Easegress this cookbook [chapter](./doc/../cookbook/multi_node_cluster.md).

## Add secondary Easegress instances

Previously in this tutorial we went through the necessary kubernetes files for single instance Easegress Ingress Controller and three member Easegress Ingress Controller cluster. In both cases we deployed *primary* Easegress instances. Sometimes we want to scale the number of instances up or down. This can be achieved using *secondary* Easegress instances, that unlike *primary* instances, can be added to existing cluster or removed anytime. *Secondary* instances do not require [stable network id](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#stable-network-id) so we can use Deployment instead of StatfulSet.

Let's create an Kubernetes Deployment that adds two *secondary* Easegress instances to cluster.

Here's Easegress configuration for *secondary* instance.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: easegress-cluster-cm
  namespace: default
data:
  eg-primary.yaml: |
    ...
  eg-secondary.yaml: |
    cluster-name: easegress-ingress-controller
    cluster-role: secondary
    cluster:
      primary-listen-peer-urls:
      - http://easegress-0.easegress-hs.default:2380
    api-addr: 0.0.0.0:2381
    data-dir: /opt/easegress/data
    wal-dir: ""
    cpu-profile-file: ""
    memory-profile-file: ""
    log-dir: /opt/easegress/log
    debug: false
  controller.yaml: |
    ...
```

The URL `http://easegress-0.easegress-hs.default:2380` defined in `cluster.primary-listen-peer-urls` is an address of one cluster member, which *secondary* instance can use to connect to the cluster.

Deployment could look like this:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: easegress-ingress
  name: easegress-secondary
  namespace: default
spec:
  replicas: 2
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
          echo name: $EG_NAME > /opt/eg-config/config.yaml &&
          cat /opt/eg-config/eg-secondary.yaml >> /opt/eg-config/config.yaml &&
          /opt/easegress/bin/easegress-server \
            -f /opt/eg-config/config.yaml \
            --initial-object-config-files /opt/eg-config/controller.yaml
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
        name: easegress-secondary
        ports:
        - containerPort: 2381
          name: admin-port
          protocol: TCP
        - containerPort: 2380
          name: peer-port
          protocol: TCP
        - containerPort: 2379
          name: client-port
          protocol: TCP
        resources:
          limits:
            cpu: 1200m
            memory: 2Gi
          requests:
            cpu: 100m
            memory: 256Mi
        volumeMounts:
        - mountPath: /opt/eg-config/eg-secondary.yaml
          name: easegress-cluster-cm
          subPath: eg-secondary.yaml
        - mountPath: /opt/eg-config/controller.yaml
          name: easegress-cluster-cm
          subPath: controller.yaml
        - mountPath: /opt/eg-data/
          name: ingress-data-volume
      restartPolicy: Always
      volumes:
      - emptyDir: {}
        name: ingress-data-volume
      - configMap:
          defaultMode: 420
          items:
          - key: eg-secondary.yaml
            path: eg-secondary.yaml
          - key: controller.yaml
            path: controller.yaml
          name: easegress-cluster-cm
        name: easegress-cluster-cm
---
```

Applying the deployment to the cluster creates two pods with prefix `easegress-secondary-`. We can again log into one of pods and see new *secondary* members. Let's check again the cluster members using `egctl`:

```bash
egctl --server {NODE_IP}:31255 member list |grep " name"
```
which should give something like following:
```bash
    # name: easegress-0
    # name: easegress-1
    # name: easegress-2
    # name: easegress-secondary-758695b96c-2rzqt
    # name: easegress-secondary-758695b96c-2vcpz
```
Note that secondary member names might something different, as we did not use headless ClusterIP for them.

We now have 5 Easegress Ingress Controller instances running, handling the ingress traffic of our Kubernetes cluster.
