#! /usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..

EG_SERVER=${REPOPATH}/bin/easegateway-server
EG_CLIENT=${REPOPATH}/bin/egctl

for MEMBER_PATH in writer-00{1,2,3} reader-00{4,5}
do
	echo "start ${MEMBER_PATH}"
	cd ${SCRIPTPATH}/${MEMBER_PATH} && nohup ${EG_SERVER} -f start_config.yaml &
	sleep 1
done
