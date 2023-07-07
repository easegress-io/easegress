# WasmHost

- [WasmHost](#wasmhost)
  - [Write business logic and compile it to Wasm](#write-business-logic-and-compile-it-to-wasm)
  - [Create a WasmHost](#create-a-wasmhost)
  - [Test](#test)
  - [Parameters](#parameters)
  - [Sharing Data](#sharing-data)
  - [Hot Update](#hot-update)
  - [The Return Value of the Wasm Code](#the-return-value-of-the-wasm-code)
  - [Benchmark](#benchmark)

The WasmHost is a filter of Easegress which can be orchestrated into a pipeline. But while the behavior of all other filters are defined by filter developers and can only be fine-tuned by configuration, this filter implements a host environment for user-developed [WebAssembly](https://webassembly.org/) code, which enables users to control the filter behavior completely.

## Write business logic and compile it to Wasm

It is possible to write Wasm code in [text format](https://webassembly.github.io/spec/core/text/index.html), but this is painful.

To make things easier, MegaEase and the community released SDKs to help users to develop their logic in high-level languages, SDKs for the following languages are available currently:

* [AssemblyScript](https://github.com/megaease/easegress-assemblyscript-sdk)
* [Golang](https://github.com/xmh19936688/easegress-go-sdk) (developed by @[xmh19936688](https://github.com/xmh19936688))
* [Rust](https://github.com/megaease/easegress-rust-sdk)

We can follow the documentation of these SDKs to write business logic and compile it to Wasm.

Take AssemblyScript as an example, suppose our business logic is:

* Get the value of request header 'Foo'
* If the length of the value is greater than 10, log a warning message
* If the header exists, copy its value to a new header named 'Wasm-Added'
* Set the request body to 'I have a new body now'

The corresponding AssemblyScript code would be:

```typescript
export * from '../easegress/proxy'
import { Program, request, LogLevel, log, registerProgramFactory } from '../easegress'

class AddHeaderAndSetBody extends Program {
    constructor(params: Map<string, string>) {
        super(params)
    }

    run(): i32 {
        super.run()
        let v = request.getHeader( "Foo" )
        if( v.length > 10 ) {
            log( LogLevel.Warning, "The length of Foo is greater than 10" )
        }
        if( v.length > 0 ) {
            request.addHeader( "Wasm-Added", v )
        }
	      request.setBody( String.UTF8.encode("I have a new body now") )
        return 0
    }
}

registerProgramFactory((params: Map<string, string>) => {
    return new AddHeaderAndSetBody(params)
})
```

And in the rest of this document, we assume the result Wasm file is save at `/home/megaease/demo.wasm`.

## Create a WasmHost

Before doing this, we should have Easegress set up by following [these steps in README.md](../README.md#setting-up-easegress).

Let's first create an HTTPServer listening on port 10080 to handle the HTTP traffic:

```bash
$ echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: wasm-pipeline' | egctl create -f -
```

And then create the pipeline `wasm-pipeline` which includes a `WasmHost` filter:

```bash
$ echo '
name: wasm-pipeline
kind: Pipeline
flow:
  - filter: wasm
  - filter: proxy

filters:
  - name: wasm
    kind: WasmHost
    maxConcurrency: 2
    code: /home/megaease/demo.wasm
    timeout: 100ms
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin' | egctl create -f -
```

Note we are using the path of the Wasm file as the value of `code` in the spec of `WasmHost`, but the value of `code` can also be a URL (HTTP/HTTPS) or the base64 encoded Wasm code.

Then, we need to set up the backend service by following command.

```bash
go run easegress/example/backend-service/echo/echo.go
```
## Test

Now, let's send some requests to the HTTP server, we can see request header and body are set as desired.

```bash
$ curl http://127.0.0.1:10080/pipeline
Your Request
==============
Method: GET
URL   : /pipeline
Header:
    User-Agent: [curl/7.68.0]
    Accept: [*/*]
    Accept-Encoding: [gzip]
Body  : i have a new body now

$ curl http://127.0.0.1:10080/pipeline -HFoo:hello
Your Request
==============
Method: GET
URL   : /pipeline
Header:
    Accept: [*/*]
    Foo: [hello]
    Wasm-Added: [hello]
    Accept-Encoding: [gzip]
    User-Agent: [curl/7.68.0]
Body  : i have a new body now
```

## Parameters

From the example AssemblyScript code above, you may already notice that the `constructor` takes a parameter `params`, which is a map of string to string. The key/value pairs are parameters of the WebAssembly program, they can be set in the filter configuration.

```yaml
filters:
  - name: wasm
    kind: WasmHost
    parameters:               # +
      key1: "value1"          # +
      key2: "value2"          # +
```

And then read out by WebAssembly:

```typescript
	constructor(params: Map<string, string>) {
		super(params)
		this.key1 = params.get("key1")
		this.key2 = params.get("key2")
	}
```

## Sharing Data

When the `maxConcurrency` field of a WasmHost filter is larger than `1`, or when Easegress is deployed as a cluster, a single WasmHost filter could have more than one `Wasm Virtual Machine`s. Because safety is the design principle of WebAssembly, these VMs are isolated and can not share data with each other.

But sometimes, sharing data is useful, Easegress provides APIs for this:

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, cluster, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class SharedData extends Program {
	run(): i32 {
		let counter1 = cluster.AddInteger("counter1", 1)
		let counter2 = cluster.AddInteger("counter2", 2)
    // ...
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new SharedData(params)
})
```

The above code creates two shared counters for all VMs, `counter1` increases `1` for each request while `counter2` increases `2` for each request.

We can view the shared data with:

```bash
$ egctl wasm list-data wasm-pipeline wasm
counter1: "3"
counter2: "6"
```

where `wasm-pipeline` is the pipeline name and `wasm` is the filter name.

The shared data can be modified with:

```bash
$ echo '
counter1: 100
counter2: 101' | egctl wasm apply-data wasm-pipeline wasm
```

And can be deleted with:

```bash
$ egctl wasm delete-data wasm-pipeline wasm
```
## Hot Update

The Wasm code can be hot updated without restart Easegress with below command:

```bash
$ egctl wasm apply-data --reload-code
```

This sends a notification to all `WasmHost` instances, and they will reload their Wasm code if the code was modified.

## The Return Value of the Wasm Code

From the [developer guide](./developer-guide.md#jumpif-mechanism-in-pipeline), we know the pipeline supports a `JumpIf` mechanism, which directs the pipeline to jump to another filter according to the result of the current filter.

This mechanism requires we know all the possible results of a filter before we define a pipeline, but for `WasmHost`, there's a difficulty with this: business logic is developed by users after the development of `WasmHost` filter, that's the filter has no idea about what result it should return.

The solution is `WasmHost` defines 10 results, an empty string, and `wasmResult1` to `wasmResult9`. Same as all other filters, the empty string means everything is fine, while the meaning of the other 9 results is defined by the user. 

And as a requirement, user-developed business logic must return an integer in range `[0, 9]`, the `WasmHost` convert `0` to the empty string, and `1` - `9` to `wasmResult1` - `wasmResult9` respectively. Users could leverage these results to define the `JumpIf`s of a pipeline.

## Benchmark

The WasmHost filter executes user code, so its performance depends on the complexity of user code, which is not easy to measure.

In this section, the `WasmHost` filter is made to simulate a `Mock` filter, and we will compare its performance statistics against a real `Mock` filter. Below is the AssemblyScript code for the WasmHost filter:

```typescript
export * from '../easegress/proxy'
import { Program, response, registerProgramFactory } from '../easegress'

class Mock extends Program {
    constructor(params: Map<string, string>) {
        super(params)
    }

    run(): i32 {
        super.run()
        response.setStatusCode(200)
	      response.setBody( String.UTF8.encode("hello wasm\n") )
        return 0
    }
}

registerProgramFactory((params: Map<string, string>) => {
    return new Mock(params)
})
```

The HTTP server configuration is:

```bash
$ echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
- paths:
  - pathPrefix: /wasm
    backend: wasm-pipeline
  - pathPrefix: /mock
    backend: mock-pipeline' | egctl create -f -
```

The `wasm-pipeline` configuration is (we will adjust the value of `maxConcurrency` during the test):

```bash
$ echo '
name: wasm-pipeline
kind: Pipeline
flow:
- filter: wasm
filters:
- name: wasm
  kind: WasmHost
  maxConcurrency: 2
  code: /home/megaease/demo.wasm
  timeout: 100ms' | egctl create -f -
```

The `mock-pipeline` configuration is:

```bash
$ echo '
name: mock-pipeline
kind: Pipeline
flow:
- filter: mock
filters:
- name: mock
  kind: Mock
  rules:
  - body: "hello mock\n"
    code: 200' | egctl create -f -
```

* **Scenario 1**: concurrency: 10; duration: 1 miniute

``` bash
./hey -c 10 -z 1m http://127.0.0.1:10080/{wasm|mock}
```

| Filter (maxConcurrency) | Slowest | Fastest | Average | RPS   | 90% Latency | 95% Latency | 99% Latency |
| ----------------------- | ------- | ------- | ------- | ----- | ----------- | ----------- | ----------- |
| Mock (N/A)              | 0.0211s | 0.0001s | 0.0006s | 32983 | 0.0005s     | 0.0005s     | 0.0010s     |
| WasmHost (2)            | 0.0402s | 0.0001s | 0.0006s | 20127 | 0.0007s     | 0.0008s     | 0.0012s     |
| WasmHost (100)          | 0.0246s | 0.0001s | 0.0006s | 25654 | 0.0006s     | 0.0007s     | 0.0014s     |
| WasmHost (200)          | 0.0257s | 0.0001s | 0.0006s | 25902 | 0.0006s     | 0.0007s     | 0.0012s     |

* **Scenario 2**: concurrency: 100; duration: 1 miniute

``` bash
./hey -c 100 -z 1m http://127.0.0.1:10080/{wasm|mock}
```
| Filter (maxConcurrency) | Slowest | Fastest | Average | RPS   | 90% Latency | 95% Latency | 99% Latency |
| ----------------------- | ------- | ------- | ------- | ----- | ----------- | ----------- | ----------- |
| Mock (N/A)              | 0.0644s | 0.0001s | 0.0060s | 43536 | 0.0039s     | 0.0056s     | 0.0116s     |
| WasmHost (2)            | 0.0254s | 0.0001s | 0.0060s | 20827 | 0.0056s     | 0.0064s     | 0.0095s     |
| WasmHost (100)          | 0.0707s | 0.0001s | 0.0060s | 32263 | 0.0061s     | 0.0088s     | 0.0164s     |
| WasmHost (200)          | 0.0659s | 0.0001s | 0.0060s | 31287 | 0.0064s     | 0.0090s     | 0.0167s     |

* **Scenario 3**: concurrency: 200; duration: 1 miniute

``` bash
./hey -c 200 -z 1m http://127.0.0.1:10080/{wasm|mock}
```

| Filter (maxConcurrency) | Slowest | Fastest | Average | RPS   | 90% Latency | 95% Latency | 99% Latency |
| ----------------------- | ------- | ------- | ------- | ----- | ----------- | ----------- | ----------- |
| Mock (N/A)              | 0.0767s | 0.0001s | 0.0120s | 43789 | 0.0086s     | 0.0123s     | 0.0216s     |
| WasmHost (2)            | 0.0308s | 0.0001s | 0.0120s | 20739 | 0.0112s     | 0.0129s     | 0.0172s     |
| WasmHost (100)          | 0.0913s | 0.0001s | 0.0120s | 32270 | 0.0126s     | 0.0174s     | 0.0285s     |
| WasmHost (200)          | 0.0900s | 0.0001s | 0.0120s | 31514 | 0.0131s     | 0.0179s     | 0.0298s     |
