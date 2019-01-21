#!/bin/sh

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CLIENT=${SCRIPTPATH}/../../../bin/egwctl

echo ""
echo "Update Speech Plugins"

${CLIENT} --address "${ADDRESS}" admin plugin update ${SCRIPTPATH}/plugins_template/*.json

echo ""
echo "Update Speech Pipelines"
${CLIENT} --address "${ADDRESS}" admin pipeline update ${SCRIPTPATH}/pipelines_template/*.json

