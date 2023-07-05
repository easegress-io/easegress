### Gateway Class

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: GatewayClass
metadata:
  name: cluster-gateway
spec:
  controllerName: "megaease.com/gateway-controller"
```

### Gateway

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: prod-web
spec:
  gatewayClassName: cluster-gateway
  listeners:
  - protocol: HTTP
    port: 80
    name: prod-web-gw
    allowedRoutes:
      namespaces:
        from: Same
```


### RBAC

```yaml
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: easegress-gateway-controller
rules:
- apiGroups: [""] # "" indicates the core API group
  resources: ["services", "secrets", "endpoints", "namespaces"]
  verbs: ["get", "watch", "list"]
- apiGroups: ["gateway.networking.k8s.io"]
  resources: ["gatewayclasses", "httproutes", "gateways"]
  verbs: ["get", "watch", "list"]

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: easegress-gateway-controller
  namespace: default

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: easegress-gateway-controller
subjects:
- kind: ServiceAccount
  name: easegress-gateway-controller
  namespace: default
roleRef:
  kind: ClusterRole
  name: easegress-gateway-controller
  apiGroup: rbac.authorization.k8s.io
```

### Config Map

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: easegress-cm
  namespace: default
data:
  easegress-server.yaml: |
    name: gateway-easegress
    cluster-name: easegress-gateway-controller
    cluster-role: primary
    api-addr: 0.0.0.0:2381
    data-dir: /opt/easegress/data
    log-dir: /opt/easegress/log
    debug: false
  controller.yaml: |
    kind: GatewayController
    name: gateway-controller-example
    kubeConfig:
    masterURL:
    namespaces: []
```

### Gateway Controller

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: easegress-gateway
  name: easegress
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: easegress-gateway
  template:
    metadata:
      labels:
        app: easegress-gateway
    spec:
      serviceAccountName: easegress-gateway-controller
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
          name: gateway-data-volume
        - mountPath: /opt/easegress/log
          name: gateway-data-volume
      restartPolicy: Always
      volumes:
      - emptyDir: {}
        name: gateway-data-volume
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
