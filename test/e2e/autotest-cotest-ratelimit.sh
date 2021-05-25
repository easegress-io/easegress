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

function sethey(){
  heycmd="${HEY} -c 100 -n 10000000 -q 1000 TESTHOST:10080/ratelimit >${SCRIPTPATH}/cotest3.res 2>&1 &"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  return 0
}
function stophey(){
  heycmd="${HEY} -c 100 -n 10000000 -q 1000 ${BACKENDHOST}:10080/ratelimit"
  PID=`ps -ef|grep "$heycmd"|grep -v grep|awk '{print $2}'`
  if [ "$PID" != "" ];then
    echo $PID|xargs kill -9
  fi
  echo "stop hey successed."
  return 0
}
function chkcurrlimit(){
  tpslimit=$1
  exceedtimes=0
  # limit changed ,need some time to stabilize
  sleep ${WATISTABINTV}
  for ((i=0;i<${TESTCHKTIMES};i++)){
    currenthit=`tail -1 ${BACKENDDIR}/ratelimit.res|awk '{printf("%.0f", $2)}'`
    echo "current tpslimit:[$tpslimit], currenthit:[$currenthit]"
    tpsmax=`expr $tpslimit \* 130 \/ 100`
    if [ $currenthit -gt $tpsmax ];then
      exceedtimes=`expr $exceedtimes + 1`
    fi
    if [ $exceedtimes -ge 3 ];then
      echo "exceed tps limit 3 times"
      return 1
    fi
    sleep 1
  }
  return 0
}
function changeproxy(){
  newtps=$1
  sed -E "s#tps: 100#tps: $newtps#" ${CONFIGDIR}/ratelimit-proxy-example.yaml > ${CONFIGDIR}/generate-ratelimit-proxy-example.yaml
  if [ $? -ne 0 ];then
     echo "generate yaml error"
     return 1
  fi
  ${EG1_EGCTL} --server ${EG1_API} object update -f ${CONFIGDIR}/generate-ratelimit-proxy-example.yaml
  if [ $? -ne 0 ];then
     echo "update generate-ratelimit-proxy-example error"
     return 1
  fi
  return 0
}
function adjustlimit(){
# current tps limit 100
  chkcurrlimit 100
  if [ $? -ne 0 ];then
      return 1
  fi
  for tps in 200 500 800 1000 700 400 100
  do
    changeproxy $tps
    if [ $? -ne 0 ];then
        echo "changeproxy ratelimit $tps error"
        return 1
    fi
    chkcurrlimit $tps
    if [ $? -ne 0 ];then
      return 1
    fi
  done
  return 0
}

sethey
if [ $? -ne 0 ];then
   exit 1
fi
adjustlimit
if [ $? -ne 0 ];then
  stophey
  exit 1
fi
stophey
exit 0
