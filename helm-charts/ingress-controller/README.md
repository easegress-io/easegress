# Easegress ingress controller chart

Helm charts for install Easegress ingress controller on Kubernetes

## Usage

```shell
# create namespace at first
kubectl create ns ingress-easegress

# install with default values
helm install ingress-easegress -n ingress-easegress ./charts/ingress-controller

# install with custom values
helm install test-eg -n test-eg ./charts/ingress-controller \
  --set service.nodePort=4080 \
  --set image.tag=v1.2.1 \
  --set ingressClass.name=test-eg \
  --set controller.name=test-eg \
  --set 'controller.namespaces={test, default}'
```
