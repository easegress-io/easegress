#! /usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..


for MEMBER_PATH in writer-00{1,2,3} reader-00{4,5}
do
	echo "start ${MEMBER_PATH}"
    ${SCRIPTPATH}/${MEMBER_PATH}/start.sh
done
