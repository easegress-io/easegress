- [Background](#background)
- [Environment](#environment)
- [Topology](#topology)
- [Configuration](#configuration)
  - [Nginx](#nginx)
  - [Easegress](#easegress)
  - [Traefik](#traefik)
  - [Echo Server](#echo-server)
- [Testing](#testing)
- [Summary](#summary)
- [Ubuntu system status](#ubuntu-system-status)
  - [Easegress](#easegress-1)
  - [NGINX](#nginx-1)
  - [Traefik](#traefik-1)
- [References](#references)
## Background 
* Easegress is commonly used in Traffic Gateway and API Gateway scenarios. This benchmark aims to indicate the performance level of Easegress as Traffic Gateway.


## Environment
1. **Baremetal**: AWS r5.xlarge X 3 (4core/32GB memory/100GB disk/Up to 10 Gigabit bandwidth) 
2. Operation system 

``` bash
Linux vmname 5.4.0-1029-aws #30-Ubuntu SMP Tue Oct 20 10:06:38 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux`


```
3. Nginx version `nginx/1.18.0 (Ubuntu)`
3. Easegress version `1.0.1/ Go version: go1.16.5 OS/Arch: linux/amd64`
4. Traefik version `2.4.9/ Go version:/go1.16.5 OS/Arch: linux/amd64` 
5. Stress test tool [hey](https://github.com/rakyll/hey): v0.1.4

## Topology

``` plain 

   +----------------+                  +---------------+  
   |                |                  |               |  
   |    vm01        |<-----------------+     vm02      |
   | (Easegress     |                  | (Testtool:hey)|
   | /Traefik/Nginx)|                  |               |
   |                |                  |               |
   +----------------+------+           +---------------+
                           | 
                           |
   +----------------+      |        
   |                |      |
   |    vm03        |      |
   | (Echo HTTPSvr) |<-----+
   |                |
   |                | 
   +----------------+ 

```

## Configuration
### Nginx 

```bash
server {
    listen       8080;
    server_name  localhost;


    location /pipeline {
        proxy_pass     http://${echo_svr_ip}:9095;
        keepalive_timeout  60;
    }
}
```

### Easegress
1. Easegress config
``` yaml
name: member-001
cluster-name: cluster-test
cluster-role: writer
cluster-client-url: http://127.0.0.1:2379
cluster-peer-url: http://127.0.0.1:2380
cluster-join-urls:
api-addr: 127.0.0.1:2381
data-dir: ./data
wal-dir: ""
cpu-profile-file:
memory-profile-file:
log-dir: ./log
member-dir: ./member
std-log-level: INFO
```

2.HTTPServer+Pipeline

``` yaml
- filters:
  - kind: Proxy
    mainPool:
      loadBalance:
        policy: roundRobin
      servers:
      - url: http://${echo_server_ip}:9095
    name: proxy
  flow:
  - filter: proxy
    jumpIf: {}
  kind: HTTPPipeline
  name: pipeline-demo

- http3: false
  https: false
  keepAlive: true
  keepAliveTimeout: 60s
  kind: HTTPServer
  maxConnections: 10240
  name: server-demo
  port: 10080
  rules:
  - host: ""
    hostRegexp: ""
    paths:
    - backend: pipeline-demo
      headers: []
      pathPrefix: /pipeline
      rewriteTarget: ""
```

### Traefik
1. Running binary directly with command `./traefik -c ./traefik.yml` [1]
2. Static lonfig: traefik.yaml
``` yaml 
log:
  level: INFO

entryPoints:
  web:
    address: ":8081"

providers:
  file:
    filename: /${file_path}/dynamic_conf.yml

```
3. Dynamic config: dynamic_conf.yml
``` yaml
http:
  routers:
    my-router:
      rule: "PathPrefix(`/pipeline`)"
      service: foo
      entryPoints:
      - web

  services:
    foo:
      loadBalancer:
        servers:
        - url: "http://${echo_server_ip}:9095"
```

### Echo Server
1. [Source code](https://github.com/megaease/easegress/tree/main/example/backend-service/mirror)
2. Its logic is accepting HTTP request and printing to the consul, in this stress testing, only uses `9095` port.


## Testing 
* Scenario 1: 50concurrency/900request/2miniute/not QPS limitation

``` bash

./hey -n 900   -c 50  -m GET http://172.20.2.35:10080/pipeline -z 2m   # Easegress
./hey -n 900   -c 50  -m GET http://172.20.2.35:8080/pipeline -z 2m    # Nginx 
./hey -n 900   -c 50  -m GET http://172.20.2.35:8081/pipeline -z 2m    # Traefik 

```

* Scenario 2: 100concurrency/90000request/2miniute/not QPS limitation

``` bash

./hey -n 90000    -c 100  -m GET http://172.20.2.35:10080/pipeline -z 2m   # Easegress
./hey -n 90000    -c 100  -m GET http://172.20.2.35:8080/pipeline -z 2m    # Nginx 
./hey -n 90000    -c 100  -m GET http://172.20.2.35:8081/pipeline -z 2m    # Traefik

```

* Scenario 3: 120concurrency/90000request/2miniute/not QPS limitation

``` bash

./hey -n 90000    -c 120  -m GET http://172.20.2.35:10080/pipeline -z 2m   # Easegress
./hey -n 90000    -c 120  -m GET http://172.20.2.35:8080/pipeline -z 2m    # Nginx 
./hey -n 90000    -c 120  -m GET http://172.20.2.35:8081/pipeline -z 2m    # Traefik

```

* Scenario 4: 100concurrency/900000request/5miniute/not QPS limitation

``` bash

./hey -n 900000   -c 100  -m GET http://172.20.2.35:10080/pipeline -z 5m   # Easegress
./hey -n 900000   -c 100  -m GET http://172.20.2.35:8080/pipeline -z 5m    # Nginx 
./hey -n 900000   -c 100  -m GET http://172.20.2.35:8081/pipeline -z 5m    # Traefik

```

* Scenario 5: 50concurrency/90000request/2miniute/not QPS limitation/with body `100000000000000000000000000000`
`100000000000000000000000000000` contains 30 characters which is 240 bytes, the HTTP request body length average is `from ~200 bytes to over 2KB`. [1]

``` bash

./hey -n 90000   -c 100  -m GET http://172.20.2.35:10080/pipeline -d '100000000000000000000000000000' -z 2m   # Easegress
./hey -n 90000   -c 100  -m GET http://172.20.2.35:8080/pipeline -d '100000000000000000000000000000' -z 2m    # Nginx 
./hey -n 90000   -c 100  -m GET http://172.20.2.35:8081/pipeline -d '100000000000000000000000000000' -z 2m    # Traefik

```

| Scenario/Product | Total | Slowest | Fastest | Average | RPS   | 90% Latency | 95% Latency | 99% Latency | load average(top -c ) |
| ---------------- | ----- | ------- | ------- | ------- | ----- | ----------- | ----------- | ----------- | --------------------- |
| #1/Easegress     | 0.2s  | 0.017s  | 0.0104s | 0.0113s | 4312  | 0.0125s     | 0.0140s     | 0.0164s     | 0/0/0                 |
| #1/Nginx         | 0.2s  | 0.015s  | 0.0104s | 0.0112s | 4383  | 0.0124s     | 0.0135s     | 0.0151s     | 0/0/0                 |
| #1/Traefik       | 0.2s  | 0.018s  | 0.0104s | 0.0113s | 4320  | 0.0123s     | 0.0133s     | 0.0174s     | 0/0/0                 |
| #2/Easegress     | 10s   | 0.035s  | 0.0103s | 0.0113s | 8826  | 0.0124s     | 0.0136s     | 0.0179s     | 0.34/0.10/0.03        |
| #2/Nginx         | 28s   | 0.095s  | 0.0103s | 0.0308s | 3146  | 0.0468s     | 0.0500s     | 0.0657s     | 1.37/0.35/0.11        |
| #2/Traefik       | 10s   | 0.051s  | 0.0103s | 0.0114s | 8685  | 0.0129      | 0.0139s     | 0.0167s     | 0.34/0.27/0.10        |
| #3/Easegress     | 8s    | 0.040s  | 0.0103s | 0.0114s | 10391 | 0.0129s     | 0.0145s     | 0.0199s     | 0.62/0.25/0.12        |
| #3/Nginx         | 29s   | 0.133s  | 0.0103s | 0.0373s | 3022  | 0.0614s     | 0.0607s     | 0.122s      | 1.42/0.46/0.20        |
| #3/Traefik       | 9s    | 0.011s  | 0.0103s | 0.0120s | 9892  | 0.0143s     | 0.0158s     | 0.0197s     | 0.40/0.21/0.15        |
| #4/Easegress     | 102s  | 1.515s  | 0.0103s | 0.0114s | 8775  | 0.0123s     | 0.0134s     | 0.0176s     | 2.52/0.94/0.42        |
| #4/Nginx         | 311s  | 1.112s  | 0.0103s | 0.0343s | 2893  | 0.0484s     | 0.0513s     | 0.0569s     | 3.93/2.65/1.33        |
| #4/Traefik       | 107s  | 1.394s  | 0.0103s | 0.0119s | 8371  | 0.0133s     | 0.0144s     | 0.0171s     | 0.95/0.37/0.37        |
| #5/Easegress     | 10s   | 0.059s  | 0.0103s | 0.0113s | 8797  | 0.0124s     | 0.0134s     | 0.0177s     | 0.27/0.23/0.31        |
| #5/Nginx         | 28s   | 0.086s  | 0.0103s | 0.0311s | 3130  | 0.0481s     | 0.0510s     | 0.0577s     | 1.25/0.45/0.37        |
| #5/Traefik       | 10s   | 0.172s  | 0.0103s | 0.0118s | 8393  | 0.0136s     | 0.0149s     | 0.0182s     | 0.68/0.44/0.36        |

## Summary
1. RPS comparing
![rps](./stress-test-rps.png)

2. P99 Latency comparing 

![latency](./stress-test-p99-latency.png)
 

## Ubuntu system status 
* In Scenario #4

### Easegress

``` bash
top - 12:39:23 up 1 day, 22:36,  1 user,  load average: 1.71, 0.83, 0.51
Tasks: 130 total,   1 running, 129 sleeping,   0 stopped,   0 zombie
%Cpu0  : 29.2 us,  7.8 sy,  0.0 ni, 60.1 id,  0.0 wa,  0.0 hi,  2.8 si,  0.0 st
%Cpu1  : 32.1 us,  7.3 sy,  0.0 ni, 58.5 id,  0.0 wa,  0.0 hi,  2.1 si,  0.0 st
%Cpu2  : 30.1 us,  7.0 sy,  0.0 ni, 58.7 id,  0.0 wa,  0.0 hi,  4.2 si,  0.0 st
%Cpu3  : 30.4 us,  8.9 sy,  0.0 ni, 57.7 id,  0.0 wa,  0.0 hi,  3.1 si,  0.0 st
MiB Mem :  31654.1 total,  20964.3 free,    548.4 used,  10141.3 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  30652.3 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                             
  34028 root      20   0  836680 144760  44128 S 172.0   0.4  17:49.58 /home/ubuntu/easegress-stresstest/bin/easegress-server --config-file /home/ubuntu/e+
    491 root      20   0   81896   3748   3436 S   0.3   0.0   0:02.89 /usr/sbi         
```

### NGINX

``` bash
top - 12:40:33 up 1 day, 22:37,  1 user,  load average: 2.30, 1.10, 0.62
Tasks: 130 total,   6 running, 124 sleeping,   0 stopped,   0 zombie
%Cpu0  :  1.0 us, 95.7 sy,  0.0 ni,  0.0 id,  0.0 wa,  0.0 hi,  3.3 si,  0.0 st
%Cpu1  :  2.0 us, 95.0 sy,  0.0 ni,  0.0 id,  0.0 wa,  0.0 hi,  3.0 si,  0.0 st
%Cpu2  :  2.0 us, 95.7 sy,  0.0 ni,  0.0 id,  0.0 wa,  0.0 hi,  2.3 si,  0.0 st
%Cpu3  :  1.3 us, 96.3 sy,  0.0 ni,  0.0 id,  0.0 wa,  0.0 hi,  2.3 si,  0.0 st
MiB Mem :  31654.1 total,  20936.9 free,    564.7 used,  10152.5 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  30636.0 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                             
  33301 www-data  20   0   56144   6296   4212 R 100.0   0.0  13:02.18 nginx: worker process                                                               
  33298 www-data  20   0   56448   6600   4212 R  99.7   0.0  13:35.58 nginx: worker process                                                               
  33302 www-data  20   0   56144   6296   4212 R  99.7   0.0  13:12.51 nginx: worker process                                                               
  33299 www-data  20   0   56124   6408   4212 R  99.3   0.0  13:22.12 nginx: worker process  
```

### Traefik

``` bash
top - 12:41:53 up 1 day, 22:38,  1 user,  load average: 2.43, 1.45, 0.79
Tasks: 130 total,   1 running, 129 sleeping,   0 stopped,   0 zombie
%Cpu0  :  0.4 us, 10.8 sy, 21.9 ni, 65.2 id,  0.0 wa,  0.0 hi,  1.8 si,  0.0 st
%Cpu1  :  0.0 us, 10.3 sy, 23.5 ni, 64.1 id,  0.0 wa,  0.0 hi,  2.1 si,  0.0 st
%Cpu2  :  0.0 us, 10.7 sy, 21.4 ni, 66.4 id,  0.0 wa,  0.0 hi,  1.5 si,  0.0 st
%Cpu3  :  0.4 us, 10.1 sy, 21.9 ni, 64.7 id,  0.0 wa,  0.0 hi,  2.9 si,  0.0 st
MiB Mem :  31654.1 total,  20901.3 free,    591.5 used,  10161.3 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.  30609.3 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                             
  63509 root      25   5  789068  96656  44852 S 153.5   0.3   4:10.48 ./traefik -c traefik.yml  
```

1. Nginx processes use most time in system mode, may due to the context switch between kernel mode and user mode. 
2. Easegress/Traefik uses goroutine user-space scheduling for avoding heavy context switching cost.
 
## References
[1] https://stackoverflow.com/questions/60227270/simple-reverse-proxy-example-with-traefik
[2] https://stackoverflow.com/questions/5358109/what-is-the-average-size-of-an-http-request-response-header
