#!/bin/bash
set -e

pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

set -e

COLOR_NONE='\033[0m'
COLOR_INFO='\033[0;36m'
COLOR_ERROR='\033[1;31m'

function error() {
    NOW=$(date +'%Y/%m/%d %H:%M:%S:%3N')
    echo -e "${COLOR_ERROR}${NOW} � ${1}${COLOR_NONE}"
}

function info() {
    NOW=$(date +'%Y/%m/%d %H:%M:%S:%3N')
    echo -e "${COLOR_INFO}${NOW} � ${1}${COLOR_NONE}"
}
if [ $# -lt 1 ]; then
     error "please specify multiple apache benmark csv files to merge"
     exit -1
fi

DATE=`date +"%Y_%m_%d_%H_%M_%S"`
FILE=${DATE}"_qps_rt.csv"
info "merging $# files"
for f in "$@"
do
    awk 'BEGIN {FS=":"; OFS="," } $1 ~/per second/ || /\[ms\] \(mean\)/ ' $f | sed -r 's/[^0-9]+([0-9]+\.?[0-9]*).*/\1/g' | tr '\n' ',' >> ${FILE}
    echo "" >> ${FILE}
done
info "finished, please check ${FILE}"
