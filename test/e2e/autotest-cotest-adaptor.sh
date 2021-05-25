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

function setclient(){
  sendtimes=`expr $COTESTCNT \* 1000`
  for ((i=0;i<${sendtimes};i++))
  do
    heycmd="${BACKENDDIR}/backend_adaptor -t client -h TESTHOST -p 10080"
    cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
    eval $cmd2
    if [ $? -ne 0 ];then
        echo "exec $cmd2 error"
        return 1
    fi
  done
  if [ $? -ne 0 ];then
    return 1
  fi
  return 0
}
function chkresult(){
  errtimes=`grep 'Not' ${BACKENDDIR}/adaptor.res|wc -l`
  if [ ${errtimes} -gt 0 ] ;then
    echo "adaptor server report error, Please check"
    return 1
  fi
  return 0
}

setclient
if [ $? -ne 0 ];then
   exit 1
fi
chkresult
if [ $? -ne 0 ];then
   exit 1
fi

exit 0
