#!/usr/bin/env bash

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

# Color
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
# no color 
NC='\033[0m'

echo -e ${GREEN}"status:"${NC}
${SCRIPTPATH}/primary-001/status.sh

echo -e ${GREEN}"members:"${NC}
${SCRIPTPATH}/primary-001/egctl.sh get member

echo -e ${GREEN}"objects:"${NC}
${SCRIPTPATH}/primary-001/egctl.sh get all

