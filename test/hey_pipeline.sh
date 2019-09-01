#!/usr/bin/env bash

hey -c 100 -n 10000 \
	-H 'Content-Type: application/json' \
	-H 'X-Filter: candidate' \
	http://127.0.0.1:10080/pipeline

