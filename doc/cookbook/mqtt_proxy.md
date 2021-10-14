# MQTT Proxy

- [MQTT Proxy](#mqtt-proxy)
  - [Background](#background)
  - [Design](#design)
    - [Match different topic mapping policy](#match-different-topic-mapping-policy)
    - [Detail of single policy](#detail-of-single-policy)
  - [References](#references)



# Background
- MQTT is a standard messaging protocol for IoT (Internet of Things) which is extremely lightweight and used by a wide variety of industries.
- By supporting MQTT Proxy in Easegress, MQTT clients can produce messages to Kafka backend directly.
- We also provide the HTTP endpoint to allow the backend to send messages to MQTT clients.

# Design
- `MQTTProxy` is now a `BusinessController` to Easegress. 
- Use `github.com/eclipse/paho.mqtt.golang/packets` to parse MQTT packet. `paho.mqtt.golang` is a MQTT 3.1.1 go client introduced by Eclipse Foundation (who also introduced the most widely used MQTT broker mosquitto).
- As a MQTT proxy, we now support MQTT clients to `publish` messages to backend Kafka with a powerful topic mapper to map multi-level MQTT topics to Kafka topics with headers (Details in following).
- We also support MQTT clients to `subscribe` topics (wildcard is supported) and send messages back to the MQTT clients through the HTTP endpoint.

```
             publish msg                       topic mapper
MQTT client ------------> Easegress MQTTProxy ------------> Kafka

all published msg will go to Kafka, will not send to other MQTT clients. 
             
             subscribe msg                                 
MQTT client <---------------- Easegress MQTT HTTP Endpoint <---- Backend

all msg send back to MQTT clients come from HTTP endpoint. 
```
- We assume that IoT devices (use MQTT client) report their status to the backend (through Kafka), and backend process these messages and send instructions back to IoT devices.

# Example 
Save following yaml to file `mqttproxy.yaml` and then run 
```bash 
egctl object create -f mqttproxy.yaml
```
```yaml
kind: MQTTProxy
name: mqttproxy
port: 1883  # tcp port for mqtt clients to connect 
backendType: Kafka
kafkaBroker:
  backend: ["123.123.123.123:9092", "234.234.234.234:9092"]
useTLS: true
certificate:
  - name: cert1
    cert: balabala
    key: keyForbalabala
  - name: cert2
    cert: foo
    key: bar
auth:
  # username and password for mqtt clients to connect broker (from MQTT protocol) 
  - userName: test
    passBase64: dGVzdA==
  - userName: admin
    passBase64: YWRtaW4=
topicMapper:
  # map MQTT multi-level topic to Kafka topic with headers
  # detail described in following 
  # if topicMapper is None, no map will happen and Kafka topic will be the MQTT topic 
  matchIndex: 0
  route: 
    - name: gateway
      matchExpr: "gate*"
    - name: direct
      matchExpr: "dir*"
  policies:
    - name: direct
      topicIndex: 1
      route: 
      - topic: iot_phone
        exprs: ["iphone", "xiaomi", "oppo", "pixel"]
      - topic: iot_other
        exprs: [".*"]  
      headers: 
      0: direct
      1: device
      2: status
    - name: gateway
      topicIndex: 3
      route: 
      - topic: iot_phone
        exprs: ["iphone", "xiaomi", "oppo", "pixel"]
      - topic: iot_other
        exprs: [".*"]  
      headers: 
      0: gateway
      1: gatewayID
      2: device
      3: status
```
In MQTT, there are multi-levels in a topic. Topic mapping is used to map MQTT topic to Kafka topic with headers. For example:
```
MQTT multi-level topics:
- beijing/car/123/log
- shanghai/tv/234/status
- nanjing/phone/456/error

with corresponding pattern: 
- loc/device/ID/event

with Topic mapper, may produce Kafka topic: 
- topic: iot_device, headers: {loc: beijing, device: car, ID: 123, event: log} 
- topic: iot_device, headers: {loc: shanghai, device: tv, ID: 234, event: status} 
- topic: iot_device, headers: {loc: nanjing, device: phone, ID: 456, event: error}
```

## Match different topic mapping policy 
Consider there may be multiple schemas for your MQTT topic, so we first provide a router to route your MQTT topic to different mapping policies and then do the map in that policy.

For example,
```
schema1: gateway/gatewayID/device/status
schema2: direct/device/status 

...
    # matchIndex is the MQTT topic level used to match the route policy
    matchIndex: 0 
    route: 
    - name: gateway
        matchExpr: "gate*"
    - name: direct
        matchExpr: "dir*"
...
```
means that we use MQTT topic level 0 to match `matchExpr` to find a corresponding policy. In this case, `gateway/gate123/iphone/log` will match policy `gateway`, `direct/iphone/log` will match policy `direct`. 


## Detail of single policy 
```
  policies:
    - name: direct
      topicIndex: 1
      route: 
      - topic: iot_phone
        exprs: ["iphone", "xiaomi", "oppo", "pixel"]
      - topic: iot_other
        exprs: [".*"]  
      headers: 
      0: direct
      1: device
      2: status
```
`topicIndex` is the MQTT topic level used to produce Kafka topic (Regular expressions supported), in this case, `direct/iphone/...` will produce Kafka topic `iot_phone`, but `direct/car/...` will produce Kafka topic `iot_other`. `headers` used to produce Kafka headers. 

More example about topic mapper: 
```
use yaml above:
MQTT topic: 
pattern1: gateway/gatewayID/device/status -> match policy gateway
pattern2: direct/device/status -> match policy direct

example1: "gateway/gate123/iphone/log"
Kafka 
    topic: iot_phone
    headers: 
        gateway: gateway
        gatewayID: gate123
        device: iphone
        status: log

example2: "direct/xiaomi/status"
Kafka 
    topic: iot_phone
    headers: 
        direct: direct
        device: xiaomi
        status: status

example3: "direct/tv/log"
Kafka 
    topic: iot_other
    headers: 
        direct: direct
        device: tv
        status: log
```
Empty `topicMapper` means there is no map between the MQTT topic and Kafka topic.

# HTTP endpoint
We support the backend to send messages back to MQTT clients through the HTTP endpoint.

API for http endpoint:
- Host: Easegress IP, for example `http://127.0.0.1`
- Port: Easegress API address, by default, `:2381`
- Path: `apis/v1/mqttproxy/{name}/topics/publish`, where name is the name of MQTT proxy
- Method: POST
- Body: 
```json
{
  "topic": "yourTopicName", 
  "qos": 1,
  "payload": "dataPayload",
  "base64": false
}
```
> Note:   Currently, the QoS only support `0` and `1`

To send binary data, you can encode your binary data base64 and send `base64` flag to `true`. Your client will receive the original binary data, we will do the decode. 
- Status code:
  - 200: Success
  - 400: StatusBadRequest, may wrong http method, or wrong data (qos send to illegal number) etc. 

The HTTP endpoint schema also works for multi-node deployment. Say you have 3 Easegress instances called `eg-0`, `eg-1`, `eg-2`, and your MQTT client connects to `eg-0`, if you send messages to `eg-1`, your client will receive the message too.

We also support wildcard subscriptions.
For example, 
```
POST http://127.0.0.1:2381/apis/v1/mqttproxy/mqttproxy/topics/publish
{
  "topic": "Beijing/Phone/Update", 
  "qos": 1, // currently only support 0 and 1
  "payload": "time to update",
  "base64": false
}

the clients subscribe following topics will receive the message:
"Beijing/Phone/Update"
"Beijing/+/Update"
"Beijing/Phone/+"
"+/Phone/Update"
"Beijing/+/+"
"Beijing/#"
"+/+/+"
```

# References 
1. https://github.com/eclipse/paho.mqtt.golang
2. http://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html
