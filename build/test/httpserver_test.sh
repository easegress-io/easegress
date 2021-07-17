#!/bin/bash
# Test the Easegress' basic functionality which is generating
# an HTTPServer and Pipeline for testing HTTP Requests.
# author:   benjaminwu 
# date:     2021/0716

# path related define.
SCRIPTPATH=`pwd -P`
pushd $SCRIPTPATH"/../../example" > /dev/null
EXAMPDIR=`pwd -P`
WRITER01DIR=$EXAMPDIR"/writer-001"

# target file related define.
server=$WRITER01DIR/bin/easegress-server
backend=$EXAMPDIR/backend-service/mirror/mirror.go

# color define.
COLOR_NONE='\033[0m'
COLOR_INFO='\033[0;36m'
COLOR_ERROR='\033[1;31m'

# clean cleans writer-001's cluster data and the `go run` process.
function clean()
{
     # basic cleaning routine
     bash $EXAMPDIR/stop_cluster.sh 
     bash $EXAMPDIR/clean_cluster.sh


     # clean the go mirror backend
     if [ "$1" != "" ];then
        echo -e "\n${COLOR_INFO}finish mirror running pid=$1${COLOR_NONE}"
	child_pid=`pgrep -P $1`

	if [ "$child_pid" != "" ]; then
	    kill -9 $child_pid
            echo -e "\n${COLOR_INFO}finish mirror running child process pid=$child_pid${COLOR_NONE}"
	fi

	kill -9 $1
     fi

}

# clean the cluster resource first.
clean

# start writer01 for testing. 
start_svr=`$WRITER01DIR/start.sh `

# wait Easegress to be ready
sleep 2

# check the writer01 running status
pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ "$pid" = "" ]; then
    echo -e "\n${COLOR_ERROR}start test server $server failed${COLOR_NONE}"
    clean
    exit 2
fi

# create HTTPServer
echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: pipeline-demo' | $WRITER01DIR/egctl.sh object create

#  create Pipeline
echo '
name: pipeline-demo
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin' | $WRITER01DIR/egctl.sh object create

# run the backend.
(go run $backend & )
sleep 2

# check the mirror backend running status.
mirror_pid=`ps -eo pid,args|grep mirror.go |grep -v grep |awk '{print $1}'`
if [ "$mirror_pid" = "" ]; then
	echo  -e "\n${COLOR_ERROR}start test backend server failed, command=go run $backend${COLOR_NONE}"
	clean
	exit 3 
else
        echo -e "\n${COLOR_INFO}start mirror, its pid=$mirror_pid${COLOR_NONE}"
fi

# test backend routed by HTTPServer and Pipeline with curl.
response=$(curl --write-out '%{http_code}' --silent --output /dev/null http://127.0.0.1:10080/pipeline -d'hello easegress')
if [ "$response" != "200" ]; then
	echo "curl http server failed, response code "$response
	clean $mirror_pid
	exit 4
else 
       echo -e "\n${COLOR_INFO}test succ${COLOR_NONE}"
fi

# clean all created resources.
clean $mirror_pid

popd > /dev/null