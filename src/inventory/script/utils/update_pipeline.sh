#!/bin/bash

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"

CLIENT=${SCRIPTPATH}/../../../bin/easegateway-client

${CLIENT} admin pipeline update "$@"
