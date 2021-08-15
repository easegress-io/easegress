# Easegress Benchmark

|  Topology   | QPS  | 
|  ----  | ----  |
| ab --> Echo HTTPSvr  | 15698.81 |
| ab --> Nginx --> Echo HTTPSvr  | 13064.07 |
| ab --> Easegress --> Echo HTTPSvr  | 9747.49 |

## Preparing
### Environment
instance specification |vCPU|menory|quantity
|----|----|----|----|
|ecs.g6e.large|2 vCPU|	8 GiB|3|

|name| port|vm|version|ip|port
|----|----|----|----|----|----|
|Echo HTTPSvr|9095|vm1|(golang1.16.5)|172.20.97.112|9095|
|Nginx|8080|vm2|1.18.0|172.20.97.117|8080|
|Easegress|10080|vm2|1.0.1(golang 1.16.5)|172.20.97.117|10080|
|ab|-|vm3|-|172.20.97.115|-|

### Topology
``` plain 
+----------------+                          +---------------+  
|                |                          |               |  
|    vm02        |<------stress test--------+     vm03      |
| (Easegress     |                          | (Testtool:ab) |
| /Nginx)        |                          |               |
|                |                          |               |
+----------------+------+                   +---------+-----+
                        |                             |
                    stress test            echo svr base line test
+----------------+      |                             |
|                |      |                             |
|    vm1         |      |                             |
| (Echo HTTPSvr) |<-----+                             |
|                |<-----------------------------------+
|                |
+----------------+ 

```

## Configuration
**Echo Server**
1. [Source code](https://github.com/megaease/easegress/tree/main/example/backend-service/echo/echo.go)
2. Its logic is accepting HTTP request and printing to the console, in this testing, we only use `9095` port.

|ip| port|
|----|----| 
|172.20.97.112|9095|

**Nginx**

1. [nginx 1.18.0 scouce](https://nginx.org/download/nginx-1.18.0.tar.gz)
2. compile preparing
```shell
# 01. install gcc-c++
yum -y install gcc-c++

# 02. download source pcre-8.44 
wget https://ftp.pcre.org/pub/pcre/pcre-8.44.tar.gz
tar -xzvf pcre-8.44.tar.gz

# 03. download source zlib-1.2.11
wget http://zlib.net/zlib-1.2.11.tar.gz
tar -xzvf zlib-1.2.11.tar.gz

# 04. download source openssl-1.1.1j
wget https://www.openssl.org/source/old/1.1.1/openssl-1.1.1j.tar.gz
tar -xzvf openssl-1.1.1j.tar.gz

# 05. configure
wget https://nginx.org/download/nginx-1.18.0.tar.gz
tar -xzvf nginx-1.18.0.tar.gz
cd nginx-1.18.0
./configure
    --sbin-path=/usr/local/nginx/nginx
    --conf-path=/usr/local/nginx/nginx.conf
    --pid-path=/usr/local/nginx/nginx.pid
    --with-http_ssl_module
    --with-pcre=../pcre-8.44
    --with-zlib=../zlib-1.2.11
    --with-openssl=..openssl-1.1.1j

# 06. make
make

```

3. configuration file 
   
/usr/local/nginx/nginx.conf
```
worker_processes  1;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;
    sendfile        on;

    server {
        listen       8080;
        server_name  localhost;

        location / {
            root   html;
            index  index.html index.htm;
        }
        
        location /pipeline {
            proxy_pass  http://172.20.97.112:9095; # 转发规则
            proxy_set_header Host $proxy_host; # 修改转发请求头，让8080端口的应用可以受到真实的请求
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }
    }

}
```

4. start
```shell
/usr/local/nginx/nginx
```

5. test
```shell
curl http://172.20.97.117:8080/pipeline
```

**Easegress**
1. git clone Easegress and make
```shell
git clone https://github.com/megaease/easegress.git
cd easegress
make
```

2. start
```shell
[root@iZj6c7bzufsptcwxsjr1byZ bin]# export PATH=${PATH}:$(pwd)/bin/
[root@iZj6c7bzufsptcwxsjr1byZ bin]# ./easegress-server 
2021-08-15T14:15:04.012+08:00   INFO    server/main.go:57       Easegress release: v1.1.0, repo: https://github.com/megaease/easegress.git, commit: git-d7c3c52
2021-08-15T14:15:04.012+08:00   INFO    cluster/config.go:125   etcd config: init-cluster:eg-default-name=http://localhost:2380 cluster-state:existing force-new-cluster:false
2021-08-15T14:15:04.516+08:00   INFO    cluster/cluster.go:675  server is ready
2021-08-15T14:15:04.516+08:00   INFO    cluster/cluster.go:389  client connect with endpoints: [http://localhost:2380]
2021-08-15T14:15:04.516+08:00   INFO    cluster/cluster.go:402  client is ready
2021-08-15T14:15:04.519+08:00   INFO    cluster/cluster.go:535  lease is ready (grant new one: b6a7b48730a4f05)
2021-08-15T14:15:04.519+08:00   INFO    cluster/cluster.go:213  cluster is ready
2021-08-15T14:15:04.52+08:00    INFO    supervisor/supervisor.go:115    create TrafficController
2021-08-15T14:15:04.52+08:00    INFO    supervisor/supervisor.go:115    create RawConfigTrafficController
2021-08-15T14:15:04.52+08:00    INFO    supervisor/supervisor.go:115    create StatusSyncController
2021-08-15T14:15:04.521+08:00   INFO    cluster/cluster.go:575  session is ready
2021-08-15T14:15:04.521+08:00   INFO    api/api.go:72   register api group admin
2021-08-15T14:15:04.521+08:00   INFO    api/server.go:78        api server running in localhost:2381
2021-08-15T14:15:09.521+08:00   INFO    cluster/member.go:154   store clusterMembers: eg-default-name(689e371e88f78b6a)=http://localhost:2380
2021-08-15T14:15:09.521+08:00   INFO    cluster/member.go:155   store knownMembers  : eg-default-name(689e371e88f78b6a)=http://localhost:2380
2021-08-15T14:29:46.384+08:00   INFO    trafficcontroller/trafficcontroller.go:180      create namespace default
2021-08-15T14:29:46.384+08:00   INFO    trafficcontroller/trafficcontroller.go:188      create http server default/server-demo
2021-08-15T14:31:06.051+08:00   INFO    trafficcontroller/trafficcontroller.go:422      create http pipeline default/pipeline-demo
2021-08-15T15:15:04.523+08:00   INFO    cluster/cluster.go:744  defrag successfully

```

3. Create an HTTPServer and Pipeline
```shell
$ echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: pipeline-demo' | egctl object create
```

```shell
$ echo '
name: pipeline-demo
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://172.20.97.112:9095
      loadBalance:
        policy: roundRobin' | egctl object create
```

4. test
```shell
curl http://172.20.97.117:10080/pipeline
```

**ab test**
```shell
yum -y install httpd-tools
```

## stress test
**ab --> Echo HTTPSvr**
```shell
[root@iZj6cc0krt5qubycizmku9Z ~]# ab -c 300 -n 100000 http://172.20.97.112:9095/pipeline

Concurrency Level:      300
Time taken for tests:   6.370 seconds
Complete requests:      100000
Failed requests:        0
Total transferred:      24500000 bytes
HTML transferred:       12700000 bytes
Requests per second:    15698.81 [#/sec] (mean)
Time per request:       19.110 [ms] (mean)
Time per request:       0.064 [ms] (mean, across all concurrent requests)
Transfer rate:          3756.06 [Kbytes/sec] received

```

**ab --> Nginx --> Echo HTTPSvr**

```shell
[root@iZj6cc0krt5qubycizmku9Z ~]# ab -c 300 -n 100000 http://172.20.97.117:8080/pipeline

Concurrency Level:      300
Time taken for tests:   7.655 seconds
Complete requests:      100000
Failed requests:        0
Total transferred:      37800000 bytes
HTML transferred:       21900000 bytes
Requests per second:    13064.07 [#/sec] (mean)
Time per request:       22.964 [ms] (mean)
Time per request:       0.077 [ms] (mean, across all concurrent requests)
Transfer rate:          4822.48 [Kbytes/sec] received

```

**ab --> Easegress --> Echo HTTPSvr** 
```shell

[root@iZj6cc0krt5qubycizmku9Z ~]# ab -c 300 -n 100000 http://172.20.97.117:10080/pipeline

Concurrency Level:      300
Time taken for tests:   10.259 seconds
Complete requests:      100000
Failed requests:        0
Total transferred:      27300000 bytes
HTML transferred:       15500000 bytes
Requests per second:    9747.49 [#/sec] (mean)
Time per request:       30.777 [ms] (mean)
Time per request:       0.103 [ms] (mean, across all concurrent requests)
Transfer rate:          2598.70 [Kbytes/sec] received

```