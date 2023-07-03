#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..


CONFIG_PATH=${SCRIPTPATH}/config
for CONFIG_FILE in ${CONFIG_PATH}/*.yaml
do
	KIND_NAME=`grep -m 1 'kind:' ${CONFIG_FILE} | cut -d ':' -f 2 | tr -d " "`
	OBJECT_NAME=`echo ${CONFIG_FILE} | sed -e "s%^${CONFIG_PATH}/%%" -e "s%\.yaml$%%"`
	if [ -n "${KIND_NAME}" ]
	then
		echo "delete ${KIND_NAME}: ${OBJECT_NAME}"
		$SCRIPTPATH/primary-001/egctl.sh delete ${KIND_NAME} ${OBJECT_NAME}
	fi
done
