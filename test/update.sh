#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..

EG_SERVER=${REPOPATH}/bin/easegateway-server
EG_CLIENT=${REPOPATH}/bin/egctl

CONFIG_PATH=${SCRIPTPATH}/config
for CONFIG_FILE in ${CONFIG_PATH}/*.yaml
do
	echo "update object: ${CONFIG_FILE}"
	${EG_CLIENT} --server 127.0.0.1:12381 object update -f ${CONFIG_FILE}
done
