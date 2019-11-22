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
  heycmd="${HEY} -c 100 -n ${HEYCOUNT} -q 1000 TESTHOST:10080/ratelimit >${SCRIPTPATH}/cotest7.res 2>&1"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  return 0
}
function chkresult(){
  code200_hits=`grep -E '\[200\]\s[0-9]+\sresponses' ${SCRIPTPATH}/cotest7.res|sed -E 's/\s*\[200\]\s([0-9]+)\sresponses/\1/'`
  backend_hits=`tail -1 ${BACKENDDIR}/backend1.res|awk '{printf("%.0f", $3)}'`
  if [ ${code200_hits} -ne ${HEYCOUNT} ] ;then
    echo "request lost, Please check"
    return 1
  fi
  if [ ${backend_hits} -eq ${HEYCOUNT} ] ;then
    echo "request not use memcache, Please check"
    return 1
  fi
  currenthit=`tail -1 ${BACKENDDIR}/backend2.res|awk '{printf("%.0f", $3)}'`
  if [ -z "${currenthit}" ]  || [ ${currenthit} -eq 0 ];then
    echo "not mirror request, Please check"
    return 1
  fi
  return 0
}

sethey
if [ $? -ne 0 ];then
   exit 1
fi
chkresult
if [ $? -ne 0 ];then
   exit 1
fi

exit 0
