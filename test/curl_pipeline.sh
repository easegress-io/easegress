#!/usr/bin/env bash


# `X-Filter` Could be mirror or candidate.
curl -v http://127.0.0.1:10080/pipeline?epoch="$(date +%s)" \
	-H 'Content-Type: application/json' \
	-H 'X-Filter: candidate' \
	-d 'I am pipeline requester'
