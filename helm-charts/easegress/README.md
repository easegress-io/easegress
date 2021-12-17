# Easegress chart

Helm charts for installing Easegress on Kubernetes.

## Setup
```shell
# create namespace at first
kubectl create ns easegress

# update common helm charts
helm dependency update ./helm-charts/easegress
```

### Prepare persistent volume (optional)

If you are going to use persistent volumes, run following shell command on each persistent volume node:
```bash
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
  --set image.tag=v1.4.0 \

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
egctl --server {NODE_IP}:31255 object create -f pipeline.yaml
```
where NODE_IP is the IP address a node running Easegress pod and `pipeline.yaml` Easegress object definition.
