#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..


CONFIG_PATH=${SCRIPTPATH}/config
for CONFIG_FILE in ${CONFIG_PATH}/*.yaml
do
	echo "update object: ${CONFIG_FILE}"
	$SCRIPTPATH/writer-001/egctl.sh object update -f ${CONFIG_FILE}
done
