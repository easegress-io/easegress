# Regular Expression Mux Test

There are two main properties are attached to regular expression entry in mux:
1. URL Pattern: Scheme, Host, Port, Path, Query, Fragment
2. Priority: Type uint32, smaller number higher priority, can be repeated with the condition.


For nicely read:
- Sequence number added
- Omit query and fragment part for nicely read)
- literal dot `.` has not be escaped to `\.`. In real json config, the `127.0.0.1` should be `"host": "127\\.0\\.0\\.1"` as regular expression.
- Abbr.
  - LP: Literal Prefix

## 1. Different URL Pattern with Same Priority
### 1.1 Pattern
- 1.1.1: `http://127.0.0.1:10080/v1/(?P<service>.+)/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v1/`
  - parameters:
    - service
    - id

- 1.1.2: `http://127.0.0.1:10080/v1/search/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v1/search/`
  - parameters:
    - id

- 1.1.3: `http://127.0.0.1:10080/v2/(?P<service>.+)/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v2/`
  - parameters:
    - service
    - id

- 1.1.4: `http://127.0.0.1:10080/v2/search/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v2/search/`
  - parameters:
    - id

- 1.1.5: `http://127.0.0.1:10080/v3/search/16`
  - LP : `http://127.0.0.1:10080/v3/search/16`
  - parameters:

The conflict will happen at:
- 1.1.1 and 1.1.2
- 1.1.3 and 1.1.4

Other pairs are okay to coexist.

let's enble 1.1.1, 1.1.4 and 1.1.5 for below Request URL match examples.

### 1.2 Requst URL
- 1.2.1: `http://127.0.0.1:9898/v1/search/11`
  - match: false

- 1.2.2: `http://127.0.0.1:10080/v1/search/11`
  - match: 1.1.1
  - parameters values:
    - service: search
    - id: 11

- 1.2.3: `http://127.0.0.1:10080/v2/search/23`
  - match: 1.1.4
  - parameters values:
    - id: 23

- 1.2.4: `http://127.0.0.1:10080/v3/search/23`
  - match: false

- 1.2.5: `http://127.0.0.1:10080/v3/search/16`
  - match: 1.1.5

## 2. Difference Pattern with Different Priority

### 2.1 Pattern
- 2.1.1: `http://127.0.0.1:10080/v1/(?P<service>.+)/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v1/`
  - priority: 1
  - parameters:
    - service
    - id

- 2.1.2: `http://127.0.0.1:10080/v1/search/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v1/search/`
  - priority: 2
  - parameters:
    - id

- 2.1.3: `http://127.0.0.1:10080/v1/(?P<service>.+)/?P<sub_service>.+)`
  - LP : `http://127.0.0.1:10080/v1/`
  - priority: 3
  - parameters:
    - service
    - sub_service

- 2.1.4: `http://127.0.0.1:10080/v2/search/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v2/search/`
  - priority: 4
  - parameters:
    - id

- 2.1.5: `http://127.0.0.1:10080/v2/search/16`
  - LP : `http://127.0.0.1:10080/v2/search/16`
  - priority: 5
  - parameters:

You may notice results:
- 2.1.1 hides all requests satisfy 2.1.2
- 2.1.1 does not hide requests satisfy 2.1.3 except the value of `sub_service` is made up of pure numbers.
- 2.1.4 hides all requests satisfy 2.1.5 (adjust priority could fix it, see examples below)

Let's enable all of these patterns to below Request URL match examples.

### 2.2 Requst URL
- 2.2.1: `http://localhost:10080/v1/search/11`
  - match: false

- 2.2.2: `http://127.0.0.1:10080/v1/search/11`
  - match: 2.1.1 (NOTICE)
  - parameters values:
    - service: search
    - id: 11

- 2.2.3: `http://127.0.0.1:10080/v1/search/order_search`
  - match: 2.1.3 (NOTICE)
  - parameters values:
    - service: service
    - sub_service: order_search

- 2.2.4: `http://127.0.0.1:10080/v2/search/23`
  - match: 2.1.4
  - parameters values:
    - id: 23

- 2.2.5: `http://127.0.0.1:10080/v2/search/16`
  - match: 2.1.4
  - parameters values:
    - id: 16
(NOTICE: Change priority of 2.1.5 to 3 or higher will make this URL match with 2.1.5,
BTW we can't change it to 4 which causes literal prefix conflict with 2.1.4)

## 3. Mix Them Up
### 3.1 Pattern
- 3.1.1: `http://localhost:10080/v1/search/16`
  - LP : `http://localhost:10080/v1/search/16`
  - priority: 0

- 3.1.2: `http://127.0.0.1:10080/v1/search/16`
  - LP : `http://127.0.0.1:10080/v1/search/16`
  - priority: 0

- 3.1.3: `http://127.0.0.1:10080/v1/(?P<service>.+)/(?P<id>\d+)`
  - LP : `http://127.0.0.1:10080/v1/`
  - priority: 1
  - parameters:
    - service
    - id

- 3.1.4: `http://localhost:10080/v1/(?P<service>.+)/(?P<id>\d+)`
  - LP : `http://localhost:10080/v1/`
  - priority: 1
  - parameters:
    - service
    - id

- 3.1.5: `http://127.0.0.1:10080/v1/(?P<service>.+)/?P<sub_service>.+)`
  - LP : `http://127.0.0.1:10080/v1/`
  - priority: 2
  - parameters:
    - service
    - sub_service

### 3.2 Request URL
- 3.2.1: `http://localhost:10080/v1/search/11`
  - match: 3.1.4
  - parameters values:
    - service: search
    - id: 11

- 3.2.2: `http://127.0.0.1:10080/v1/search/11`
  - match: 3.1.3
  - parameters values:
    - service: search
    - id: 11

- 3.2.3: `http://127.0.0.1:10080/v1/search/order_search`
  - match: 3.1.5
  - parameters values:
    - service: search
    - sub_service: order_search

- 3.2.4: `http://127.0.0.1:10080/v1/search/16`
  - match: 3.1.2
  - parameters values:

- 3.2.5: `http://localhost:10080/v1/search/16`
  - match: 3.1.1
  - parameters values:
