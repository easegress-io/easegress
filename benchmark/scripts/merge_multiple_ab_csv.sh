#!/bin/bash
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
    echo -e "${COLOR_ERROR}${NOW} ▶ ${1}${COLOR_NONE}"
}

function info() {
    NOW=$(date +'%Y/%m/%d %H:%M:%S:%3N')
    echo -e "${COLOR_INFO}${NOW} ▶ ${1}${COLOR_NONE}"
}
if [ $# -lt 2 ]; then
     error "please specify multiple apache benmark csv files to merge"
     exit -1
fi

DATE=`date +"%Y_%m_%d_%H_%M_%S"`
FILE="merged_"${DATE}".csv"

info "merging $# files, headers are appended one by one"
awk 'BEGIN { FS = ","; OFS="," } NR==FNR { a[$1]=$0; next} $1 in a {a[$1]=a[$1]OFS$2} END {for (k in a ) print a[k] }' "$@" | sort -k 1 -g > ${FILE}
info "merged finished, please check ${FILE}"
