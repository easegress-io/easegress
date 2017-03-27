#!/usr/bin/env bash

echo -e "#### Taste Gateway Admin APIs ####"

################

echo -e "\n> 1. Requests on metadata endpoints."

echo -e "\n>> Query available plugin types."
curl http://127.0.0.1:9090/admin/v1/plugin-types -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Query available pipeline types."
curl http://127.0.0.1:9090/admin/v1/pipeline-types -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

################

echo -e "\n> 2. Requests on plugin endpoints."

echo -e "\n>> Cleanup test environment."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X DELETE -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Query existing plugins."
curl http://127.0.0.1:9090/admin/v1/plugins -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"name_pattern": "", "types": []}'

echo -e "\n>> Create new test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/entry-point", "method": "GET", "headers_enum": {"foo": ["bar"]}}}'

echo -e "\n>> Query created test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Update test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/updated-entry-point", "method": "POST", "headers_enum": {"new": ["one"]}}}'

echo -e "\n>> Query updated test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Delete test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X DELETE -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Query deleted test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"


echo -e "\n> 3. Requests on pipeline endpoints."

echo -e "\n>> Cleanup test environment."
curl http://127.0.0.1:9090/admin/v1/pipelines/test-httppipeline -X DELETE -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X DELETE -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Query existing pipelines."
curl http://127.0.0.1:9090/admin/v1/pipelines -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"name_pattern": "", "types": []}'

echo -e "\n>> Create new test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/entry-point", "method": "GET", "headers_enum": {"foo": ["bar"]}}}'

echo -e "\n>> Create new test pipeline."
curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-httppipeline", "plugin_names": ["test-httpinput"], "parallelism": 1}}'

echo -e "\n>> Query created test pipeline."
curl http://127.0.0.1:9090/admin/v1/pipelines/test-httppipeline -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Update test pipeline."
curl http://127.0.0.1:9090/admin/v1/pipelines -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-httppipeline", "plugin_names": ["test-httpinput"], "parallelism": 20}}'

echo -e "\n>> Query updated test pipeline."
curl http://127.0.0.1:9090/admin/v1/pipelines/test-httppipeline -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Delete test pipeline."
curl http://127.0.0.1:9090/admin/v1/pipelines/test-httppipeline -X DELETE -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Query deleted test pipeline."
curl http://127.0.0.1:9090/admin/v1/pipelines/test-httppipeline -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Delete test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X DELETE -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\n>> Query deleted test plugin."
curl http://127.0.0.1:9090/admin/v1/plugins/test-httpinput -X GET -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n"

echo -e "\nDone."
