#!/bin/bash

# Copyright (c) 2017, The Easegress Authors
# All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Test the Easegress' basic functionality which is generating
# an HTTPServer and Pipeline for testing HTTP Requests.
set -e

# path related define.
# Note: use $(dirname $(realpath ${BASH_SOURCE[0]})) to value SCRIPTPATH is OK in linux platform, 
#       but not in MacOS.(cause there is not `realpath` in it)
# reference: https://stackoverflow.com/questions/4774054/reliable-way-for-a-bash-script-to-get-the-full-path-to-itself
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
pushd $SCRIPTPATH"/../../example" > /dev/null
EXAMPLEDIR="$SCRIPTPATH"/../../example
PRIMARYDIR=$EXAMPLEDIR"/primary-single"
EGCTL=$PRIMARYDIR"/bin/egctl"
EGBUILDER=$PRIMARYDIR"/bin/egbuilder"

# target file related define.
server="primary-single/bin/easegress-server"
eg_apiport=12381

# color define.
COLOR_NONE='\033[0m'
COLOR_INFO='\033[0;36m'
COLOR_ERROR='\033[1;31m'

# clean cleans primary-single's data and cluster data and the `go run` process.
function clean()
{
    # basic cleaning routine
    bash $EXAMPLEDIR/stop_cluster.sh
    bash $EXAMPLEDIR/clean_cluster.sh
}

# clean the cluster resource first.
clean

# start primary-single for testing.
bash $PRIMARYDIR/start.sh
try_time=0
# wait Easegress to be ready, it will retry three times.
# reference: https://unix.stackexchange.com/questions/5277/how-do-i-tell-a-script-to-wait-for-a-process-to-start-accepting-requests-on-a-po
while ! nc -z localhost $eg_apiport </dev/null
do
    sleep 5
    try_time=$(($try_time+1))
    if [[ $try_time -ge 3 ]]; then
       echo -e "\n{COLOR_ERROR}start test server $server failed${COLOR_NONE}"
       exit 1
    fi
done

# check the primary-single running status
pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ "$pid" = "" ]; then
    echo -e "\n${COLOR_ERROR}start test server $server failed${COLOR_NONE}"
    clean
    exit 2
else
    echo -e "\n${COLOR_INFO}start test server $server ok${COLOR_NONE}"
fi

# run go test
env EGCTL=$EGCTL EGBUILDER=$EGBUILDER go test -v $SCRIPTPATH

popd > /dev/null
exit 0
