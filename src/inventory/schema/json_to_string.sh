#!/bin/bash

JSON=$(jq -c '.' < "$1")
JSON=$(echo $JSON | sed 's/"/\\"/g')
echo -n $JSON
