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

Add filters and objects to Easegress:

```shell
egctl --server {NODE_IP}:31255 create object -f pipeline.yaml
```
where NODE_IP is the IP address a node running Easegress pod and `pipeline.yaml` Easegress object definition.

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
| service.adminPort | int | `31255` | nodePort for egctl access. |
| cluster.primaryReplicas | int | `1` | number of easegress service that persists cluster data to disk |
| cluster.volumeType | string | `emptyDir` | `emptyDir`: use pods internal filesystem that is not persisted when pod crashes. Use `emptyDir` only when primaryReplicas is 1. | `persistentVolume`, create as many persistenVolumes and persistentVolumeClaims as there are nodeHostnames.
| cluster.volumeLocalPath | string | `/opt/easegress` | local path of persistenVolume on nodes |
| cluster.nodeHostnames | list | `[]` | nodeHostnames are hostnames of VMs/Kubernetes nodes. Only used when `volumeType: persistentVolume`. Note that this require nodes to be static. |
| secondaryReplicas | int | `0` | number of easegress service that not persists cluster data to disk. |
| log.path | string | `/opt/easegress/log` | log path inside container |

> By default, k8s use range 30000-32767 for NodePort. Make sure you choose right port number.
