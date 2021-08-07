# Workflow

- [Workflow](#workflow)
  - [Background](#background)
  - [Sequence workflow with HTTPTextTemplate](#sequence-workflow-with-httptexttemplate)
  - [Example](#example)
    - [Step 1: Create NBA API pipeline](#step-1-create-nba-api-pipeline)
    - [Step 2: Create Fun translator API pipeline](#step-2-create-fun-translator-api-pipeline)
    - [Step 3: Create orchestrion pipeline](#step-3-create-orchestrion-pipeline)
    - [Step 4: Create HTTPServer for routing](#step-4-create-httpserver-for-routing)
    - [Step 5: See the result](#step-5-see-the-result)
  - [References](#references)

## Background

* A workflow consists of an orchestrated and repeatable pattern of activity, enabled by the systematic organization of resources into processes that transform materials, provide services, or process information. It can be depicted as a sequence of operations, the work of a person or group, the work of an organization of staff, or one or more simple or complex mechanisms.[1]
* Easegress currently has a build-in sequence workflow in Pipeline. Furthermore, we also provide a template mechanism for more advanced usage.

## Sequence workflow with HTTPTextTemplate

* Already executed filter's metadata can be the input for next filter.
* Orchestrating pipeline with APIAggregator, RequestAdaptor, and ResponseAdaptor.


## Example
* We use the free, fun, and open RESTful APIs to achieve this example.[2]
* API1: NBA list, http://www.balldontlie.io/api/v1/players, its response is a list for all player's informactions.
```
{
   "data":[
      {
         "height_inches":null,
         "last_name":"Anigbogu",
         "position":"C",
         "team":{
            "id":12,
            "abbreviation":"IND",
            "city":"Indiana",
            "conference":"East",
            "division":"Central",
            "full_name":"Indiana Pacers",
            "name":"Pacers"
         },
         "weight_pounds":null,
         "id":14,
         "first_name":"Ike",
         "height_feet":null
      },
      {
         "last_name":"Baker",
         "position":"G",
         "team":{
            "id":20,
            "abbreviation":"NYK",
            ...
         },
         ...
      },
   ]
}
```
* API2: Fun translator, http://api.funtranslations.com/translate/minion.json, its response body will be liked:
```json
{
    "success": {
        "total": 1
    },
    "contents": {
        "translated": "yo yo bada pik prompo",
        "text": "yo yo check it now",
        "translation": "minion"
    }
}%  
```
* Yes, we love Minions!

* We want to orchestrate these two APIs in one request, furthermore, we will take NBA API's response's fifth player's last name and combined it into a sentence for fun translater API's to translate into `minion` language.

* In Easegress, a pipeline usually represents a particular HTTP service(maybe with several backends), APIAggregator can forward the request to a dedicated pipeline. And we can use HTTPTextTemplate syntax to extract the responses and turn them into the input for the next pipeline with Aggregator.

### Step 1: Create NBA API pipeline 

``` bash  
echo '
name: pipeline-nba
kind: HTTPPipeline
flow:
  - filter: requestAdp
  - filter: proxy
filters:
  - kind: RequestAdaptor
    name: requestAdp
    host: www.balldontlie.io
    path:
      replace: /api/v1/players
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://www.balldontlie.io
      loadBalance:
        policy: roundRobin' | egctl object create 
```

### Step 2: Create Fun translator API pipeline 

* This pipeline uses a `requestAdaptor` to change the request method to `POST`, replace its path to `/translate/minion.json`, and add a `Content-Type` header.

``` bash

echo '
name: pipeline-translate
kind: HTTPPipeline
flow:
  - filter: requestAdp
  - filter: proxy
filters:
  - kind: RequestAdaptor
    name: requestAdp
    host: api.funtranslations.com 
    header:
      del: []
      set:
      add:
        Content-Type: application/x-www-form-urlencoded
    method: "POST"
    path:
      replace: /translate/minion.json 
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://api.funtranslations.com
      loadBalance:
        policy: roundRobin' | egctl object create 

```

### Step 3: Create orchestrion pipeline 

* This pipeline needs to get the fifth player's last name as the input body for the translator pipeline. It achieves this goal by using `requestAdaptor` and the build-in `HTTPTemplate` in the pipeline.
* At last, this pipeline combines the output from NBA API and Fun translator API to form a new response.

``` bash 
echo '
name: pipeline-agg
kind: HTTPPipeline
flow:
  - filter: agg-demo
  - filter: req-adaptor1
  - filter: agg-demo1
  - filter: rsp-adaptor

filters:
  - pipelines:
      - name: pipeline-nba
    kind: APIAggregator
    mergeResponse: true
    name: agg-demo
  - name: req-adaptor1
    kind: RequestAdaptor
    header:
      del: [] 
      set: 
      add: 
        Content-Type: application/x-www-form-urlencoded
    body: "text=hi my name is [[filter.agg-demo.rsp.body.data.4.last_name]] yoyo check it now" 
  - pipelines:
      - name: pipeline-translate
    kind: APIAggregator
    name: agg-demo1
    mergeResponse: true
  - name: rsp-adaptor
    kind: ResponseAdaptor
    header:
      del: [] 
      set: 
        last_name: "[[filter.agg-demo.rsp.body.data.4.last_name]]" 
      add: 
    body: "{\"name\": \"[[filter.agg-demo.rsp.body.data.4.last_name]]\",\"translated\":\"[[filter.agg-demo1.rsp.body.contents.translated]]\", \"origin\":\"[[filter.agg-demo1.rsp.body.contents.text]]\", \"language\":\"[[filter.agg-demo1.rsp.body.contents.translation]]\"}" ' | egctl  object create 

```

### Step 4: Create HTTPServer for routing

``` bash
echo '
kind: HTTPServer
name: server-demo
certs:
keys:
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /workflow
      backend: pipeline-agg ' | egctl  object create 

```

### Step 5: See the result 

``` bash
$ curl http://127.0.0.1:10080/workflow -vv
*   Trying 127.0.0.1...
* TCP_NODELAY set
* Connected to 127.0.0.1 (127.0.0.1) port 10080 (#0)
> GET /workflow HTTP/1.1
> Host: 127.0.0.1:10080
> User-Agent: curl/7.64.1
> Accept: */*
> 
< HTTP/1.1 200 OK
< Last_name: Brown
< Date: Wed, 04 Aug 2021 09:28:07 GMT
< Content-Length: 144
< Content-Type: text/plain; charset=utf-8
< 
{"name": "Brown","translated":"hi mi nomba tis nub yoyo bada pik prompo", "origin":"hi my name is Brown yoyo check it now", "language":"minion"}

```


* `filter.agg-demo.rsp.body.data.4.last_name` in rsp-adaptor will extract fifth player's last name fron NBA API's response body.
* `filter.agg-demo1.rsp.body.contents.translated` in rsp-adaptor will extract the translated result in minion language.
* The template syntax above supports GJSON[3] in the last field.

## References

1. https://en.wikipedia.org/wiki/Workflow
2. https://learn.vonage.com/blog/2021/03/15/the-ultimate-list-of-fun-apis-for-your-next-coding-project/ 
3. https://github.com/tidwall/gjson
