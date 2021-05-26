1. Setup Jenkins pipeline script file `jenkinsfile`. Need set source pull and build directory.
2. Setup `deploy.env` , It's configuation deploy host and directory, and health status check api.(The file move to easegress-configuration repo)
```
DEPLOY_ENV=km
DEPLOY_ENV=km
if [ "$BUILDDIR" = "" ];then
    BUILDDIR=${HOME}/backdemo/easegress
fi
if [ "$PRODLABDIR" = "" ];then
    PRODLABDIR=${HOME}/backdemo/prodlab/bin
fi
# set path to store execute product file version
PRODLABDIR=/home/ubuntu/backdemo/prodlab/bin
# ssh host array, one host per row
DEPLOYHOST=(
"ubuntu@192.168.50.101"
"ubuntu@192.168.50.102"
"ubuntu@192.168.50.103"
"ubuntu@192.168.50.104"
"ubuntu@192.168.50.105"
)
# set node1 info, for member list check
EG1_SERVER=${DEPLOYDIR}/easegress-server
EG1_EGCTL=${DEPLOYDIR}/egctl
EG1_CONFIG=${DEPLOYDIR}/eg-01.yaml
EG1_API=192.168.50.101:12381
```
