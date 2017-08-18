#!/bin/bash

SCRIPTFILE="$(readlink --canonicalize-existing "$0")"
echo "SCRIPTFILE: ${SCRIPTFILE}"
SCRIPTPATH="$(dirname "$SCRIPTFILE")"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CLIENT=${SCRIPTPATH}/../../../bin/easegateway-client

ADDRESS="$1"
if [ -z "${ADDRESS}" ]; then
    ADDRESS='127.0.0.1:9090'
fi

KAFKA_BOOTSTRAP_SERVERS="$2"
if [ -z "${KAFKA_BOOTSTRAP_SERVERS}" ]; then
    KAFKA_BOOTSTRAP_SERVERS='127.0.0.1:9092'
fi

echo ""
echo "Initial APM Plugins"
rm -fr ${SCRIPTPATH}/apm_plugins
cp -r ${SCRIPTPATH}/apm_plugins_template ${SCRIPTPATH}/apm_plugins
sed -i "s#KAFKA_BOOTSTRAP_SERVERS#"${KAFKA_BOOTSTRAP_SERVERS}"#g" ${SCRIPTPATH}/apm_plugins/*.json
${CLIENT} --address "${ADDRESS}" admin plugin add ${SCRIPTPATH}/apm_plugins/*.json

echo ""
echo "Initial APM Pipelines"
${CLIENT} --address "${ADDRESS}" admin pipeline add ${SCRIPTPATH}/apm_pipelines_template/*.json
