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
  heycmd="${HEY} -c 100 -n 10000000 -q 1000 TESTHOST:10080/ratelimit >${SCRIPTPATH}/cotest9.res 2>&1 &"
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
function chkcurrtags(){
  currtags=$1
  # limit changed ,need some time to stabilize
  sleep ${WATISTABINTV}
  for ((i=0;i<${TESTCHKTIMES};i++)){
    bluehit=`tail -1 ${BACKENDDIR}/blue.res|awk '{printf("%.0f", $2)}'`
    greenhit=`tail -1 ${BACKENDDIR}/green.res|awk '{printf("%.0f", $2)}'`
    echo "current [${currtags}] bluehit:[$bluehit], greenhit:[$greenhit]"
    if [ "$currtags" = "blue" ] && [ $greenhit -gt 0 ];then
      echo "current blue ,has green hit"
      return 1
    fi
    if [ "$currtags" = "green" ] && [ $bluehit -gt 0 ];then
      echo "current green ,has blue hit"
      return 1
    fi
    sleep 1
  }
  return 0
}
function changeproxy(){
  currtags=$1
  sed -E "s#- \"blue\"#- \"${currtags}\"#" ${CONFIGDIR}/bluegreen-proxy-example.yaml > ${CONFIGDIR}/generate-bluegreen-proxy-example.yaml
  if [ $? -ne 0 ];then
     echo "generate yaml error"
     return 1
  fi
  ${EG1_EGCTL} --server ${EG1_API} object update -f ${CONFIGDIR}/generate-bluegreen-proxy-example.yaml
  if [ $? -ne 0 ];then
     echo "update generate-bluegreen-proxy-example error"
     return 1
  fi
  return 0
}
function adjustlimit(){
  currtags="blue"
  chkcurrtags "$currtags"
  if [ $? -ne 0 ];then
      return 1
  fi
  for ((j=0;j<${COTESTCNT};j++)){
    if [ "$currtags" = "blue" ]; then
      currtags="green"
    else
      currtags="blue"
    fi
    changeproxy "$currtags"
    if [ $? -ne 0 ];then
        echo "changeproxy bluegreen error"
        return 1
    fi
    chkcurrtags "$currtags"
    if [ $? -ne 0 ];then
      return 1
    fi
  }
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
