# Easegress ingress controller chart

Helm charts for installing Easegress ingress controller on Kubernetes

## Setup

```shell
# create namespace at first
kubectl create ns ingress-easegress
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
```

You can reference [this example](/doc/reference/ingresscontroller.md#create-backend-service--kubernetes-ingress) for creating Kubernetes Ingress rules.

## Uninstall

```shell
helm uninstall ingress-easegress -n ingress-easegress
```

## Parameters

The following table lists the configurable parameters of the Ingress Controller Helm installation.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| service.nodePort | int | `30080` | nodePort for Easegress Ingress Controller. |
| replicas | int | `1` | number of Easegress Ingress Controllers |
| log.path | string | `/opt/easegress/log` | log path inside container |


> By default, k8s use range 30000-32767 for NodePort. Make sure you choose right port number.
