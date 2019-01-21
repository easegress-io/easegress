#!/bin/bash

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"

CLIENT=${SCRIPTPATH}/../../../bin/egwctl

${CLIENT} admin pipeline update "$@"
