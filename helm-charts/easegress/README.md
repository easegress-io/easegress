# Easegress chart

Helm charts for installing Easegress on Kubernetes.

## Setup
```shell
# create namespace at first
kubectl create ns easegress
```

### Prepare persistent volume (optional)

If you are going to use persistent volumes, run following shell command on each persistent volume node:
```shell
sudo mkdir /opt/easegress
sudo chmod 700 /opt/easegress
```

## Usage
```shell

# install with default values
helm install easegress -n easegress ./helm-charts/easegress

# install with custom values
helm install easegress -n easegress ./helm-charts/easegress \
  --set service.nodePort=4080 \
  --set image.tag=v1.5.0 \

# install cluster of 3 primary and 2 secondary Easegress instances
helm install easegress -n easegress ./helm-charts/easegress \
  --set cluster.primaryReplicas=3 \
  --set cluster.secondaryReplicas=2

# install using persistentVolume on node with hostname "hostname-xyz"
# to support recovery when pod crashes
helm install easegress -n easegress ./helm-charts/easegress \
  --set cluster.volumeType=persistentVolume \
  --set 'cluster.nodeHostnames={hostname-xyz}'
```

The Admin API listens on `127.0.0.1:2381` inside the pod and is not exposed by a service by default.
Use `kubectl port-forward` for local administration:

```shell
kubectl port-forward -n easegress pod/easegress-0 2381:2381
```

Add filters and objects to Easegress:

```shell
egctl --server 127.0.0.1:2381 create object -f pipeline.yaml
```
where `pipeline.yaml` is an Easegress object definition.

To expose the Admin API through a NodePort, enable the admin service explicitly and configure authentication:

```shell
helm install easegress -n easegress ./helm-charts/easegress \
  --set admin.apiAddr=0.0.0.0:2381 \
  --set service.admin.enabled=true \
  --set service.admin.type=NodePort \
  --set admin.basicAuth.admin=change-me
```

Set `admin.allowUnsafeNoAuth=true` only when an external control already protects the Admin API.

## Uninstall

```shell
helm uninstall easegress -n easegress

# sometimes helm does not delete pvc and pv. Delete manually each pvc.
kubectl delete pvc easegress-pv-easegress-0 -n easegress
# same for easegress-pv-easegress-i...n
```

## Parameters

The following table lists the configurable parameters of the Easegress Helm installation.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| service.nodePort | int | `30780` | nodePort for easegress service. |
| service.adminPort | int | `31255` | Deprecated fallback for `service.admin.nodePort`. |
| service.admin.enabled | bool | `false` | create a service for the Admin API. |
| service.admin.type | string | `ClusterIP` | service type for the Admin API. |
| service.admin.nodePort | int | `31255` | nodePort for egctl access when `service.admin.type` is `NodePort`. |
| admin.apiAddr | string | `127.0.0.1:2381` | address the Admin API listens on. |
| admin.basicAuth | map | `{}` | username to password map for Admin API basic authentication. |
| admin.allowUnsafeNoAuth | bool | `false` | allow external Admin API services without chart-configured authentication. |
| serviceAccount.automountServiceAccountToken | bool | `false` | mount the Kubernetes service account token into Easegress pods. |
| rbac.readSecrets | bool | `false` | grant read/list/watch permissions on Kubernetes Secrets. |
| cluster.primaryReplicas | int | `1` | number of easegress service that persists cluster data to disk |
| cluster.volumeType | string | `emptyDir` | `emptyDir`: use pods internal filesystem that is not persisted when pod crashes. Use `emptyDir` only when primaryReplicas is 1. | `persistentVolume`, create as many persistenVolumes and persistentVolumeClaims as there are nodeHostnames.
| cluster.volumeLocalPath | string | `/opt/easegress` | local path of persistenVolume on nodes |
| cluster.nodeHostnames | list | `[]` | nodeHostnames are hostnames of VMs/Kubernetes nodes. Only used when `volumeType: persistentVolume`. Note that this require nodes to be static. |
| secondaryReplicas | int | `0` | number of easegress service that not persists cluster data to disk. |
| log.path | string | `/opt/easegress/log` | log path inside container |

> By default, k8s use range 30000-32767 for NodePort. Make sure you choose right port number.
