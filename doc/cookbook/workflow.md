# Workflow

- [Workflow](#workflow)
  - [Background](#background)
  - [Example](#example)
    - [Step 1. Create a Slack Webhook](#step-1-create-a-slack-webhook)
    - [Step 2. Create a pipeline for the workflow](#step-2-create-a-pipeline-for-the-workflow)
    - [Step 3: Create an HTTPServer to receive client request](#step-3-create-an-httpserver-to-receive-client-request)
    - [Step 4: See the result](#step-4-see-the-result)
  - [References](#references)

## Background

A workflow consists of an orchestrated and repeatable pattern of activity, enabled by the systematic organization of resources into processes that transform materials, provide services, or process information. It can be depicted as a sequence of operations, the work of a person or group, the work of an organization of staff, or one or more simple or complex mechanisms.[1]

## Example

Read an RSS feed, build the article list into a Slack message, and then send it to Slack.

### Step 1. Create a Slack Webhook

Please follow [this document](https://api.slack.com/messaging/webhooks) to create a new Slack WebHook, the URL of the Webhook will be like `https://hooks.slack.com/services/T0XXXXXXX/B0YYYYYYYYY/ZZZZZZZZZZZZZZZZZZZZZZZZ`.

### Step 2. Create a pipeline for the workflow

Save the below YAML to `rss-pipeline.yaml`, and make sure you have replaced the Slack Webhook URL with yours.

```yaml
name: rss-pipeline
kind: Pipeline

flow:
# validate the request, a valid request must contain the 'X-Rss-Url' header, and its value must be a URL.
- filter: validator

# create the request for the RSS feed.
- filter: buildRssRequest
  namespace: rss

# read the RSS feed, a 3rd party website is used to covert the feed from XML to JSON.
- filter: sendRssRequest
  namespace: rss

# the RSS feed is gzipped, we need to decompress it.
- filter: decompressResponse
  namespace: rss

# create the request to Slack (build the Slack message).
- filter: buildSlackRequest
  namespace: slack

# send the message to Slack.
- filter: sendSlackRequest
  namespace: slack

# build the response for the client.
- filter: buildResponse

filters:
- name: validator
  kind: Validator
  headers:
    "X-Rss-Url":
       regexp: ^https?://.+$

- name: buildRssRequest
  kind: RequestBuilder
  template: |
    url: /developers/feed2json/convert?url={{index (index .requests.DEFAULT.Header "X-Rss-Url") 0 | urlquery}}

- name: sendRssRequest
  kind: Proxy
  pools:
  - loadBalance:
      policy: roundRobin
    servers:
    - url: https://www.toptal.com
  compression:
    minLength: 4096

- name: buildSlackRequest
  kind: RequestBuilder
  template: |
    method: POST
    url: /services/T0XXXXXXXXX/B0YYYYYYY/ZZZZZZZZZZZZZZZZZZZZ   # This the Slack webhook address, please change it to your own.
    body: |
      {
         "text": "Recent posts - {{.responses.rss.JSONBody.title}}",
         "blocks": [{
            "type": "section",
            "text": {
              "type": "plain_text",
              "text": "Recent posts - {{.responses.rss.JSONBody.title}}"
            }
         }, {
            "type": "section",
            "text": {
              "type": "mrkdwn",
              "text": "{{range $index, $item := .responses.rss.JSONBody.items}}• <{{$item.url}}|{{$item.title}}>\n{{end}}"
         }}]
      }

- name: sendSlackRequest
  kind: Proxy
  pools:
  - loadBalance:
      policy: roundRobin
    servers:
    - url: https://hooks.slack.com
  compression:
    minLength: 4096


- name: decompressResponse
  kind: ResponseAdaptor
  decompress: gzip


- name: buildResponse
  kind: ResponseBuilder
  template: |
    statusCode: {{.responses.slack.StatusCode}}
    body: {{if eq .responses.slack.StatusCode 200}}RSS feed has been sent to Slack successfully.{{else}}Failed to send the RSS feed to Slack{{end}}
```

Then create the RSS pipeline with the command:

```shell
egctl object create -f rss-pipeline.yaml
```

### Step 3: Create an HTTPServer to receive client request

Save below YAML to `http-server.yaml`.

```yaml
kind: HTTPServer
name: http-server-example
port: 8080
https: false
keepAlive: true
keepAliveTimeout: 75s
maxConnection: 10240
cacheSize: 0
rules:
  - paths:
    - pathPrefix: /rss
      backend: rss-pipeline
```

Then create the HTTP server with command:

```shell
egctl object create -f http-server.yaml
```

### Step 4: See the result 

Execute the below command and your Slack will receive the article list if everything is correct.
You may use another RSS feed, but please note the maximum message size Slack allowed is about 3K, so you will need to limit the number of articles returned by the RSS feed of some sites(e.g. Hack News)

```shell
$ curl -v http://127.0.0.1:8080/rss -H X-Rss-Url:https://www.coolshell.cn/rss
*   Trying 127.0.0.1:8080...
* TCP_NODELAY set
* Connected to 127.0.0.1 (127.0.0.1) port 8080 (#0)
> GET /rss HTTP/1.1
> Host: 127.0.0.1:8080
> User-Agent: curl/7.68.0
> Accept: */*
> X-Rss-Url:https://www.coolshell.cn/rss
> 
* Mark bundle as not supporting multiuse
< HTTP/1.1 200 OK
< Date: Fri, 17 Jun 2022 08:29:58 GMT
< Content-Length: 45
< Content-Type: text/plain; charset=utf-8
< 
* Connection #0 to host 127.0.0.1 left intact
RSS feed has been sent to Slack successfully.
```

## References

1. https://en.wikipedia.org/wiki/Workflow
