# Prometheus

- [Prometheus](#prometheus)
    - [Exporter URI](#exporter-uri)
    - [Metrics](#metrics)
        - [Objects](#objects)
            - [HTTPServer](#httpserver)
        - [Filters](#filters)
            - [Proxy](#proxy)
    - [Create Metrics for Extended Objects and Filters](#create-metrics-for-extended-objects-and-filters)

Easegress has a builtin Prometheus exporter.

## Exporter URI

```
Get /apis/v2/metrics
```

## Metrics

### Objects

#### HTTPServer

| Metric                                     | Type      | Description                                                  | Labels                                                                  |
|--------------------------------------------|-----------|--------------------------------------------------------------|-------------------------------------------------------------------------|
| httpserver_health                          | gauge     | show the status for the http server: 1 for ready, 0 for down | clusterName, clusterRole, instanceName, name, kind                      |
| httpserver_total_requests                  | counter   | the total count of http requests                             | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_total_responses                 | counter   | the total count of http resposnes                            | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_total_error_requests            | counter   | the total count of http error requests                       | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_requests_duration               | histogram | request processing duration histogram                        | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_requests_size_bytes             | histogram | a histogram of the total size of the request. Includes body  | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_responses_size_bytes            | histogram | a histogram of the total size of the returned responses body | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_requests_duration_percentage    | summary   | request processing duration summary                          | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_requests_size_bytes_percentage  | summary   | a summary of the total size of the request. Includes body    | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |
| httpserver_responses_size_bytes_percentage | summary   | a summary of the total size of the returned responses body   | clusterName, clusterRole, instanceName, name, kind, routerKind, backend |

### Filters

#### Proxy

| Metric                              | Type      | Description                                   | Labels                                                                              |
|-------------------------------------|-----------|-----------------------------------------------|-------------------------------------------------------------------------------------|
| proxy_total_connections             | counter   | the total count of proxy connections          | clusterName, clusterRole, instanceName, name, kind, loadBalancePolicy, filterPolicy |
| proxy_total_error_connections       | counter   | the total count of proxy error connections    | clusterName, clusterRole, instanceName, name, kind, loadBalancePolicy, filterPolicy |
| proxy_request_body_size             | histogram | a histogram of the total size of the request  | clusterName, clusterRole, instanceName, name, kind, loadBalancePolicy, filterPolicy |
| proxy_response_body_size            | histogram | a histogram of the total size of the response | clusterName, clusterRole, instanceName, name, kind, loadBalancePolicy, filterPolicy |
| proxy_request_body_size_percentage  | summary   | a summary of the total size of the request    | clusterName, clusterRole, instanceName, name, kind, loadBalancePolicy, filterPolicy |
| proxy_response_body_size_percentage | summary   | a summary of the total size of the response   | clusterName, clusterRole, instanceName, name, kind, loadBalancePolicy, filterPolicy |

## Create Metrics for Extended Objects and Filters

We provide several helper functions to help you create Prometheus metrics
when develop your own Objects and Filters.

* `prometheushelper.NewCounter`
* `prometheushelper.NewGauge`
* `prometheushelper.NewHistogram`
* `prometheushelper.NewSummary`

These metrics will be registered to `DefaultRegisterer` automatically.

Besides, you can use native Prometheus SDK to create metrics and register them
by yourself.
