#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

CONFIG_PATH=${SCRIPTPATH}/config
for CONFIG_FILE in ${CONFIG_PATH}/*.yaml
do
	echo "apply resources: ${CONFIG_FILE}"
	$SCRIPTPATH/primary-001/egctl.sh apply -f ${CONFIG_FILE}
done
