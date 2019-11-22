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
  heycmd="${HEY} -c 100 -n ${HEYCOUNT} -q 1000 TESTHOST:10080/ >${SCRIPTPATH}/cotest8.res 2>&1"
  cmd2=`echo $heycmd|sed -E "s#TESTHOST#${BACKENDHOST}#"`
  eval $cmd2
  if [ $? -ne 0 ];then
      echo "exec $cmd2 error"
      return 1
  fi
  return 0
}
function chkresult(){
  chkresult=`awk 'BEGIN{a=0;errcnt=0}{if ($4=="true"){ a=1; errcnt=0}; if ($4 == "false"){ a=0; errcnt=0};
        if (a==1){ if ( $2 == 0 ) errcnt++;if ( errcnt>3 ) printf("circuit close, requst 0 exceed 3 times\n"); }
        else { if ( $2 != 0 && $2 != 50 ) errcnt++;if ( errcnt>3 ) printf("circuit open, requst not 0 exceed 3 times\n"); };}' circuitbreaker.res|wc -l`
  if [ $chkresult -ne 0 ]; then
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
