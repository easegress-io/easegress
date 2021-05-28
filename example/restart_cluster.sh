#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

${SCRIPTPATH}/stop_cluster.sh
sleep 1 # Waiting for closing
${SCRIPTPATH}/start_cluster.sh

