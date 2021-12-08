# Easegress ingress controller chart

Helm charts for installing Easegress ingress controller on Kubernetes

## Usage

```shell
# create namespace at first
kubectl create ns ingress-easegress

# update common helm charts
helm dependency update ./helm-charts/ingress-controller

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
