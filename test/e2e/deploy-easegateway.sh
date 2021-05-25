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
if [ -n "${ROLLBACK_ID}" ];then
  BUILD_ID=${ROLLBACK_ID}
else
  cp ${BUILDDIR}/bin/easegateway-server ${PRODLABDIR}/easegateway-server.${BUILD_ID}
  cp ${BUILDDIR}/bin/egctl ${PRODLABDIR}/egctl.${BUILD_ID}
fi

for i in "${DEPLOYHOST[@]}";
do
    scp ${PRODLABDIR}/easegateway-server.${BUILD_ID}  "${i}:${DEPLOYDIR}/easegateway-server.deploy"
    if [ $? -ne 0 ];then 
        exit 1
    fi
    scp ${PRODLABDIR}/egctl.${BUILD_ID}  "${i}:${DEPLOYDIR}/egctl.deploy"
    if [ $? -ne 0 ];then
        exit 1
    fi
    ssh ${i} "mv ${DEPLOYDIR}/easegateway-server.deploy ${DEPLOYDIR}/easegateway-server"
    if [ $? -ne 0 ];then
        exit 1
    fi
    ssh ${i} "mv ${DEPLOYDIR}/egctl.deploy ${DEPLOYDIR}/egctl"
    if [ $? -ne 0 ];then
        exit 1
    fi
    ssh ${i} "PID=\`cat ${DEPLOYDIR}/easegateway.pid\`;LSPS=\`ps -ef|grep \$PID|grep easegateway|wc -l\`;if [ \"\$LSPS\" = \"1\" ];then kill -USR2 \$PID; else cd ${DEPLOYDIR};nohup ./start.sh>/dev/null 2>&1 &; fi" 
    if [ $? -ne 0 ];then
        exit 1
    fi
done

sleep 10
${SCRIPTPATH}/deploy-status.sh
exit $?
