#!/usr/bin/env bash

# `X-Filter` Could be mirror or candidate.
curl -v http://127.0.0.1:10080/proxy?epoch="$(date +%s)" \
	-H 'Content-Type: application/json' \
	-H 'X-Filter: mirror' \
	-d 'I am proxy reqeuster'
