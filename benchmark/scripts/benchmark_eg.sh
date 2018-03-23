#!/bin/bash
set -x
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

## Use FAMILY and TITLE to distinguish different benchmark purpose
## They are used as directoy names, more details, see  ${LOG_DIR}
FAMILY="default"
TITLE=""
URL='http://10.0.0.10/test?sleep=100&len=3000'

CONCURRENCY=1000
NUMBER=`expr ${CONCURRENCY} \* 499`

POSITIONAL=()
while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    -c|--concurrency)
    CONCURRENCY="$2"
    shift # past argument
    shift # past value
    ;;
    -n|--number)
    NUMBER="$2"
    shift # past argument
    shift # past value
    ;;
    -f|--family)
    FAMILY="$2"
    shift # past argument
    shift # past value
    ;;
    -t|--title)
    TITLE="$2"
    shift # past argument
    shift # past value
    ;;
    -u|--url)
    URL="$2"
    shift # past argument
    shift # past value
    ;;
    *)    # unknown option
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

if [ -z "${TITLE}" ]; then
    error "please use -t to specify benchmark title, such as -t nethttp"
    exit -1
fi

DATE=`date +'%Y-%m-%d'`
LOG_DIR=${SCRIPTPATH}/"eg-benchmark-result"/${DATE}/${FAMILY}/${TITLE}/
FILE=${LOG_DIR}/${DATE}.log

BEGIN_TIME=`date +'%T'`
CSV_FILE=${LOG_DIR}/c-${CONCURRENCY}-${BEGIN_TIME}.csv
RESULT_FILE=${LOG_DIR}/c-${CONCURRENCY}-${BEGIN_TIME}.ab

mkdir -p ${LOG_DIR}
info "benchmark log will be write to dir : ${LOG_DIR}"

echo "begin time: ${BEGIN_TIME}" > ${RESULT_FILE}
ab -k -c ${CONCURRENCY} -e ${CSV_FILE} -n ${NUMBER} -p /dev/null ${URL} >> ${RESULT_FILE}

END_TIME=`date +'%T'`

echo "end time: ${END_TIME}" >> ${RESULT_FILE}

info "benchmark finished"
