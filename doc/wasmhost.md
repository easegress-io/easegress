# WasmHost

- [WasmHost](#wasmhost)
  - [Write business logic and compile it to Wasm](#write-business-logic-and-compile-it-to-wasm)
  - [Create a WasmHost](#create-a-wasmhost)
  - [Test](#test)
  - [Hot Update](#hot-update)
  - [The Return Value of the Wasm Code](#the-return-value-of-the-wasm-code)

The WasmHost is a filter of Easegress which can be orchestrated into a pipeline. But while the behavior of all other filters are defined by filter developers and can only be fine-tuned by configuration, this filter implements a host environment for user-developed [WebAssembly](https://webassembly.org/) code, which enables users to control the filter behavior completely.

## Write business logic and compile it to Wasm

It is possible to write Wasm code in [text format](https://webassembly.github.io/spec/core/text/index.html), but this is painful.

To make things easier, MegaEase released some SDKs to help users to develop their logic in high-level languages, SDKs for the following languages are available currently:

* [AssemblyScript](https://github.com/megaease/easegress-assemblyscript-sdk)

We can follow the documentation of these SDKs to write business logic and compile it to Wasm.

Take AssemblyScript as an example, suppose our business logic is:

* Get the value of request header 'Foo'
* If the length of the value is greater than 10, log a warning message
* If the header exists, copy its value to a new header named 'Wasm-Added'
* Set the request body to 'I have a new body now'

The corresponding AssemblyScript code would be:

```typescript
function wasm_run(): i32 {
	let v = request.getHeader( "Foo" )
	if( v.length > 10 ) {
		log( LogLevel.Warning, "The length of Foo is greater than 10" )
	}
	if( v.length > 0 ) {
		request.addHeader( "Wasm-Added", v )
	}
	request.setBody( String.UTF8.encode("I have a new body now") )
	return 0;
}
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
      backend: wasm-pipeline' | egctl object create
```

And then create the pipeline `wasm-pipeline` which includes a `WasmHost` filter:

```bash
$ echo '
name: wasm-pipeline
kind: HTTPPipeline
flow:
  - filter: wasm
  - filter: proxy
    jumpIf: { fallback: END }

filters:
  - name: wasm
    kind: WasmHost
    maxConcurrency: 2
    code: /home/megaease/demo.wasm
    timeout: 100ms
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin' | egctl object create
```

Note we are using the path of the Wasm file as the value of `code` in the spec of `WasmHost`, but the value of `code` can also be a URL (HTTP/HTTPS) or the base64 encoded Wasm code.

Then, we need to set up the backend service by following [the steps in README.md](../README.md#test).

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

## Hot Update

The Wasm code can be hot updated without restart Easegress with below command:

```bash
$ egctl wasm update
```

This sends a notification to all `WasmHost` instances, and they will reload their Wasm code if the code was modified.

## The Return Value of the Wasm Code

From the [developer guide](./developer-guide.md#jumpif-mechanism-in-pipeline), we know the pipeline supports a `JumpIf` mechanism, which directs the pipeline to jump to another filter according to the result of the current filter.

This mechanism requires we know all the possible results of a filter before we define a pipeline, but for `WasmHost`, there's a difficulty with this: business logic is developed by users after the development of `WasmHost` filter, that's the filter has no idea about what result it should return.

The solution is `WasmHost` defines 10 results, an empty string, and `wasmResult1` to `wasmResult9`. Same as all other filters, the empty string means everything is fine, while the meaning of the other 9 results is defined by the user. 

And as a requirement, user-developed business logic must return an integer in range `[0, 9]`, the `WasmHost` convert `0` to the empty string, and `1` - `9` to `wasmResult1` - `wasmResult9` respectively. Users could leverage these results to define the `JumpIf`s of a pipeline.
