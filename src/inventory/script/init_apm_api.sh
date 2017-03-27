#!/usr/bin/env bash

SCRIPTFILE="$(readlink --canonicalize-existing "$0")"
echo "SCRIPTFILE: ${SCRIPTFILE}"
SCRIPTPATH="$(dirname "$SCRIPTFILE")"
echo "SCRIPTPATH: ${SCRIPTPATH}"

PROTOCOL="http://"
LISTEN_ADDRESS="127.0.0.1:9090"

echo -e "#### Initial APM Plugins ####"

# create_plugin body
function create_plugin() {
    if [ $# -ne 1 ]; then
        echo "wrong arguments: " $#
        return 1
    fi

    curl ${PROTOCOL}{$LISTEN_ADDRESS}/admin/v1/plugins \
        -i \
        -X POST \
        -H "Content-Type:application/json" \
        -H "Accept:application/json" \
        -w "\n" \
        -d "$1"
}

################ Begin Plugin HTTPInput
echo -e '1.1 Create Plugin HTTPInput - http_input.metrics'
create_plugin '{"type": "HTTPInput", "config": {"plugin_name": "http_input.metrics", "url": "/v1/metrics", "method": "POST", "headers": {"Content-Type": ["application/x-graphite"], "Content-Encoding": ["", "gzip", "gzip, deflate"], "User-Agent": ["collectd/5.7.0"]}}}'

echo -e '1.2 Create Plugin HTTPInput - http_input.logs'
create_plugin '{"type": "HTTPInput", "config": {"plugin_name": "http_input.logs", "url": "/v1/logs", "method": "POST", "headers": {"Content-Type": ["application/json"], "Content-Encoding": ["", "gzip", "gzip, deflate"], "User-Agent": ["filebeat/5.2.3", "filebeat/5.0.0"]}}}'

echo -e '1.3 Create Plugin HTTPInput - http_input.app_metrics'
create_plugin '{"type": "HTTPInput", "config": {"plugin_name": "http_input.app_metrics", "url": "/v1/app_metrics", "method": "POST", "headers": {"Content-Type": ["application/json"], "Content-Encoding": ["", "gzip", "gzip, deflate"], "User-Agent": ["easeagent/0.1.0"]}}}'

echo -e '1.4 Create Plugin HTTPInput - http_input.app_requests'
create_plugin '{"type": "HTTPInput", "config": {"plugin_name": "http_input.app_requests", "url": "/v1/app_requests", "method": "POST", "headers": {"Content-Type": ["application/json"], "Content-Encoding": ["", "gzip", "gzip, deflate"], "User-Agent": ["easeagent/0.1.0"]}}}'

echo -e '1.5 Create Plugin HTTPInput - http_input.zipkin_spans'
create_plugin '{"type": "HTTPInput", "config": {"plugin_name": "http_input.zipkin_spans", "url": "/v1/zipkin_spans", "method": "POST", "headers": {"Content-Type": ["application/json"], "Content-Encoding": ["", "gzip", "gzip, deflate"], "User-Agent": ["easeagent/0.1.0"]}}}'

echo -e '1.6 Create Plugin HTTPInput - http_input.mobile'
create_plugin '{"type": "HTTPInput", "config": {"plugin_name": "http_input.mobile", "url": "/v1/mobile_metrics", "method": "POST", "headers": {"Content-Type": ["application/x-protobuf"], "Content-Encoding": ["", "gzip", "gzip, deflate"], "User-Agent": ["iOS", "Andriod"]}}}'
################ End Plugin HTTPInput


################ Begin Plugin GWProtoAdaptor
echo -e '2.1. Create Plugin GWProtoAdaptor - gwproto_adaptor.mobile'
create_plugin '{"type": "GWProtoAdaptor", "config": {"plugin_name": "gwproto_adaptor.mobile"}}'
################ End Plugin GWPoroAdaptor

################ Begin Plugin GraphiteValidator
echo -e '3.1. Create Plugin GraphiteValidator - graphite_validator.metrics'
create_plugin '{"type": "GraphiteValidator", "config": {"plugin_name": "graphite_validator.metrics"}}'
################ End Plugin GraphiteValidator


################ Begin Plugin JSONValidator
LOGS_SCHEMA=$(cat ${SCRIPTPATH}/../schema/logs.json)
LOGS_SCHEMA=$(echo $LOGS_SCHEMA | tr -d '\r\n[:blank:]' | sed 's/"/\\"/g')

APP_METRICS_SCHEMA=$(cat ${SCRIPTPATH}/../schema/app_metrics.json)
APP_METRICS_SCHEMA=$(echo $APP_METRICS_SCHEMA | tr -d '\r\n[:blank:]' | sed 's/"/\\"/g')

APP_REQUESTS_SCHEMA=$(cat ${SCRIPTPATH}/../schema/app_requests.json)
APP_REQUESTS_SCHEMA=$(echo $APP_REQUESTS_SCHEMA | tr -d '\r\n[:blank:]' | sed 's/"/\\"/g')

ZIPKIN_SPANS_SCHEMA=$(cat ${SCRIPTPATH}/../schema/zipkin_spans.json)
ZIPKIN_SPANS_SCHEMA=$(echo $ZIPKIN_SPANS_SCHEMA | tr -d '\r\n[:blank:]' | sed 's/"/\\"/g')

echo -e '4.1 Create Plugin JSONValidator - json_validator.logs'
BODY=`printf '{"type": "JSONValidator", "config": {"plugin_name": "json_validator.logs", "schema": "%s"}}' "${LOGS_SCHEMA}"`
create_plugin "${BODY}"

echo -e '4.2 Create Plugin JSONValidator - json_validator.app_metrics'
BODY=`printf '{"type": "JSONValidator", "config": {"plugin_name": "json_validator.app_metrics", "schema": "%s"}}' "${APP_METRICS_SCHEMA}"`
create_plugin "${BODY}"

echo -e '4.3 Create Plugin JSONValidator - json_validator.app_requests'
BODY=`printf '{"type": "JSONValidator", "config": {"plugin_name": "json_validator.app_requests", "schema": "%s"}}' "${APP_REQUESTS_SCHEMA}"`
create_plugin "${BODY}"

echo -e '4.4 Create Plugin JSONValidator - json_validator.zipkin_spans'
BODY=`printf '{"type": "JSONValidator", "config": {"plugin_name": "json_validator.zipkin_spans", "schema": "%s"}}' "${ZIPKIN_SPANS_SCHEMA}"`
create_plugin "${BODY}"
################ End Plugin JSONValidator


################ Begin Plugin GraphiteGidExtractor
echo -e '5.1 Create Plugin GraphiteGidExtractor - graphite_gid_extractor.metrics'
create_plugin '{"type": "GraphiteGidExtractor", "config": {"plugin_name": "graphite_gid_extractor.metrics"}}'
################ End Plugin GraphiteGidExtractor


################ Begin Plugin JSONGidExtractor
echo -e '6.1 Create Plugin JSONGidExtractor - json_gid_extractor.logs'
create_plugin '{"type": "JSONGidExtractor", "config": {"plugin_name": "json_gid_extractor.logs"}}'

echo -e '6.2 Create Plugin JSONGidExtractor - json_gid_extractor.app_metrics'
create_plugin '{"type": "JSONGidExtractor", "config": {"plugin_name": "json_gid_extractor.app_metrics"}}'

echo -e '6.3 Create Plugin JSONGidExtractor - json_gid_extractor.app_requests'
create_plugin '{"type": "JSONGidExtractor", "config": {"plugin_name": "json_gid_extractor.app_requests"}}'
################ End Plugin JSONGidExtractor


################ Begin Plugin KafkaOutput
echo -e '7.1 Create Plugin KafkaOutput - kafka_output.metrics'
create_plugin '{"type": "KafkaOutput", "config": {"plugin_name": "kafka_output.metrics", "brokers": ["127.0.0.1:9092"], "client_id": "gateway.kafka_output.metrics", "topic": "collectd"}}'

echo -e '7.2 Create Plugin KafkaOutput - kafka_output.logs'
create_plugin '{"type": "KafkaOutput", "config": {"plugin_name": "kafka_output.logs", "brokers": ["127.0.0.1:9092"], "client_id": "gateway.kafka_output.logs", "topic": "filebeat"}}'

echo -e '7.3 Create Plugin KafkaOutput - kafka_output.app_metrics'
create_plugin '{"type": "KafkaOutput", "config": {"plugin_name": "kafka_output.app_metrics", "brokers": ["127.0.0.1:9092"], "client_id": "gateway.kafka_output.app_metrics", "topic": "sm-metrics"}}'

echo -e '7.4 Create Plugin KafkaOutput - kafka_output.app_requests'
create_plugin '{"type": "KafkaOutput", "config": {"plugin_name": "kafka_output.app_requests", "brokers": ["127.0.0.1:9092"], "client_id": "gateway.kafka_output.app_requests", "topic": "sm-requesttraces"}}'

echo -e '7.5 Create Plugin KafkaOutput - kafka_output.zipkin_spans'
create_plugin '{"type": "KafkaOutput", "config": {"plugin_name": "kafka_output.zipkin_spans", "brokers": ["127.0.0.1:9092"], "client_id": "gateway.kafka_output.zipkin_spans", "topic": "zipkin_spans"}}'

echo -e '7.6 Create Plugin KafkaOutput - kafka_output.mobile'
create_plugin '{"type": "KafkaOutput", "config": {"plugin_name": "kafka_output.mobile", "brokers": ["127.0.0.1:9092"], "client_id": "gateway.kafka_output.mobile", "topic": "mobile"}}'
################ End Plugin KafkaOutput


################ Begin Plugin IOReader
echo -e '8.1 Create Plugin IOReader - io_reader.metrics'
create_plugin '{"type": "IOReader", "config": {"plugin_name": "io_reader.metrics", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'

echo -e '8.2 Create Plugin IOReader - io_reader.logs'
create_plugin '{"type": "IOReader", "config": {"plugin_name": "io_reader.logs", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'

echo -e '8.3 Create Plugin IOReader - io_reader.app_metrics'
create_plugin '{"type": "IOReader", "config": {"plugin_name": "io_reader.app_metrics", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'

echo -e '8.4 Create Plugin IOReader - io_reader.app_requests'
create_plugin '{"type": "IOReader", "config": {"plugin_name": "io_reader.app_requests", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'

echo -e '8.5 Create Plugin IOReader - io_reader.zipkin_spans'
create_plugin '{"type": "IOReader", "config": {"plugin_name": "io_reader.zipkin_spans", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'

echo -e '8.6 Create Plugin IOReader - io_reader.mobile'
create_plugin '{"type": "IOReader", "config": {"plugin_name": "io_reader.mobile", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
################ End Plugin IOReader

# create_pipeline body
function create_pipeline() {
    if [ $# -ne 1 ]; then
        echo "wrong arguments"
        return 1
    fi

    curl ${PROTOCOL}{$LISTEN_ADDRESS}/admin/v1/pipelines \
        -i \
        -X POST \
        -H "Content-Type:application/json" \
        -H "Accept:application/json" \
        -w "\n" \
        -d "$1"

    echo -e
}


################ Begin Pipeline Metrics
echo -e '9.1 Create LinearPipeline - linear_pipeline.metrics'
create_pipeline '{"type": "LinearPipeline", "config": {"pipeline_name": "linear_pipeline.metrics", "parallelism": 1, "plugin_names": ["http_input.metrics", "io_reader.metrics", "graphite_validator.metrics", "graphite_gid_extractor.metrics", "kafka_output.metrics"]}}'

echo -e '9.2 Create LinearPipeline - linear_pipeline.logs'
create_pipeline '{"type": "LinearPipeline", "config": {"pipeline_name": "linear_pipeline.logs", "parallelism": 1, "plugin_names": ["http_input.logs", "io_reader.logs", "json_validator.logs", "json_gid_extractor.logs", "kafka_output.logs"]}}'

echo -e '9.3 Create LinearPipeline - linear_pipeline.app_metrics'
create_pipeline '{"type": "LinearPipeline", "config": {"pipeline_name": "linear_pipeline.app_metrics", "parallelism": 1, "plugin_names": ["http_input.app_metrics", "io_reader.app_metrics", "json_validator.app_metrics", "json_gid_extractor.app_metrics", "kafka_output.app_metrics"]}}'

echo -e '9.4 Create LinearPipeline - linear_pipeline.app_requests'
create_pipeline '{"type": "LinearPipeline", "config": {"pipeline_name": "linear_pipeline.app_requests", "parallelism": 1, "plugin_names": ["http_input.app_requests", "io_reader.app_requests", "json_validator.app_requests", "json_gid_extractor.app_requests", "kafka_output.app_requests"]}}'

echo -e '9.5 Create LinearPipeline - linear_pipeline.zipkin_spans'
create_pipeline '{"type": "LinearPipeline", "config": {"pipeline_name": "linear_pipeline.zipkin_spans", "parallelism": 1, "plugin_names": ["http_input.zipkin_spans", "io_reader.zipkin_spans", "json_validator.zipkin_spans", "kafka_output.zipkin_spans"]}}'

echo -e '9.6 Create LinearPipeline - linear_pipeline.mobile'
create_pipeline '{"type": "LinearPipeline", "config": {"pipeline_name": "linear_pipeline.mobile", "parallelism": 1, "plugin_names": ["http_input.mobile", "io_reader.mobile", "gwproto_adaptor.mobile", "kafka_output.mobile"]}}'
################ End Pipeline Metrics
