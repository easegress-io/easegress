#!/bin/sh

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CLIENT=${SCRIPTPATH}/../../../bin/egctl

echo ""
echo "Update Image AI Plugins"

${CLIENT} --address "${ADDRESS}" admin plugin update ${SCRIPTPATH}/plugins_template/*.json

echo ""
echo "Update Image AI Pipelines"
${CLIENT} --address "${ADDRESS}" admin pipeline update ${SCRIPTPATH}/pipelines_template/*.json

