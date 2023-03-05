# Security

- [Security](#security)
  - [Basic: Load Balance](#basic-load-balance)
  - [Security: Verify Credential](#security-verify-credential)
    - [Header](#header)
    - [JWT](#jwt)
    - [Signature](#signature)
    - [OAuth2](#oauth2)
    - [Basic Auth](#basic-auth)
  - [References](#references)
    - [Header](#header-1)
    - [JWT](#jwt-1)
    - [Signature](#signature-1)
    - [OAuth2](#oauth2-1)
    - [Basic Auth](#basic-auth-1)
    - [Concepts](#concepts)

As a production-ready cloud-native traffic orchestrator, Easegress cares about security and provides several features to ensure that.

## Basic: Load Balance

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

## Security: Verify Credential

### Header

* Using Headers validation in Easegress. This is the simplest way for validating requests. It checks the HTTP request headers by using regular expression matching.

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: header-validator
  - filter: proxy
filters:
  - kind: Validator
    name: header-validator
    headers:
      Is-Valid:
        values: ["abc", "goodplan"]
        regexp: "^ok-.+$"
  - name: proxy
    kind: Proxy
```

* To enable Header type validator correctly in the pipeline, we should add it before filter `Proxy`.
* As the example above, it will check the `Is-Valid` header field by trying to match `abc` or `goodplan`. Also, it will use `^ok-.+$` regular expression for checking if it can't match the `values` filed.

*For the full YAML, see [here](#header-1)

### JWT

* Using JWT validation in Easegress. JWT is wildly used in the modern web environment. JSON Web Token (JWT, pronounced /dʒɒt/, same as the word "jot") is a proposed Internet standard for creating data with optional signature and/or optional encryption whose payload holds JSON that asserts some number of claims.[1]
> Easegress supports three types of JWT, HS256, HS384, and HS512.


``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: jwt-validator
  - filter: proxy
filters:
  - kind: Validator
    name: jwt-validator
    jwt:
      cookieName: auth
      algorithm: HS256
      secret: 6d79736563726574
  - name: proxy
    kind: Proxy
```

The example above will check the value named `auth` in the cookie with HS256 with the secret,6d79736563726574.
For the full YAML, see [here](#jwt-1)

### Signature
* Using Signature validation in Easegress. Signature validation implements an Amazon Signature V4[2] compatible signature validation validator. Once you enable this kind of validation, please make sure your HTTP client has followed the signature generation process in AWS V4 doc and bring it to request Easegress.

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: signature-validator
  - filter: proxy
filters:
  - kind: Validator
    name: signature-validator
    signature:
      accessKeys:
        AKID: SECRET
  - name: proxy
    kind: Proxy

```

The example here only uses an `accessKeys` for processing Amazon Signature V4 validation. It also has other complicated and customized fields for more security purposes. Check it out in the Easegress filter doc if needed.[3]

For the full YAML, see [here](#signature-1)

### OAuth2

* Using OAuth2 validation in Easegress. OAuth 2.0 is the industry-standard protocol for authorization. OAuth 2.0 focuses on client developer simplicity while providing specific authorization flows for web applications, desktop applications, mobile phones, and living room devices. This specification and its extensions are being developed within the IETF OAuth Working Group.[4]

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: oauth-validator
  - filter: proxy
filters:
  - kind: Validator
    name: oauth-validator
    oauth2:
      tokenIntrospect:
      endPoint: https://127.0.0.1:8443/auth/realms/test/protocol/openid-connect/token/introspect
      clientId: easegress
      clientSecret: 42620d18-871d-465f-912a-ebcef17ecb82
      insecureTls: false
  - name: proxy
    kind: Proxy
```

* The example above uses a token introspection server, which is provided by `endpoint` filed for validation. It also supports `Self-Encoded Access Tokens mode` which will require a JWT related configuration included. Check it out in the Easegress filter doc if needed. [5]

* For the full YAML, see [here](#oauth-1)

### Basic Auth

* Using Basic Auth validation in Easegress. Basic access authentication is the simplest technique for enforcing access control to web resources [6]. You can create .htpasswd file using *apache2-util* `htpasswd` [7] for storing encrypted user credentials. Please note that Basic Auth is not the most secure access control technique and it is not recommended to depend solely to Basic Auth when designing the security features of your environment.

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: oauth-validator
  - filter: proxy
filters:
  - kind: Validator
    name: oauth-validator
    basicAuth:
      mode: "FILE"
      userFile: '/etc/apache2/.htpasswd'
  - name: proxy
    kind: Proxy
```

* The example above uses credentials defined in `/etc/apache2/.htpasswd` to restrict access. Please check out apache2-utils documentation [7] for more details.

* For the full YAML, see [here](#basic-auth-1)

## References

### Header

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: header-validator
  - filter: proxy
filters:
  - kind: Validator
    name: header-validator
    headers:
      Is-Valid:
        values: ["abc", "goodplan"]
        regexp: "^ok-.+$"
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

### JWT

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: jwt-validator
  - filter: proxy
filters:
  - kind: Validator
    name: jwt-validator
    jwt:
      cookieName: auth
      algorithm: HS256
      secret: 6d7973656372657
  - kind: Proxy
    name: proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

### Signature

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: signature-validator
  - filter: proxy
filters:
  - kind: Validator
    name: signature-validator
    signature:
      accessKeys:
        AKID: SECRET
  - kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

### OAuth2

```yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: oauth-validator
  - filter: proxy
filters:
  - kind: Validator
    name: oauth-validator
    oauth2:
      tokenIntrospect:
      endPoint: https://127.0.0.1:8443/auth/realms/test/protocol/openid-connect/token/introspect
      clientId: easegress
      clientSecret: 42620d18-871d-465f-912a-ebcef17ecb82
      insecureTls: false
  - kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

### Basic Auth

``` yaml
name: pipeline-reverse-proxy
kind: Pipeline
flow:
  - filter: header-validator
  - filter: proxy
filters:
  - kind: Validator
    name: basic-auth-validator
    basicAuth:
      mode: "FILE"
      userFile: '/etc/apache2/.htpasswd'
  - name: proxy
    kind: Proxy
    pools:
    - servers:
      - url: http://127.0.0.1:9095
      - url: http://127.0.0.1:9096
      - url: http://127.0.0.1:9097
      loadBalance:
        policy: roundRobin
```

### Concepts

1. https://en.wikipedia.org/wiki/JSON_Web_Token
2. https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
3. https://github.com/megaease/easegress/blob/main/doc/reference/filters.md#signerliteral
4. https://oauth.net/2/
5. https://github.com/megaease/easegress/blob/main/doc/reference/filters.md#validatoroauth2jwt
6. https://en.wikipedia.org/wiki/Basic_access_authentication
7. https://manpages.debian.org/testing/apache2-utils/htpasswd.1.en.html
