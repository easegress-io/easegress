#!/bin/bash
pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

set -e

PROCESS="easegateway"
DATE=`date +'%m-%d-%Y-%T'`
LOG_DIR="${SCRIPTPATH}/eg-cpu-memory-log/"
FILE="${LOG_DIR}/eg-${DATE}.log"

mkdir  -p ${LOG_DIR}
touch ${FILE}
echo "start monitoring easegateway-server every 5s to file ${FILE}"
pidstat -ru -C "${PROCESS}" -h 5 | awk 'BEGIN {OFS=","} NR==3 {print "ts", $3, $8, $13, $14} NR != 1 &&( $0 !~ /^#|^$/ ) {print strftime("%H:%M:%S"), $3, $7/32, $12, $13; fflush(); }' | tee $FILE
