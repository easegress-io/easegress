# Routers

- [Routers](#routers)
  - [Ordered](#ordered)
  - [RadixTree](#radixTree)

Router determines how requests are routed to the corresponding Pipeline for subsequent processing. We currently support two routing strategies, `Ordered` and `RadixTree`, and you can choose a Router that suits your needs based on its characteristics, and we also provide the ability to customize Router, if the built-in Router does not meet your needs, you can choose Write a custom Router.

## Ordered

Ordered router is the default router for httpserver, as the name implies, its matching rules are in the order of the route definition.

Let's look at some examples to better understand the strategy.

```yaml
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
routerKind: Ordered
rules:
  - paths:
    - path: /pipeline/abc
      backend: static-backend
    - pathRegexp: `^/pipeline/regexp$`
      backend: regexp-backend
    - pathPrefix: /pipeline
      backend: prefix-backend
```

| path | Match backend |
|------|--------------|
| /pipeline/abc | `static-backend` |
|/pipeline/regexp | `regexp-backend` |
|/pipeline/test | `prefix-backend` |
|/pipeline/test | `prefix-backend` |
|/blog/bar | not match |

Take a look at another example.

```yaml
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
routerKind: Ordered
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: prefix-backend
    - path: /pipeline/abc
      backend: static-backend
    - pathRegexp: `^/pipeline/regexp$`
      backend: regexp-backend
```

| path | Match backend |
|------|--------------|
|/pipeline/abc | `prefix-backend` |
|/pipeline/regexp | `prefix-backend` |
|/pipeline/test | `prefix-backend` |
|/pipeline/test | `prefix-backend` |
|/blog/bar | not match |

It is clear to see that the matching rules of the router are matched in the order of route definition, and the matching stops when the result is reached.

## RadixTree

As the name implies, you can see that the underlying mechanism of the router uses a [Radix tree](https://en.wikipedia.org/wiki/Radix_tree])

Still in the same way, let's use some examples to better understand the strategy.

### 1. Full match

```shell
/users/aniaan
```

This rule will only match `/users/aniaan`

### 2. Prefix match

```shell
/users/*
```

This rule will match all paths prefixed with `/users`, eg: `/users/aniaan`, `/users/localvar`, `/users/megaease/abc`

### 3. Parameter match

```shell
/users/{username}/hovercard
```

This rule will match `/users/aniaan/hovercard` and `/users/localvar/hovercard`

### 4. Regexp match

```shell
/users/{username:[a-z]+}/hovercard
```

This rule will match `/users/aniaan/hovercard` and `/users/localvar/hovercard`, not `/users/123/hovercard`.

### 5. Match priority

`Full match` -> `Regexp match` -> `Parameter match` -> `Prefix match`

Let's look at an example to get a better impression.

```yaml
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
routerKind: RadixTree
rules:
  - paths:
    - path: /users/{username}/hovercard
      backend: parameter-path-backend
    - path: /users/{username:[a-z]+}/hovercard
      backend: regexp-backend
    - path: /users/*
      backend: prefix-backend
    - path: /users/aniaan/hovercard
      backend: full-match-backend
```

| path | Match backend |
|------|--------------|
|/users/aniaan/hovercard | `full-match-backend` |
|/users/12345/hovercard | `parameter-path-backend` |
|/users/localvar/hovercard | `regexp-backend` |
|/users/test | `prefix-backend` |
|/blog/bar | not match |

Let's take a look at another example to deepen our understanding

```yaml
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
routerKind: RadixTree
rules:
  - paths:
    - path: /user/{username}/123
      backend: h1
    - path: /user/wang/{num}
      backend: h2
    - path: /{type}/wang/123
      backend: h3
```

`/user/wang/123` will match `/user/wang/{num}`, which is determined by the matching order of RadixTree, you can split the path into individual characters, each character follows the matching order of `Full match` -> `Regexp match` -> `Parameter match` -> `Prefix match`, so the final match result is `/user/wang/{num}`.

RadixTree, compared with Ordered, has lost its definition order.

See one more.

```yaml
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
routerKind: RadixTree
- paths:
    - path: /users/{u:[a-z]+}sername/hovercard
      backend: h1
    - path: /users/u{sername}/hovercard
      backend: h2
```

Similarly, `/users/username/hovercard` will match `/users/u{sername}/hovercard` for the same reason as above, so you can try to understand it.

### Rewrite

Let's take a look at how RadixTree's Path Rewrite feature works. The syntax of Rewrite remains basically the same as that of Path, with a few minor differences.

```yaml
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
routerKind: RadixTree
rules:
  - paths:
    - path: /users/{username}/hovercard
      rewriteTarget: /api/users/{username}/card
      backend: parameter-path-backend
    - path: /users/{username:[a-z]+}/hovercard
      rewriteTarget: /api/users/{username}/card
      backend: regexp-backend
    - path: /users/*
      rewriteTarget: /api/users/{EG_WILDCARD}
      backend: prefix-backend
    - path: /users/aniaan/hovercard
      rewriteTarget: /api/users/aniaan/card
      backend: full-match-backend
```

| path | Match backend | Rewrite target|
|------|--------------|----------------|
|/users/aniaan/hovercard | `full-match-backend` | /api/users/aniaan/card |
|/users/12345/hovercard | `parameter-path-backend` | /api/users/12345/card |
|/users/localvar/hovercard | `regexp-backend` |  /api/users/localvar/card |
|/users/test | `prefix-backend` | /api/users/test |

You can see that only the wildcard `*` needs to be replaced by the built-in variable `EG_WILDCARD`.
