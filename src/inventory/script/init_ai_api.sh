#!/bin/bash

SCRIPTFILE="$(readlink --canonicalize-existing "$0")"
echo "SCRIPTFILE: ${SCRIPTFILE}"
SCRIPTPATH="$(dirname "$SCRIPTFILE")"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CLIENT=${SCRIPTPATH}/../../bin/easegateway-client

ADDRESS="$1"
if [ -z "${ADDRESS}" ]; then
    ADDRESS='127.0.0.1:9090'
fi

echo ""
echo "Initial AI Plugins"
rm -fr ${SCRIPTPATH}/ai_plugins
cp -r ${SCRIPTPATH}/ai_plugins_template ${SCRIPTPATH}/ai_plugins
${CLIENT} --address "${ADDRESS}" admin plugin add ${SCRIPTPATH}/ai_plugins/*.json

echo ""
echo "Initial AI Pipelines"
${CLIENT} --address "${ADDRESS}" admin pipeline add ${SCRIPTPATH}/ai_pipelines_template/*.json
