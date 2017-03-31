#!/bin/bash

SCRIPTFILE="$(readlink --canonicalize-existing "$0")"
echo "SCRIPTFILE: ${SCRIPTFILE}"
SCRIPTPATH="$(dirname "$SCRIPTFILE")"
echo "SCRIPTPATH: ${SCRIPTPATH}"

ADMIN=${SCRIPTPATH}/../../bin/easegateway_admin

KAFKA_BOOTSTRAP_SERVERS="$1"
if [ -z "${KAFKA_BOOTSTRAP_SERVERS}" ]; then
    KAFKA_BOOTSTRAP_SERVERS='127.0.0.1:9092'
fi

echo ""
echo "Update APM Plugins"
rm -fr ${SCRIPTPATH}/apm_plugins
cp -r ${SCRIPTPATH}/apm_plugins_template ${SCRIPTPATH}/apm_plugins
sed -i "s#KAFKA_BOOTSTRAP_SERVERS#"${KAFKA_BOOTSTRAP_SERVERS}"#g" ${SCRIPTPATH}/apm_plugins/*.json
${ADMIN} plugin update ${SCRIPTPATH}/apm_plugins/*.json

echo ""
echo "Update APM Pipelines"
${ADMIN} pipeline update ${SCRIPTPATH}/apm_pipelines_template/*.json

