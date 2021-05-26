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
)
minintv=10
randintv=20

function gethost(){
	local result=$1
	key=`expr $RANDOM % ${#DEPLOYHOST[@]}`
	eval $result="${DEPLOYHOST[$key]}"
}
function getcommand(){
	local result=$1
	key=`expr $RANDOM % ${#co_command[@]}`
	eval $result='${co_command[$key]}'
}
function sethey(){
  heycmd="${HEY} -c 100 -n ${HEYCOUNT} -H 'Content-Type: application/json' -H 'X-Filter: candidate' TESTHOST:10080/proxy >${SCRIPTPATH}/cotest2.res 2>&1 &"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  return 0
}
function signalupdate(){
  for ((i=0;i<${COTESTCNT};i++))
  do
    gethost host
    getcommand cmd
    now=`date "+%Y%m%d %H:%M:%S"`
    echo "$host: $cmd ($now)"
    ssh $host $cmd
    if [ $? -ne 0 ];then
        echo "exec $cmd error"
        return 1
    fi
    intv=`expr $minintv + $RANDOM % $randintv`
    echo "next command will execute after $intv seconds"
    sleep $intv
  done
  return 0
}
function chkresult(){
  code200_hits=`grep -E '\[200\]\s[0-9]+\sresponses' ${SCRIPTPATH}/cotest2.res|sed -E 's/\s*\[200\]\s([0-9]+)\sresponses/\1/'`
  error_hits=`grep 'Error distribution' cotest2.res`
  if [ ${code200_hits} -lt ${HEYCOUNT} ] || [ -n "$error_hits" ];then
    echo "Seems lost some response, Please check"
    return 1
  fi
  return 0
}

sethey
if [ $? -ne 0 ];then
   exit 1
fi
signalupdate
if [ $? -ne 0 ];then
   exit 1
fi
chkresult
if [ $? -ne 0 ];then
   exit 1
fi

exit 0
