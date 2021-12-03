# Easegress ingress controller chart

Helm charts for install Easegress ingress controller on Kubernetes

## Usage

```shell
# create namespace at first
kubectl create ns ingress-easegress

# install with default values
helm install ingress-easegress -n ingress-easegress ./helm-charts/ingress-controller

# install with custom values
helm install ingress-easegress -n ingress-easegress ./helm-charts/ingress-controller \
  --set service.nodePort=4080 \
  --set image.tag=v1.3.2 \
  --set ingressClass.name=test-eg \
  --set controller.name=test-eg \
  --set 'controller.namespaces={ingress-easegress, default}'
```
