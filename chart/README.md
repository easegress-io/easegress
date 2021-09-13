# easegress chart

## usage

```shell
# create namespace at first
kubectl create ns easegress

# install with default values
helm install easegress -n easegress ./chart

# install with custom values
helm install test-eg -n test-eg ./chart \
  --set service.nodePort=4080 \
  --set image.tag=v1.2.1 \
  --set ingressClass.name=test-eg \
  --set controller.name=test-eg \
  --set 'controller.namespaces={test, default}'
```
