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
  heycmd="${HEY} -c 100 -n 4000000 -q 1000 TESTHOST:10080/urlratelimit/activity/1 >${SCRIPTPATH}/cotest41.res 2>&1 &"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  heycmd="${HEY} -c 100 -n 4000000 -q 1000 TESTHOST:10080/urlratelimit/activity/2 >${SCRIPTPATH}/cotest42.res 2>&1 &"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  return 0
}
function stophey(){
  heycmd="${HEY} -c 100 -n 4000000 -q 1000 ${BACKENDHOST}:10080/urlratelimit"
  PID=`ps -ef|grep "$heycmd"|grep -v grep|awk '{print $2}'`
  if [ "$PID" != "" ];then
    echo $PID|xargs kill -9
  fi
  echo "stop hey successed."
  return 0
}
function chkcurrlimit(){
  path1=$1
  path1limit=$2
  path2=$3
  path2limit=$4
  # limit changed ,need some time to stabilize
  sleep ${WATISTABINTV}
  exceedtimes=0
  for ((i=0;i<${TESTCHKTIMES};i++)){
    currenttime=`tail -1 ${BACKENDDIR}/urlratelimit.res|awk '{print $1}'`
    currenthit=`tail -4 ${BACKENDDIR}/urlratelimit.res|grep $currenttime|awk '{printf("%s:%.0f ", $4,$2);tot+=$2}'|xargs echo`
    echo "current tpslimit:[$path1limit $path2limit], currenthit:[$currenthit]"
    path1testhit=`echo $currenthit|sed -E "s#.*${path1}\:([0-9]+).*#\1#"`
    path1max=`expr $path1limit \* 130 \/ 100`
    if [ $path1testhit -gt $path1max ];then
      exceedtimes=`expr $exceedtimes + 1`
    fi
    path2testhit=`echo $currenthit|sed -E "s#.*${path2}\:([0-9]+).*#\1#"`
    path2max=`expr $path2limit \* 130 \/ 100`
    if [ $path2testhit -gt $path2max ];then
      exceedtimes=`expr $exceedtimes + 1`
    fi
    if [ $exceedtimes -ge 6 ];then
      echo "exceed tps limit 6 times"
      return 1
    fi
    sleep 1
  }
  return 0
}
function changeproxy(){
  newintv=$1
  newpath1tps=`expr 80 + $RANDOM % $newintv`
  sed -E "s#tps: 80#tps: $newpath1tps#" ${CONFIGDIR}/urlratelimit-proxy-example.yaml > ${CONFIGDIR}/generate-urlratelimit-proxy-example1.yaml
  if [ $? -ne 0 ];then
     echo "generate yaml error1"
     return 1
  fi
  newpath2tps=`expr 50 + $RANDOM % $newintv`
  sed -E "s#tps: 50#tps: $newpath2tps#" ${CONFIGDIR}/generate-urlratelimit-proxy-example1.yaml > ${CONFIGDIR}/generate-urlratelimit-proxy-example.yaml
  if [ $? -ne 0 ];then
     echo "generate yaml error2"
     return 1
  fi
  ${EG1_EGCTL} --server ${EG1_API} object update -f ${CONFIGDIR}/generate-urlratelimit-proxy-example.yaml
  if [ $? -ne 0 ];then
     echo "update generate-urlratelimit-proxy-example error"
     return 1
  fi
  rm -f  ${CONFIGDIR}/generate-urlratelimit-proxy-example1.yaml
  chkcurrlimit "/urlratelimit/activity/1" $newpath1tps "/urlratelimit/activity/2" $newpath2tps
  if [ $? -ne 0 ];then
    return 1
  fi
  return 0
}
function adjustlimit(){
# current tps limit 100
  chkcurrlimit "/urlratelimit/activity/1" 80 "/urlratelimit/activity/2" 50
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
