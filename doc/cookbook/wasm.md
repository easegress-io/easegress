# WasmHost

- [WasmHost](#wasmhost)
	- [Why Use WasmHost](#why-use-wasmhost)
	- [Examples](#examples)
		- [Basic: Noop](#basic-noop)
		- [Add a New Header](#add-a-new-header)
		- [Add a New Header According to Configuration](#add-a-new-header-according-to-configuration)
		- [Add a Cookie](#add-a-cookie)
		- [Mock Response](#mock-response)
		- [Access Shared Data](#access-shared-data)
		- [Return a Result Other Than 0](#return-a-result-other-than-0)

The `WasmHost` is a filter of Easegress which can be orchestrated into a pipeline. But while the behavior of all other filters are defined by filter developers and can only be fine-tuned by configuration, this filter implements a host environment for user-developed [WebAssembly](https://webassembly.org/) code, which enables users to control the filter behavior completely.

## Why Use WasmHost

* **Zero Down Time**: filter behavior can be modified by a hot update.
* **Fast Develop, Fast Deploy**: we believe everyone can be a software developer, and you know your requirement better, no need to wait for MegaEase.
* **Every Thing Under Control**: the filter behavior is right under your fingers.
* **Develop with Your Favour Language**: choose one from `AssemblyScript`, `Go`, `C/C++`, `Rust`, and so on at your wish (Note: a language-specific SDK is required, we are working on this).

## Examples

**Note**: The `WasmHost` filter is disabled by default, to enable it, you need to build Easegress with the below command:

```bash
$ make build_server GOTAGS=wasmhost
```

We will use [AssemblyScript](https://www.assemblyscript.org/) as the language of the examples, please ensure a recent version of [Git](https://git-scm.com/), [Golang](https://golang.org), [Node.js](https://nodejs.org/) and its package manager [npm](https://www.npmjs.com/) are installed before continue. Basic knowledge about writing and working with TypeScript modules, which is very similar to AssemblyScript, is a plus.

### Basic: Noop

The AssemblyScript code of this example is just a noop. But this example includes all the required steps to write, build and deploy WebAssembly code, and it shows the basic structure of the code. All latter examples are based on this one.

1. Clone git repository `easegress-assemblyscript-sdk` to someware on disk

	```bash
	$ git clone https://github.com/megaease/easegress-assemblyscript-sdk.git
	```

2. Switch to a new directory and initialize a new node module:

	```bash
	npm init
	```

3. Install the AssemblyScript compiler using npm, assume that the compiler is not required in production and make it a development dependency:

	```bash
	npm install --save-dev assemblyscript
	```

4. Once installed, the compiler provides a handy scaffolding utility to quickly set up a new AssemblyScript project, for example in the directory of the just initialized node module:

	```bash
	npx asinit .
	```

5. Add `--use abort=` to the `asc` in `package.json`, for example:

	```json
	"asbuild:untouched": "asc assembly/index.ts --target debug --use abort=",
	"asbuild:optimized": "asc assembly/index.ts --target release --use abort=",
	```

6. Replace the content of `assembly/index.ts` with the code below, note to replace `{EASEGRESS_SDK_PATH}` with the path in step 1:

	```typescript
	// this line exports everything required by Easegress,
	export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'

	// import everything you need from the SDK,
	import { Program, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

	// define the program, 'Noop' is the name
	class Noop extends Program {
		// constructor is the initializer of the program, will be called once at the startup
		constructor(params: Map<string, string>) {
			super(params)
		}

		// run will be called for every request
		run(): i32 {
			return 0
		}
	}

	// register a factory method of the program, the only thing you
	// may want to change is the program name, here is 'Noop'
	registerProgramFactory((params: Map<string, string>) => {
		return new Noop(params)
	})
	```

7. Build with the below command, if everything is right, `untouched.wasm` (the debug version) and `optimized.wasm` (the release version) will be generated at the `build` folder.
   
   ```bash
   $ npm run asbuild
   ```

8. Create an HTTPServer in Easegress to listen on port 10080 to handle the HTTP traffic:

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

9. Create pipeline `wasm-pipeline` which includes a `WasmHost` filter:

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
	  code: /home/megaease/example/build/optimized.wasm
	  timeout: 100ms
	- name: proxy
	  kind: Proxy
	  pools:
	  - servers:
	    - url: http://127.0.0.1:9095
	    loadBalance:
	      policy: roundRobin' | egctl object create
	```

	Note to replace `/home/megaease/example/build/optimized.wasm` with the path of the file generated in step 7.

10. Set up a backend service at `http://127.0.0.1:9095` with the `mirror` server from the Easegress repository. Using another HTTP server is fine, but the `mirror` server prints requests it received to the console, which makes it much easier to verify the results:

	```bash
	$ git clone https://github.com/megaease/easegress.git
	$ cd easegress
	$ go run example/backend-service/mirror/mirror.go
	```

11. Execute below command in a new console to verify what we have done.

	```bash
	$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
	Your Request
	==============
	Method: POST
	URL   : /pipeline
	Header:
		User-Agent: [curl/7.68.0]
		Accept: [*/*]
		Content-Type: [application/x-www-form-urlencoded]
		Accept-Encoding: [gzip]
	Body  : Hello, Easegress
	```

### Add a New Header

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, request, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class AddHeader extends Program {
	run(): i32 {
		request.addHeader('Wasm-Added', 'I was added by WebAssembly')
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new AddHeader(params)
})
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
Your Request
==============
Method: POST
URL   : /pipeline
Header:
    Accept-Encoding: [gzip]
    User-Agent: [curl/7.68.0]
    Accept: [*/*]
    Content-Type: [application/x-www-form-urlencoded]
    Wasm-Added: [I was added by WebAssembly]
Body  : Hello, Easegress
```

### Add a New Header According to Configuration

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, request, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class AddHeader extends Program {
	headerName: string
	headerValue: string

	constructor(params: Map<string, string>) {
		this.headerName = params.get("headerName")
		this.headerValue = params.get("headerValue")
		super(params)
	}

	run(): i32 {
		request.addHeader(this.headerName, this.headerValue)
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new AddHeader(params)
})
```

And we also need to modify the filter configuration to add `headerName` and `headerValue` as parameters:

```yaml
filters:
  - name: wasm
    kind: WasmHost
    parameters:                                        # +
      headerName: "Wasm-Added-2"                       # +
      headerValue: "I was added by WebAssembly too"    # +
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
Your Request
==============
Method: POST
URL   : /pipeline
Header:
    Accept-Encoding: [gzip]
    User-Agent: [curl/7.68.0]
    Accept: [*/*]
    Content-Type: [application/x-www-form-urlencoded]
    Wasm-Added-2: [I was added by WebAssembly too]
```

### Add a Cookie

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, cookie, response, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class AddCookie extends Program {
	run(): i32 {
		let c = new cookie.Cookie()
		c.name = "wasm"
		c.value = "2021-07-29"
		c.httpOnly = true
		request.addCookie(c)
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new AddCookie(params)
})
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
Your Request
==============
Method: POST
URL   : /pipeline
Header:
    User-Agent: [curl/7.68.0]
    Accept: [*/*]
    Content-Type: [application/x-www-form-urlencoded]
    Cookie: [wasm=2021-07-29; HttpOnly=]
    Accept-Encoding: [gzip]
Body  : Hello, Easegress
```

### Mock Response

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, response, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class MockResponse extends Program {
	run(): i32 {
		response.setStatusCode(200)
		response.setBody(String.UTF8.encode("I have a new body now"))
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new MockResponse(params)
})
```

Because we are mocking a response, we need to remove the `proxy` from pipeline:

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
	code: /home/megaease/example/build/optimized.wasm
	timeout: 100ms' | egctl object update wasm-pipeline
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
I have a new body now
```


### Access Shared Data

When the `maxConcurrency` field of a WasmHost filter is larger than `1`, or when Easegress is deployed as a cluster, a single WasmHost filter could have more than one `Wasm Virtual Machine`s. Because safety is the design principle of WebAssembly, these VMs are isolated and can not share data with each other.

But sometimes, sharing data is useful, Easegress provides APIs for this:

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, response, cluster, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class SharedData extends Program {
	run(): i32 {
		let counter = cluster.AddInteger("counter", 1)
		response.setStatusCode(200)
		response.setBody(String.UTF8.encode("number of requests is: ", counter.toString()))
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new SharedData(params)
})
```

Suppose we are using the same pipeline configuration as in [Mock Response](#mock-response), we can build and verify this example with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/pipeline
number of requests is 1
$ curl http://127.0.0.1:10080/pipeline
number of requests is 2
$ curl http://127.0.0.1:10080/pipeline
number of requests is 3
```

We can view the shared data with:

```bash
$ egctl wasm list-data wasm-pipeline wasm
counter: "3"
```

where `wasm-pipeline` is the pipeline name and `wasm` is the filter name.

The shared data can be modified with:

```bash
$ echo 'counter: 100' | egctl wasm apply-data wasm-pipeline wasm
$ curl http://127.0.0.1:10080/pipeline
number of requests is 101
```

And can be deleted with:

```bash
$ egctl wasm delete-data wasm-pipeline wasm
```

### Return a Result Other Than 0

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'
import { Program, request, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class NonZeroResult extends Program {
	run(): i32 {
		let token = request.getHeader("Authorization")
		if(token == "") {
			return 1
		}
		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new NonZeroResult(params)
})
```

Return value `1` will be converted to `wasmResult1` by the `WasmHost` filter, we can use this result in the pipeline configuration:

```yaml
flow:
  - filter: wasm
    jumpIf: { wasmResult1: END }                   # +
  - filter: proxy
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress'
$ curl http://127.0.0.1:10080/pipeline -d 'Hello, Easegress' -HAuthorization:abc
Your Request
==============
Method: POST
URL   : /pipeline
Header:
    Accept-Encoding: [gzip]
    User-Agent: [curl/7.68.0]
    Accept: [*/*]
    Authorization: [abc]
    Content-Type: [application/x-www-form-urlencoded]
Body  : Hello, Easegress
```
