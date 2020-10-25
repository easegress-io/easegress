#!/usr/bin/env bash


#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null


${SCRIPTPATH}/writer-001/egctl.sh object status list
