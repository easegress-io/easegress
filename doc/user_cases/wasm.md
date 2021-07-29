# WebAssembly in Easegress

The `WasmHost` is a filter of Easegress which can be orchestrated into a pipeline. But while the behavior of all other filters are defined by filter developers and can only be fine-tuned by configuration, this filter implements a host environment for user-developed [WebAssembly](https://webassembly.org/) code, which enables users to control the filter behavior completely.

## Why Use WebAssembly in Easegress?

* **Zero Down Time**: filter behavior can be modified by a hot update.
* **Fast Develop, Fast Deploy**: we believe everyone can be a software developer, and you know your requirement better, no need to wait for MegaEase.
* **Every Thing Under Control**: the filter behavior is right under your fingers.
* **Develop with Your Favour Language**: choose one from `AssemblyScript`, `Go`, `C/C++`, `Rust` and etc at your wish (Note: a language specific SDK is required, we are working on this).

## Examples

**Note**: The `WasmHost` filter is disabled by default, to enable it, you need to build Easegress with the below command:

```bash
$ make build_server GOTAGS=wasmhost
```

We will use [AssemblyScript](https://www.assemblyscript.org/) as the language of the examples, please refer tp [this document](https://github.com/megaease/easegress-assemblyscript-sdk/blob/main/README.md) for how to build the examples, and [this document](https://github.com/megaease/easegress/blob/main/doc/wasmhost.md) for how to deploy them to Easegress.

### Basic: Noop

This example does nothing, but shows the basic structure of your code, in the latter examples, we will focus on `constructor` and `run`.

```typescript
// this line exports everything required by Easegress,
// you don't need to modify it, except the path of the SDK
export * from '../easegress/proxy'

// import everything you need from the SDK,
import { Program, request, response, cookie, LogLevel, log, registerProgramFactory } from '../easegress'

// define the program, 'Noop' is the name
class Noop extends Program {
	// constructor is the initializer of the problem, will be called once at startup
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
}
```

### Add a New Header

```typescript
class AddHeader extends Program {
	run(): i32 {
		request.addHeader('Wasm-Added', 'I was added by WebAssembly')
		return 0
	}
}
```

### Add a New Header According to Configuration

```typescript
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
```

And we also need to modify the filter configuration to add `headerName` and `headerValue` as parameters:

```yaml
filters:
  - name: wasm
    kind: WasmHost
	parameters:                                    # +
	  headerName: "Wasm-Added"                     # +
	  headerValue: "I was added by WebAssembly"    # +
```

### Set a Cookie

```typescript
class SetCookie extends Program {
	run(): i32 {
		let c = new cookie.Cookie()
		c.name = "wasm"
		c.value = "2021-07-29"
		c.httpOnly = true
		response.setCookie(c)
		return 0
	}
}
```

### Mock Response

```typescript
class MockResponse extends Program {
	run(): i32 {
		response.setStatusCode(200)
		response.setBody(String.UTF8.encode("I have a new body now"))
		return 0
	}
}
```

### Return a Result Other Than 0

```typescript
class NonZeroResult extends Program {
	run(): i32 {
		let token = request.getHeader("Authorization")
		if(token == "") {
			return 1
		}
		return 0
	}
}
```

Return value `1` will be converted to `wasmResult1` by the `WasmHost` filter, we can use this result in the pipeline configuration:

```yaml
flow:
  - filter: wasm
    jumpIf: { wasmResult1: END }                   # +
  - filter: proxy
```
