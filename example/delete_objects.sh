#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..


CONFIG_PATH=${SCRIPTPATH}/config
for CONFIG_FILE in ${CONFIG_PATH}/*.yaml
do
	OBJECT_NAME=`echo ${CONFIG_FILE} | sed -e "s%^${CONFIG_PATH}/%%" -e "s%\.yaml$%%"`
	echo "delete object: ${OBJECT_NAME}"
	$SCRIPTPATH/primary-001/egctl.sh object delete ${OBJECT_NAME}
done
