Ease Gateway HTTP Mux Reference
===============================

## Summary

There are 2 built-in mux totally in Ease Gateway current release. Mux is the changeable router running on [HTTP Server Plugin](./plugin_ref.md#http-server-plugin). Every [HTTP Input Plugin](./plugin_ref.md#http-input-plugin) attaching to the corresponding HTTP Sever registers entries on the mux.
The `Mux Type` is the config field `mux\_type` of HTTP Server Plugin, and the corresponding mux config is the config filed `mux\_config` of HTTP Server Plugin.

| Mux type | Development status | Link |
|:--|:--:|:--|
| regexp | GA | [code](../src/plugins/http_re_mux.go) |
| param | GA | [code](../src/plugins/http_param_mux.go) |

## Regexp Mux
Mux runs a router based on regular expression matching with priority from higher to lower.

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| cache\_key\_complete | bool | The flag represents if the regexp mux cache uses complete request URL as the cache key. True means from scheme to fragment, false means from scheme to path. | Functionality | Yes | false |
| cache\_max\_count | uint32 | Under mux type being regexp, the max cache count the mux to keep. Zero value means no limit. | Functionality | Yes | 1024 |

### Configure Supported in HTTP Input Plugin
- scheme
- host
- port
- path
- query
- fragment
- priority
- methods


## Param Mux

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|

### Configure Supported in HTTP Input Plugin
- path
- methods

> NOTICE: Currently param mux does not support `scheme`, `host`, `port`, `query`, `fragment`, `priority`.
