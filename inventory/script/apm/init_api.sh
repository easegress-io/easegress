#!/bin/sh

SCRIPTPATH="$(cd "$(dirname "$0")"; pwd -P)"
echo "SCRIPTPATH: ${SCRIPTPATH}"

CLIENT=${SCRIPTPATH}/../../../bin/egctl

if [[ ! -x ${CLIENT} ]]
then
    echo "${CLIENT} not exist/executable"
    exit 1
fi

if [ -z "${GATEWAY_SERVER_ADDRESS}" ]; then
    GATEWAY_SERVER_ADDRESS='127.0.0.1:9090'
fi

if [ -z "${KAFKA_BOOTSTRAP_SERVERS}" ]; then
    KAFKA_BOOTSTRAP_SERVERS='127.0.0.1:9092'
fi

echo ""
echo "====================Initial APM Plugins===================="
rm -fr ${SCRIPTPATH}/plugins
cp -r ${SCRIPTPATH}/plugins_template ${SCRIPTPATH}/plugins

# Replace Kafka Placeholder
KAFKA_BOOTSTRAP_SERVERS="\"${KAFKA_BOOTSTRAP_SERVERS}\""
KAFKA_BOOTSTRAP_SERVERS=`echo $KAFKA_BOOTSTRAP_SERVERS | sed s/,/\",\"/g`
sed -i "s/KAFKA_BOOTSTRAP_SERVERS/${KAFKA_BOOTSTRAP_SERVERS}/g" ${SCRIPTPATH}/plugins/kafka_output.*.json

# Replace JSON Schema Placeholder
BASE64_JSON_SCHEMA_APP_METRICS=`base64 ${SCRIPTPATH}/schema/app_metrics.json | tr -d '[:space:]'`
BASE64_JSON_SCHEMA_APP_REQUESTS=`base64 ${SCRIPTPATH}/schema/app_requests.json | tr -d '[:space:]'`
BASE64_JSON_SCHEMA_LOGS=`base64 ${SCRIPTPATH}/schema/logs.json | tr -d '[:space:]'`
BASE64_JSON_SCHEMA_ZIPKIN_SPANS=`base64 ${SCRIPTPATH}/schema/zipkin_spans.json | tr -d '[:space:]'`
sed -i "s/BASE64_JSON_SCHEMA_APP_METRICS/${BASE64_JSON_SCHEMA_APP_METRICS}/g"   ${SCRIPTPATH}/plugins/json_validator.app_metrics.json
sed -i "s/BASE64_JSON_SCHEMA_APP_REQUESTS/${BASE64_JSON_SCHEMA_APP_REQUESTS}/g" ${SCRIPTPATH}/plugins/json_validator.app_requests.json
sed -i "s/BASE64_JSON_SCHEMA_LOGS/${BASE64_JSON_SCHEMA_LOGS}/g"                 ${SCRIPTPATH}/plugins/json_validator.logs.json
sed -i "s/BASE64_JSON_SCHEMA_ZIPKIN_SPANS/${BASE64_JSON_SCHEMA_ZIPKIN_SPANS}/g" ${SCRIPTPATH}/plugins/json_validator.zipkin_spans.json

${CLIENT} --address "${GATEWAY_SERVER_ADDRESS}" admin plugin add ${SCRIPTPATH}/plugins/*.json

rm -fr ${SCRIPTPATH}/plugins

echo ""
echo "====================Initial APM Pipelines===================="
${CLIENT} --address "${GATEWAY_SERVER_ADDRESS}" admin pipeline add ${SCRIPTPATH}/pipelines_template/*.json
