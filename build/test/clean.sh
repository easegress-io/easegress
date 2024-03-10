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

# clean cleans primary-single's data and cluster data and the `go run` process.
function clean()
{
    # basic cleaning routine
    bash $EXAMPLEDIR/stop_cluster.sh
    bash $EXAMPLEDIR/clean_cluster.sh
}

# clean the cluster resource.
clean