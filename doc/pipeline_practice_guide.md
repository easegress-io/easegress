# Ease Gateway Pipeline Practice Guide

## Overview
In this document, we would like to introduce 6 pipelines from real practice to cover some worth usecases as examples.

| Name | Description | Complexity level |
|:--|:--|:--:|
| [Ease Monitor edge service](#ease-monitor-edge-service) | Runs an example HTTPS endpoint to receive an Ease Monitor data, processes it in the pipeline and sends prepared data to kafka finally. | Beginner |
| [Http traffic throttling](#http-traffic-throttling) | Performs latency and throughput rate based traffic control. | Beginner |
| [Service circuit breaking](#service-circuit-breaking) | As a protection function, once the service failures reach a certain threshold all further calls to the service will be returned with an error directly, and when the service recovery the breaking function will be disabled automatically. | Beginner |
| [Http proxy with caching](#http-proxy-with-caching) | Caches HTTP/HTTPS response for duplicated request | Intermediate |
| [Service downgrading to protect critical service](#service-downgrading-to-protect-critical-service) | Under unexpected taffic which higher than planed, sacrifice the unimportant services but keep critical request is handled. | Intermediate |
| [Flash sale event support](#flash-sale-event-support) | A pair of pipelines to support flash sale event. For e-Commerence, it means we have very low price items with limited stock, but have huge amount of people online compete on that. | Advanced |

## Ease Monitor edge service

## Http traffic throttling

## Service circuit breaking

## Http proxy with caching

## Service downgrading to protect critical service

## Flash sale event support
