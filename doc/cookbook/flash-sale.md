# Handle Flash Sale With Easegress and WebAssembly

- [Handle Flash Sale With Easegress and WebAssembly](#handle-flash-sale-with-easegress-and-webassembly)
	- [1. Getting Started](#1-getting-started)
		- [1.1 Setting Up The Flash Sale Project](#11-setting-up-the-flash-sale-project)
		- [1.2 Creating HTTP Server And Pipeline In Easegress](#12-creating-http-server-and-pipeline-in-easegress)
		- [1.3 Verify What We Have Done](#13-verify-what-we-have-done)
	- [2. Block All Requests Before Flash Sale Start](#2-block-all-requests-before-flash-sale-start)
	- [3. Randomly Block Requests](#3-randomly-block-requests)
	- [4. Lucky Once, Lucky Always](#4-lucky-once-lucky-always)
	- [5. Limit The Number Of Permitted Users](#5-limit-the-number-of-permitted-users)
	- [6. Reuse The Program](#6-reuse-the-program)
		- [6.1 Parameters](#61-parameters)
		- [6.2 Manage Shared Data](#62-manage-shared-data)
	- [7. Summary](#7-summary)

A flash sale is a discount or promotion offered by an eCommerce store for a short period. The quantity is limited, which often means the discounts are higher or more significant than run-of-the-mill promotions. 

However, significant discounts, limited quantity, and a short period leading to a significant high traffic spike, which often results in slow service, denial of service, or even downtime.

This document illustrates how to leverage the [WasmHost Filter](https://github.com/megaease/easegress/blob/main/doc/reference/wasmhost.md) to protect the backend service in a flash sale. The WebAssembly code is written in [AssemblyScript](https://www.assemblyscript.org/) by using the [Easegress AssemblyScript SDK](https://github.com/megaease/easegress-assemblyscript-sdk).

Before we start, we need to introduce why we use a service gateway with WebAssembly.  Firstly, Easegress as a service gateway is more responsible for the control logic. Secondly, the business logic like the flash sale would be a more customized thing and could be changed frequently.  Using Javascript or other high-level languages to write business logic could bring good productivity and lower technical barriers. With WebAssembly technology, the high-level languages code can be compiled to WASM and loaded dynamically at runtime. Furthermore, the WebAssembly code has good enough performance and security. So this combination can provide a perfect solution in terms of security, high performance, and customization extensions.

## 1. Getting Started

Please ensure a recent version of [Git](https://git-scm.com/), [Golang](https://golang.org), [Node.js](https://nodejs.org/) and its package manager [npm](https://www.npmjs.com/) are installed before continue. Basic knowledge about writing and working with TypeScript modules, which is very similar to AssemblyScript, is a plus.

**Note**: The `WasmHost` filter is disabled by default. To enable it, you need to build Easegress with the below command:

```bash
$ make build_server GOTAGS=wasmhost
```

### 1.1 Setting Up The Flash Sale Project

1 ) Clone git repository `easegress-assemblyscript-sdk` to somewhere on disk

```bash
$ git clone https://github.com/megaease/easegress-assemblyscript-sdk.git
```

2 ) Switch to a new directory and initialize a new node module:

```bash
npm init
```

3 ) Install the AssemblyScript compiler using npm, assume that the compiler is not required in production, and make it a development dependency:

```bash
npm install --save-dev assemblyscript
```

4 ) Once installed, the compiler provides a handy scaffolding utility to quickly set up a new AssemblyScript project, for example, in the directory of the just initialized node module:

```bash
npx asinit .
```

5 ) Add `--use abort=` to the `asc` in `package.json`, for example:

```json
"asbuild:untouched": "asc assembly/index.ts --target debug --use abort=",
"asbuild:optimized": "asc assembly/index.ts --target release --use abort=",
```

6 ) Replace the content of `assembly/index.ts` with the code below, note to replace `{EASEGRESS_SDK_PATH}` with the path in step 1). The code is just a skeleton and does "nothing" at present, it will be enhanced later:

```typescript
// this line exports everything required by Easegress,
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'

// import everything you need from the SDK,
import { Program, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

// define the program, 'FlashSale' is the name
class FlashSale extends Program {
	// constructor is the initializer of the program, will be called once at the startup
	constructor(params: Map<string, string>) {
		super(params)
	}

	// run will be called for every request
	run(): i32 {
		return 0
	}
}

// register a factory method of the FlashSale program
registerProgramFactory((params: Map<string, string>) => {
	return new FlashSale(params)
})
```

7 ) Build with the below command, if everything is right, `untouched.wasm` (the debug version) and `optimized.wasm` (the release version) will be generated at the `build` folder.
   
```bash
$ npm run asbuild
```

### 1.2 Creating HTTP Server And Pipeline In Easegress

Create an HTTPServer in Easegress to listen on port 10080 to handle the HTTP traffic:

```bash
$ echo '
kind: HTTPServer
name: http-server
port: 10080
keepAlive: true
https: false
rules:
- paths:
  - pathPrefix: /flashsale
    backend: flash-sale-pipeline' | egctl object create
```

Create pipeline `flash-sale-pipeline` which includes a `WasmHost` filter:

```bash
$ echo '
name: flash-sale-pipeline
kind: Pipeline
flow:
- filter: wasm
- filter: mock

filters:
- name: wasm
  kind: WasmHost
  maxConcurrency: 2
  code: /home/megaease/example/build/optimized.wasm
  timeout: 100ms
- name: mock
  kind: Mock
  rules:
  - body: "You can buy the laptop for $1 now.\n"
    code: 200' | egctl object create
```

Note to replace `/home/megaease/example/build/optimized.wasm` with the path of the file generated in step 7) of section 1.1.

In the above pipeline configuration, a `Mock` filter is used as the backend service. In practice, you will need a `Proxy` filter to forward requests to the real backend.

### 1.3 Verify What We Have Done

Execute the below command from a new console, you should get a similar result if everything is right:

```bash
$ curl http://127.0.0.1:10080/flashsale
You can buy the laptop for $1 now.
```

## 2. Block All Requests Before Flash Sale Start

All flash sale promotions have a start time, requests before the time should be blocked. Suppose the start time is UTC `2021-08-08 00:00:00`, this can be accomplished with the below code:

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'

import { Program, response, parseDate, getUnixTimeInMs, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class FlashSale extends Program {
	// startTime is the start time of the flash sale, unix timestamp in millisecond
	startTime: i64

	constructor(params: Map<string, string>) {
		super(params)
		this.startTime = parseDate("2021-08-08T00:00:00+00:00").getTime()
	}

	run(): i32 {
		// if flash sale not start yet
		if (getUnixTimeInMs() < this.startTime) {
			// we just set response body to 'not start yet' here, in practice,
			// we will use 'response.setStatusCode(302)' to redirect user to
			// a static page.
			response.setBody(String.UTF8.encode("not start yet.\n"))
			return 1
		}

		return 0
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new FlashSale(params)
})
```

Build and inform Easegress to reload with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
```

`curl` the flash sale URL, we will get `not start yet.` before the start time of the flash sale.

```bash
$ curl http://127.0.0.1:10080/flashsale
not start yet.
```

## 3. Randomly Block Requests

After the start of the flash sale, Easegress should block requests randomly, this greatly reduces the total number of requests sent to the backend service, which protects the service from the strike of the traffic spike. The randomness brings another benefit also: geography differences result in latency differences, users with a lower latency are more likely to be the early users. The randomness removes the edge of these users and makes the flash sale fairer.

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'

import { Program, response, parseDate, getUnixTimeInMs, rand, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class FlashSale extends Program {
	startTime: i64

	// blockRatio is the ratio of requests being blocked to protect backend service
	// for example: 0.4 means we blocks 40% of the requests randomly.
	blockRatio: f64

	constructor(params: Map<string, string>) {
		super(params)
		this.startTime = parseDate("2021-08-08T00:00:00+00:00").getTime()
		this.blockRatio = 0.4
	}

	run(): i32 {
		if (getUnixTimeInMs() < this.startTime) {
			response.setBody(String.UTF8.encode("not start yet.\n"))
			return 1
		}

		if (rand() > this.blockRatio) {
			// the lucky guy
			return 0
		}

		// block this request, set response body to `sold out`
		response.setBody(String.UTF8.encode("sold out.\n"))
		return 2
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new FlashSale(params)
})
```

Build and verify with (suppose the flash sale was already started):

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/flashsale
sold out.
$ curl http://127.0.0.1:10080/flashsale
You can buy the laptop for $1 now.
$ curl http://127.0.0.1:10080/flashsale
sold out.
```

We will get a `sold out` message at the possibility of 40%. Note the `blockRatio` is `0.4` in this example, while in practice, `0.999`, `0.9999` will be much more make sense.

## 4. Lucky Once, Lucky Always

From the view of business, after we permit a lucky user to go forward, we should always permit this user to go forward; but from the logic of the code in the last step, the request may be blocked if the user accesses the URL again.

Fortunately, all users need to sign in before joining the flash sale, that's the requests will contain an identifier of the user, we can use this identifier to record the lucky users.

As an example, we suppose the value of the `Authorization` header is the desired identifier (the identifier could be a JWT token, and the [Validator filter](./security.md#security-verify-credential) can be used to validate the token, but this is out of the scope of this document).

However, due to the `maxConcurrency` option in the filter configuration, using a `Set` the store all permitted users won't work.

`maxConcurrency` is the number of WebAssembly VMs of the `WasmHost` filter, and because WebAssembly is designed to be safe, two VMs can not share data even if they are executing the same copy of code. That is, after VM1 permits a user, if the next request of the user is processed by VM2, it could be blocked. This could also happen when Easegress is deployed as a cluster.

To overcome this issue, Easegress provide APIs to access shared data:

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'

import { Program, request, parseDate, response, cluster, getUnixTimeInMs, rand, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class FlashSale extends Program {
	startTime: i64
	blockRatio: f64

	constructor(params: Map<string, string>) {
		super(params)
		this.startTime = parseDate("2021-08-08T00:00:00+00:00").getTime()
		this.blockRatio = 0.4
	}

	run(): i32 {
		if (getUnixTimeInMs() < this.startTime) {
			response.setBody(String.UTF8.encode("not start yet.\n"))
			return 1
		}

		// check if the user was already permitted
		let id = request.getHeader("Authorization")
		if (cluster.getString("id/" + id) == "true") {
			return 0
		}

		if (rand() > this.blockRatio) {
			// add the lucky guy to permitted users
			cluster.putString("id/" + id, "true")
			return 0
		}

		response.setBody(String.UTF8.encode("sold out.\n"))
		return 2
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new FlashSale(params)
})
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user1
sold out.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user1
You can buy the laptop for $1 now.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user1
You can buy the laptop for $1 now.
```

Repeat the `curl` command, we will find out that the user will never be blocked again after he/she was permitted for the first time.

## 5. Limit The Number Of Permitted Users

As the quantity is often limited in a flash sale, we can block users after we have permitted a number of users. For example, if the quantity is 10, permit 100 users is enough in most cases:

```typescript
export * from '{EASEGRESS_SDK_PATH}/easegress/proxy'

import { Program, request, parseDate, response, cluster, getUnixTimeInMs, rand, registerProgramFactory } from '{EASEGRESS_SDK_PATH}/easegress'

class FlashSale extends Program {
	startTime: i64
	blockRatio: f64

	// maxPermission is the upper limits of permitted users 
	maxPermission: i32

	constructor(params: Map<string, string>) {
		super(params)
		this.startTime = parseDate("2021-08-08T00:00:00+00:00").getTime()
		this.blockRatio = 0.4
		this.maxPermission = 3
	}

	run(): i32 {
		if (getUnixTimeInMs() < this.startTime) {
			response.setBody(String.UTF8.encode("not start yet.\n"))
			return 1
		}

		let id = request.getHeader("Authorization")
		if (cluster.getString("id/" + id) == "true") {
			return 0
		}

		// check the count of identifiers to see if we have reached the upper limit
		if (cluster.countKey("id/") < this.maxPermission) {
			if (rand() > this.blockRatio) {
				cluster.putString("id/" + id, "true")
				return 0
			}
		}

		response.setBody(String.UTF8.encode("sold out.\n"))
		return 2
	}
}

registerProgramFactory((params: Map<string, string>) => {
	return new FlashSale(params)
})
```

Build and verify with:

```bash
$ npm run asbuild
$ egctl wasm reload-code
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user1
You can buy the laptop for $1 now.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user2
sold out.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user2
You can buy the laptop for $1 now.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user3
You can buy the laptop for $1 now.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user4
sold out.
$ curl http://127.0.0.1:10080/flashsale -HAuthorization:user4
sold out.
```

After 3 users were permitted, the 4th user is blocked forever.

## 6. Reuse The Program

### 6.1 Parameters

We have hard-coded `startTime`, `blockRatio`, and `maxPermission` in the above examples, which means if we have another flash sale, we need to modify the code. This is not a good practice.

A better approach is putting these parameters into the configuration:

```yaml
filters:
  - name: wasm
    kind: WasmHost
    parameters:                                        # +
      startTime: "2021-08-08T00:00:00+00:00"           # +
      blockRatio: "0.4"                                # +
      maxPermission: "3"                               # +
```

And then revise the `constructor` of the program to read in these parameters:

```typescript
	constructor(params: Map<string, string>) {
		super(params)

		let key = "startTime"
		if (params.has(key)) {
			let val = params.get(key)
			this.startTime = parseDate(val).getTime()
		}

		key = "blockRatio"
		if (params.has(key)) {
			let val = params.get(key)
			this.blockRatio = parseFloat(val)
		}

		key = "maxPermission"
		if (params.has(key)) {
			let val = params.get(key)
			this.maxPermission = i32(parseInt(val))
		}
	}
```

### 6.2 Manage Shared Data

As we can see in [Lucky Once, Lucky Always](#4-lucky-once-lucky-always), shared data is useful, but when reusing the code and configuration for a new flash sale event, the legacy data could cause problems. Easegress provides commands to manage these data.

We can view current data with (where `flash-sale-pipeline` is the pipeline name and `wasm` is the filter name):

```bash
$ egctl wasm list-data flash-sale-pipeline wasm
id/user1: "true"
id/user2: "true"
id/user3: "true"
```

Update the data with:

```bash
$ echo '
id/user4: "true"
id/user5: "true"' | egctl wasm apply-data flash-sale-pipeline wasm
$ egctl wasm list-data flash-sale-pipeline wasm
id/user1: "true"
id/user2: "true"
id/user3: "true"
id/user4: "true"
id/user5: "true"
```

And delete all data with:

```bash
$ egctl wasm delete-data flash-sale-pipeline wasm
$ egctl wasm list-data flash-sale-pipeline wasm
{}
```

Well, the above is all technical details. You can freely use the code to customize your business logic. However, it should be aware that **the above is just a demo, and the practical solution is more complicated because it also needs to filter the crawlers and hackers**. If you need a more professional solution, welcome to contact us.

## 7. Summary

Using WebAssembly's security, high performance, and real-time dynamic loading capabilities, we can not only do such a high concurrency business on the gateway but also achieve some more complex business logic support. Because WebAssembly can reuse a variety of high-level languages (such as Javascript, C/C++, Rust, Python, C#, etc.). Easegress has more capabilities to play in a high-performance traffic orchestration with distributed architecture. Both of there bring much imagination to solve the problem with efficient operation and maintenance.
