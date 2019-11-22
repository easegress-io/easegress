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
  heycmd="${HEY} -c 100 -n ${HEYCOUNT} -q 1000 -H 'IsValid: yes' TESTHOST:10080/ratelimit >${SCRIPTPATH}/cotest51.res 2>&1"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  heycmd="${HEY} -c 100 -n ${HEYCOUNT} -q 1000  -H 'IsValid: no' TESTHOST:10080/ratelimit >${SCRIPTPATH}/cotest52.res 2>&1"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  return 0
}
function chkresult(){
  code200_hits=`grep -E '\[200\]\s[0-9]+\sresponses' ${SCRIPTPATH}/cotest51.res|sed -E 's/\s*\[200\]\s([0-9]+)\sresponses/\1/'`
  if [ ${code200_hits} -lt ${HEYCOUNT} ] ;then
    echo "valid request lost, Please check"
    return 1
  fi
  u_code200_hits=`grep -E '\[200\]\s[0-9]+\sresponses' ${SCRIPTPATH}/cotest52.res|sed -E 's/\s*\[200\]\s([0-9]+)\sresponses/\1/'`
  if [ -n "${u_code200_hits}" ] ;then
    echo "unvalid request success, Please check"
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
