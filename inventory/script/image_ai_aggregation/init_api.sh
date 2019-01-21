#!/bin/sh

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CLIENT=${SCRIPTPATH}/../../../bin/egctl

ADDRESS="$1"
if [ -z "${ADDRESS}" ]; then
    ADDRESS='127.0.0.1:9090'
fi

echo ""
echo "Initial Image AI Plugins"

${CLIENT} --address "${ADDRESS}" admin plugin add ${SCRIPTPATH}/plugins_template/*.json

echo ""
echo "Initial Image AI Pipelines"
${CLIENT} --address "${ADDRESS}" admin pipeline add ${SCRIPTPATH}/pipelines_template/*.json
