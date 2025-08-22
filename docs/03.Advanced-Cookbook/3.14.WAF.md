# Web Application Firewall <!-- omit from toc -->


- [Introduction](#introduction)
- [Configuring Web Application Firewall](#configuring-web-application-firewall)
  - [Custom](#custom)
  - [OWASP Core Rule Set](#owasp-core-rule-set)
  - [IP Blocker](#ip-blocker)
    - [Whitelist Mode](#whitelist-mode)
    - [Blacklist Mode](#blacklist-mode)
    - [Combined Mode (Whitelist \& Blacklist)](#combined-mode-whitelist--blacklist)
  - [GEO IP Blocker](#geo-ip-blocker)
    - [Whitelist Mode (allowedCountries)](#whitelist-mode-allowedcountries)
    - [Blacklist Mode (deniedCountries)](#blacklist-mode-deniedcountries)
  - [Rate Limiter](#rate-limiter)
- [Observability](#observability)


## Introduction

The Web Application Firewall (WAF) protects your apps by inspecting HTTP(S) traffic and blocking malicious requests at the application layer. It supports standard rule sets like OWASP CRS and flexible custom policies.

Beyond basic request filtering, the WAF includes IP/Geo access control, rate limiting, and rich observability for tuning and incident response. This ensures strong protection with low false positives and clear operational insight.

## Configuring Web Application Firewall

Here is a simple example shows how to configure Easegress to protect your backend using a WAF with a minimal subset of the OWASP Core Rule Set (CRS) focused on SQL Injection (SQLi) detection.

```yaml
name: waf-controller
kind: WAFController
ruleGroups:
- name: sqlinjection
  rules:
    customRules: |
      // https://github.com/corazawaf/coraza-coreruleset/blob/main/rules/%40crs-setup.conf.example
      // check coraza core rule set recommend setup file 
    owaspRules:
    - REQUEST-901-INITIALIZATION.conf
    - REQUEST-942-APPLICATION-ATTACK-SQLI.conf
    - REQUEST-949-BLOCKING-EVALUATION.conf

------

name: waf-server
kind: HTTPServer
port: 10080
rules:
- paths: 
  - pathPrefix: /
    backend: waf-pipeline

-----

name: waf-pipeline
kind: Pipeline
filters:
- name: waf-filter
  kind: WAF
  ruleGroup: sqlinjection
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
    - url: http://127.0.0.1:9096
    loadBalance:
      policy: roundRobin
```

It will do following things:

1. Loads a minimal, SQLi-focused subset of the OWASP CRS:
    - 901: Initialization and variable setup
    - 942: SQL Injection detection
    - 949: Final blocking evaluation
2. Routes traffic to port 10080 through the waf-pipeline, where the WAF filter evaluates requests against the sqlinjection rule group before proxying to your backend pool.

### Custom

This example shows how to load the full OWASP Core Rule Set (CRS) and add your own custom Coraza/ModSecurity rules in Easegress. The example below blocks all POST requests at both `phase 1` and `phase 2` for demonstration purposes.

```yaml
name: waf-controller
kind: WAFController
ruleGroups:
- name: sqlinjection
  rules:
    loadOwaspCrs: true
    customRules: |
      SecRule REQUEST_METHOD "POST" "id:1000001,phase:1,block,log,msg:'Block all POST requests (phase 1)',severity:'CRITICAL'"
      SecRule REQUEST_METHOD "POST" "id:1000002,phase:2,block,log,msg:'Block all POST requests (phase 2)',severity:'CRITICAL'"
```

What this does:

- `loadOwaspCrs`: true loads the full OWASP CRS bundle. You can includes them as you needed. For full rule set, please check: [here](https://github.com/corazawaf/coraza-coreruleset/tree/main/rules/%40owasp_crs).
- `customRules` adds two rules that explicitly block all POST requests. They run in different phases to illustrate the request processing lifecycle. For more details about Coraza/Modsecurity rules please check: [here](https://coraza.io/docs/seclang/)

Feel free to add your own rules as needed.

### OWASP Core Rule Set

This example shows how to use the corazawaf/coraza-coreruleset package to protect your applications. This package provides a straightforward way to embed the official OWASP Core Rule Set (CRS) directly into a Go application using the Coraza Web Application Firewall (WAF).

To effectively use the CRS, a proper setup is essential. This involves including foundational configuration files that define the behavior of both the Coraza engine and the rule set itself.

Coraza Configuration [@coraza.conf-recommended](https://github.com/corazawaf/coraza-coreruleset/blob/main/rules/%40coraza.conf-recommended): This file provides the recommended base settings for the Coraza WAF engine.

CRS Setup [@crs-setup.conf.example](https://github.com/corazawaf/coraza-coreruleset/blob/main/rules/%40crs-setup.conf.example): This file is crucial for customizing the Core Rule Set. Here, you can configure paranoia levels, anomaly score thresholds, and other CRS-specific behaviors. You should copy this file and tailor it to your application's needs.

```yaml
name: waf-controller
kind: WAFController
ruleGroups:
- name: sqlinjection
  rules:
    customRules: |
      // https://github.com/corazawaf/coraza-coreruleset/blob/main/rules/%40crs-setup.conf.example
      // The crs-setup.conf file is required for CRS to function.
      // This is a minimal example; a full setup file offers more customization.
      SecRequestBodyAccess On
      SecDefaultAction "phase:1,log,auditlog,pass"
      SecDefaultAction "phase:2,log,auditlog,pass"
      SecAction \
          "id:900990,\
          phase:1,\
          pass,\
          t:none,\
          nolog,\
          tag:'OWASP_CRS',\
          ver:'OWASP_CRS/4.16.0',\
          setvar:tx.crs_setup_version=4160"
    owaspRules:
    - REQUEST-901-INITIALIZATION.conf
    - REQUEST-913-SCANNER-DETECTION.conf
    - REQUEST-921-PROTOCOL-ATTACK.conf
    - REQUEST-922-MULTIPART-ATTACK.conf
    - REQUEST-949-BLOCKING-EVALUATION.conf
```

For all rules, please check [here](https://github.com/corazawaf/coraza-coreruleset/tree/main/rules/%40owasp_crs).

### IP Blocker

The `ipBlocker` rule filters requests based on the source IP address, specified in `CIDR` format.

#### Whitelist Mode

Logic: Allows requests only from IPs on the list. All others are blocked.

```yaml
kind: WAFController
name: waf-controller
ruleGroups:
  - name: ipblocker
    rules:
      ipBlocker:
        whitelist:
          - 136.252.0.2/32
```

#### Blacklist Mode

Logic: Blocks requests from IPs on the list. All others are allowed.

```yaml
kind: WAFController
name: waf-controller
ruleGroups:
  - name: ipblocker
    rules:
      ipBlocker:
        blacklist:
          - 158.160.2.1/32
```

#### Combined Mode (Whitelist & Blacklist)

Logic: Blocks request if IP is in blacklist OR not in whitelist.

```yaml
kind: WAFController
name: waf-controller
ruleGroups:
  - name: ipblocker
    rules:
      ipBlocker:
        whitelist:
          - 136.252.0.2/32
        blacklist:
          - 158.160.2.1/32
```

### GEO IP Blocker

The `geoIPBlocker` rule allows or denies requests based on the visitor's country of origin. It requires a GeoIP database to function.

- `dbPath`: The file path to your GeoIP database (.mmdb file).
- `dbUpdateCron`: (Optional) A cron expression to schedule automatic updates for the database.
- `Country Codes`: Use standard two-letter ISO codes (e.g., US, CN).

#### Whitelist Mode (allowedCountries)

Logic: Only allows requests from the specified countries. All other countries will be blocked.

```yaml
kind: WAFController
name: waf-controller
ruleGroups:
  - name: geoipblocker
    rules:
      geoIPBlocker:
        dbPath: <your-dir>/Country.mmdb
        allowedCountries:
          - XX
```

#### Blacklist Mode (deniedCountries)

Logic: Blocks requests from the specified countries. All other countries will be allowed.

```yaml
kind: WAFController
name: waf-controller
ruleGroups:
  - name: geoipblocker
    rules:
      geoIPBlocker:
        dbPath: <your-dir>/Country.mmdb
        dbUpdateCron: "0 0 1 * *"
        deniedCountries:
          - XX
```

### Rate Limiter

The RateLimiter restricts how many requests a client can make within a specific time window (e.g., requests per second). This is a crucial defense against brute-force attacks and helps prevent system overload from excessive traffic.

For detailed configuration, please refer to the official documentation for the built-in Easegress RateLimiter filter [here](./../07.Reference/7.02.Filters.md#ratelimiter).

## Observability

To monitor requests blocked by the WAF, you can use the `waf_total_refused_requests` counter metric exposed to Prometheus. This metric increments every time a request is denied by a WAF rule.

It includes several labels to help you pinpoint the cause of the block:

Common Labels:

- `kind`, `clusterName`, `clusterRole`, `instanceName`: Standard labels to identify the Easegress instance.

Metric-Specific Labels:

- `ruleGroup`: The name of the rule group that blocked the request.
- `ruleID`: The ID of the specific rule that was triggered.
- `action`: The WAF action, canbe one of `drop`, `deny`, `redirect`.

Using these labels, you can create detailed dashboards and alerts, for instance, to track which rules are most active or to identify repeated block events from a specific source.

For a complete list of all available metrics, please see the [Metrics](../07.Reference/7.08.Metrics.md) reference documentation.
