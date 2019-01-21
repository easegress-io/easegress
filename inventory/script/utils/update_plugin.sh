#!/bin/bash

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"

CLIENT=${SCRIPTPATH}/../../../bin/egctl

${CLIENT} admin plugin update "$@"
