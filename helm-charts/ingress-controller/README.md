# Easegress ingress controller chart

Helm charts for installing Easegress ingress controller on Kubernetes

## Setup

```shell
# create namespace at first
kubectl create ns ingress-easegress
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
helm install ingress-easegress -n ingress-easegress ./helm-charts/ingress-controller

# install with custom values
helm install ingress-easegress -n ingress-easegress ./helm-charts/ingress-controller \
  --set service.nodePort=4080 \
  --set image.tag=v1.4.0 \
  --set ingressClass.name=test-eg \
  --set controller.name=test-eg \
  --set 'controller.namespaces={ingress-easegress, default}'

# install cluster of 3 primary and 2 secondary Easegress ingress instances
helm install ingress-easegress -n ingress-easegress ./helm-charts/ingress-controller \
  --set cluster.primaryReplicas=3 \
  --set cluster.secondaryReplicas=2

# install using persistentVolume on node with hostname "hostname-xyz"
# to support recovery when pod crashes
helm install ingress-easegress -n ingress-easegress ./helm-charts/ingress-controller \
  --set cluster.volumeType=persistentVolume \
  --set 'cluster.nodeHostnames={hostname-xyz}'
```

## Uninstall

```shell
helm uninstall ingress-easegress -n ingress-easegress

#sometimes helm does not delete pvc and pv. Delete manually each pvc.
kubectl delete pvc easegress-pv-ingress-easegress-0 -n ingress-easegress
# same for easegress-pv-ingress-easegress-i...n
```
