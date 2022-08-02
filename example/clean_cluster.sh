#! /usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

for MEMBER_PATH in primary-00{1,2,3} secondary-00{4,5} primary-single
do
	${SCRIPTPATH}/${MEMBER_PATH}/stop.sh -f
	rm -fr ${SCRIPTPATH}/${MEMBER_PATH}/{nohup.out,log,member,data,data_bak,easegress.pid,running_objects.yaml,nacos*}
done
