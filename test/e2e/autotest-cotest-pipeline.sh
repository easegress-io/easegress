#!/bin/bash
pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

if [ -f ${SCRIPTPATH}/deploy.env ];
then
  source ${SCRIPTPATH}/deploy.env
else
  echo "Can't found deploy.env file"
  exit 1
fi

if [ -f ${SCRIPTPATH}/autotest.env ];
then
  source ${SCRIPTPATH}/autotest.env
else
  echo "Can't found autotest.env file"
  exit 1
fi

co_command=(
"PID=\`cat ${DEPLOYDIR}/easegress.pid\`;kill -USR2 \$PID"
"${HEY} -c 100 -n 100000 -H 'Content-Type: application/json' -H 'X-Filter: candidate' TESTHOST:10080/pipeline"
"${HEY} -c 100 -n 100000 -H 'Content-Type: application/json' -H 'X-Filter: candidate' TESTHOST:10080/proxy"
"${HEY} -c 100 -n 100000 TESTHOST:10080/remote"
)
minintv=10
randintv=20

function gethost(){
	local result=$1
	key=`expr $RANDOM % ${#DEPLOYHOST[@]}`
	eval $result="${DEPLOYHOST[$key]}"
}
function gettesthost(){
	local result=$1
	key=`expr $RANDOM % ${#TESTHOST[@]}`
	eval $result="${TESTHOST[$key]}"
}
function getcommand(){
	local result=$1
	key=`expr $RANDOM % ${#co_command[@]}`
	eval $result='${co_command[$key]}'
}
for ((i=0;i<${COTESTCNT};i++)) 
do
	gethost host
	getcommand cmd
	now=`date "+%Y%m%d %H:%M:%S"`
        ishey=`echo $cmd|grep hey|wc -l`
	if [ $ishey -ne 0 ];then
	    gettesthost testhost
            cmd2=`echo $cmd|sed -E "s#TESTHOST#${testhost}#"`
	    echo "exec $cmd2 ($now)"
	    eval $cmd2
	else
	    echo "$host: $cmd ($now)"
	    ssh $host $cmd
	fi
        if [ $? -ne 0 ];then
            echo "exec $cmd2 error"
            exit 1
        fi
	intv=`expr $minintv + $RANDOM % $randintv`
	echo "next command will execute after $intv seconds"
	sleep $intv
done
if [ $? -ne 0 ];then
   exit 1
fi
exit 0
