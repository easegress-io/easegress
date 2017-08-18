#!/bin/bash

SCRIPTFILE="$(readlink --canonicalize-existing "$0")"
echo "SCRIPTFILE: ${SCRIPTFILE}"
SCRIPTPATH="$(dirname "$SCRIPTFILE")"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CA_FILE=${SCRIPTPATH}/../../cert/localhost-cert.pem

# collectd
echo -e '18.docker-test-system#20.docker-test-instance#11.52.69.164.6#11.docker-test.GenericJMX-tomcat_servlet.SpringApplication.count-milliseconds-processing 22613 1489377836\n18.docker-test-system#20.docker-test-instance#11.52.69.164.6#11.docker-test.GenericJMX-tomcat_servlet.StagemonitorFileServlet.count-requests 0 1489377836\n18.docker-test-system#20.docker-test-instance#11.52.69.164.6#11.docker-test.GenericJMX-tomcat_servlet.StagemonitorFileServlet.count-errors 0 1489377836' | \
     http --verify ${CA_FILE} -v 'https://localhost:10443/v1/metrics' 'User-Agent: collectd/5.7.0' 'Content-Type: application/x-graphite'

# filebeat
echo '{
    "@timestamp":"2017-03-28T04:30:06.590Z",
    "beat":{
        "hostname":"tourist.local",
        "name":"tourist.local",
        "version":"5.2.3"
    },
    "group":"group..",
    "hostipv4":"hostipv4..",
    "hostname":"hostname..",
    "input_type":"log",
    "message":"hello, world",
    "offset":121,
    "source":"/var/log/test.log",
    "system":"system..",
    "type":"log"
}' | http --verify ${CA_FILE} -v 'https://localhost:10443/v1/logs' 'User-Agent: filebeat/5.2.3' 'Content-Type: application/json'

# easeagent metrics
echo '{
  "@timestamp": 1483618250000,
  "type": "get_jdbc_connection",
  "name": "get_jdbc_connection",
  "url": "jdbc:mysql:||localhost|easeteam-root@localhost",
  "application": "backend",
  "system": "okr",
  "hostname": "OKR",
  "hostipv4": "172.31.5.25",
  "instance": "web1",
  "measurement_start": "1483353731594",
  "count": 22558,
  "m1_rate": 0.07982313850234074,
  "m5_rate": 0.08429964436701744,
  "m15_rate": 0.08493885570690576,
  "mean_rate": 0.08527692632241543,
  "m1_count": 5,
  "m5_count": 26,
  "m15_count": 77,
  "min": 0.019975,
  "max": 0.516932,
  "mean": 0.04709865255859028,
  "median": 0.041128,
  "std": 0.035359128694331554,
  "p25": 0.032736,
  "p75": 0.059182,
  "p95": 0.063017,
  "p98": 0.063017,
  "p99": 0.063017,
  "p999": 0.43349
}' | http  --verify ${CA_FILE} -v 'https://localhost:10443/v1/app_metrics' 'User-Agent: easeagent/0.1.0' 'Content-Type: application/json'

# easeagent requesttrace
echo '{
  "@timestamp": "2017-01-05T12:08:32.353+0000",
  "type": "http_request",
  "name": "Query User Org Root Object By User",
  "system": "okr",
  "application": "backend",
  "hostname": "OKR",
  "hostipv4": "172.31.5.25",
  "instance": "web1",
  "measurement_start": 1483353731594,
  "id": "4e7185db-b17d-49a8-ad44-e6dc1851a39a",
  "callStackJson":"{}"
}' | http  --verify ${CA_FILE} -v 'https://localhost:10443/v1/app_requests' 'User-Agent: easeagent/0.1.0' 'Content-Type: application/json'

# zipkin_spans
echo '[
  {
    "traceId": "8bdb50e525051192",
    "id": "80cd69ab70f8b841",
    "name": "http_recv",
    "parentId": "8bdb50e525051192",
    "timestamp": 1489995012857677,
    "duration": 1041094,
    "annotations": [
      {
        "timestamp": 1489995012858181,
        "value": "sr",
        "endpoint": {
          "serviceName": "okr-backend-gateway",
          "ipv4": "172.18.0.7"
        }
      }
    ],
    "binaryAnnotations": [
      {
        "key": "component",
        "value": "servlet",
        "endpoint": {
          "serviceName": "okr-backend-gateway",
          "ipv4": "172.18.0.7"
        }
      },
      {
        "key": "system",
        "value": "OKR",
        "endpoint": {
          "serviceName": "okr-backend-gateway",
          "ipv4": "172.18.0.7"
        }
      }

    ]
  }
]' | http  --verify ${CA_FILE} -v 'https://localhost:10443/v1/zipkin_spans' 'User-Agent: easeagent/0.1.0' 'Content-Type: application/json'
