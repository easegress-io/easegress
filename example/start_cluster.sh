#! /usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

REPOPATH=${SCRIPTPATH}/..


for MEMBER_PATH in primary-00{1,2,3} secondary-00{4,5}
do
	echo "start ${MEMBER_PATH}"
    ${SCRIPTPATH}/${MEMBER_PATH}/start.sh
done
