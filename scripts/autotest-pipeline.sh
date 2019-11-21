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

function checkports(){
    for port in ${TESTPORTS}
    do
	isconflict=`netstat -an|grep ":${port} "|grep LISTEN|wc -l`
	if [ $isconflict -ne 0 ];then
	    echo "port: ${port} conflict, please adjust."
	    return 1
	fi
    done
    if [ $? -ne 0 ];then
        return 1
    fi
    return 0
}
function buildbackend(){
    cd ${BACKENDDIR}
    go build mirror.go
    if [ $? -ne 0 ];then
        echo "build mirror error"
        return 1
    fi
    go build remote.go
    if [ $? -ne 0 ];then
        echo "build remote error"
        return 1
    fi
    mv mirror backend_mirror
    mv remote backend_remote
    echo "build backend successed."
    return 0
}

function runbackend(){
    cd ${BACKENDDIR}
    nohup ./backend_mirror &
    if [ $? -ne 0 ];then
        echo "run mirror error"
        return 1
    fi
    nohup ./backend_remote &
    if [ $? -ne 0 ];then
        echo "run remote error"
        return 1
    fi
    echo "run backend successed."
    return 0
}

function termbackend(){
    PID=`ps -ef|grep backend_mirror|grep -v grep|awk '{print $2}'`
    if [ "$PID" != "" ];then
        echo $PID|xargs kill -9
    fi
    PID=`ps -ef|grep backend_remote|grep -v grep|awk '{print $2}'`
    if [ "$PID" != "" ];then
        echo $PID|xargs kill -9
    fi
    echo "term backend successed."
    return 0
}

function createobject(){
    cd ${CONFIGDIR}
    rm -f generate*.yaml
    grep "^name:" *.yaml|sed -E 's/.*name: (.*)/\1/'|while read objname
    do
        sed -E "s#http://127.0.0.1#${BACKENDHOST}#" ${objname}.yaml > generate-${objname}.yaml
        if [ $? -ne 0 ];then
            echo "generate yaml error"
            return 1
        fi
        ${EG1_EGCTL} --server ${EG1_API} object create -f generate-${objname}.yaml
        if [ $? -ne 0 ];then
            echo "create ${objname} error"
            return 1
        fi
    done
    if [ $? -ne 0 ];then
        return 1
    fi
    echo "create object success"
    return 0
}

function deleteobject(){
    cd ${CONFIGDIR}
    rm -f generate*.yaml
    grep "^name:" *.yaml|sed -E 's/.*name: (.*)/\1/'|while read objname
    do
        ${EG1_EGCTL} --server ${EG1_API} object delete ${objname}
        if [ $? -ne 0 ];then
            echo "delete ${objname} error"
            return 1
        fi
    done
    if [ $? -ne 0 ];then
        return 1
    fi
    echo "delete object success"
    return 0
}

function checkobject(){
    curl -v ${TESTHOST}:10080/pipeline?epoch="$(date +%s)" \
         -H 'Content-Type: application/json' \
         -H 'X-Filter: candidate' \
         -d 'I am pipeline requester'
    if [ $? -ne 0 ];then
        echo "check object pipeline error"
        return 1
    fi
    curl -v ${TESTHOST}:10080/proxy?epoch="$(date +%s)" \
         -H 'Content-Type: application/json' \
         -H 'X-Filter: mirror' \
         -d 'I am proxy requester'
    if [ $? -ne 0 ];then
        echo "check object proxy error"
        return 1
    fi
    curl -v ${TESTHOST}:10080/remote
    if [ $? -ne 0 ];then
        echo "test object remote error"
        return 1
    fi
    echo "check object success"
    return 0
}

function testobject(){
    cd ${BUILDDIR}/scripts
    ./autotest-cotest.sh
    if [ $? -ne 0 ];then
        return 1
    fi
    echo "test success"
    return 0
}

function pipeline(){
    checkports
    if [ $? -ne 0 ];then
        exit 1
    fi

    buildbackend
    if [ $? -ne 0 ];then
        exit 1
    fi

    runbackend
    if [ $? -ne 0 ];then
        exit 1
    fi

    createobject
    if [ $? -ne 0 ];then
        deleteobject
        termbackend
        exit 1
    fi

    checkobject
    if [ $? -ne 0 ];then
        deleteobject
        termbackend
        exit 1
    fi

    testobject
    if [ $? -ne 0 ];then
        deleteobject
        termbackend
        exit 1
    fi

    deleteobject

    termbackend

    exit 0
}

pipeline
