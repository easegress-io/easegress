1. To run autoexec*.sh need `deploy.env` and `autotest.env`, this file pull from easegateway-configuration repository by jenkins.If Manual run testscripts without those two env file, you need create its.
2. `deploy.env` : It's configuation deploy host and directory, and health status check api.
    ```
    DEPLOY_ENV=km
    DEPLOY_ENV=km
    if [ "$BUILDDIR" = "" ];then
        BUILDDIR=${HOME}/backdemo/easegateway
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
    EG1_SERVER=${DEPLOYDIR}/easegateway-server
    EG1_EGCTL=${DEPLOYDIR}/egctl
    EG1_CONFIG=${DEPLOYDIR}/eg-01.yaml
    EG1_API=192.168.50.101:12381
    ```
3. `autotest.env`: It's configuation run autotest host and directory, and some params.
    ```
    AUTOTEST_ENV=km
    TESTDIR=/home/ubuntu/backdemo/easegateway/test
    BACKENDDIR=${TESTDIR}/backend-service
    CONFIGDIR=${TESTDIR}/config
    TESTHOST=(
    "http://192.168.50.101"
    "http://192.168.50.102"
    "http://192.168.50.103"
    "http://192.168.50.104"
    "http://192.168.50.105"
    )
    # all autotest need use ports . if conflict, need change first
    TESTPORTS="9091 9092 9093 9094 9095 9096 9097 9098 10000 10080"
    # the host to run autotest backend service
    BACKENDHOST="http://192.168.50.105"
    # tims to repeat test one feature
    COTESTCNT=5
    # when change proxy params , wait secends to stabilize
    WATISTABINTV=10
    # script chk test result per secends, this setup chk times
    TESTCHKTIMES=5
    # hey tool path
    HEY=${HOME}/go/bin/hey
    # setup hey send request count
    HEYCOUNT=1000000
    ```
4. this 10 features test csae :
    - autotest:pipeline
      - run `autotest-pipeline.sh` test a simple httppipeline work 
    - autotest:graceupdate
      - run `autotest-graceupdate.sh` test will random send usr2 singal to easegateway node trigger grace update 
    - autotest:ratelimit
      - run `autotest-ratelimit.sh` test will change some tpslimit to test ratelimiter feature
    - autotest:urlratelimit
      - run `autotest-urlratelimit.sh` test will change some tpslimit to test urlratelimiter feature
    - autotest:vaildator
      - run `autotest-vaildator.sh` test will send `valid:yes` and `valid:no` header to test vaildator feature
    - autotest:requestAdaptor/responseAdaptor
      - run `autotest-adaptor.sh` test will check requestAdaptor and responseAdaptor add/set/del header key-value feature
    - autotest:backend/memoryPool/mirrorPool 
      - run `autotest-backend.sh` test will check mainpool/memorycache/mirrorpool backend feature
    - autotest:circuitBreaker 
      - run `autotest-circuitbreaker.sh` test will send request to a special backend, it is turn back httpcode 200 and 503. Check CircuitBreaker feature
    - autotest:BlueGreen 
      - run `autotest-bluegreen.sh` test will turn to change backend servertags between blue and green, check bluegreen deloyment feature.
    - autotest:canary 
      - run `autotest-canary.sh` test will change backend candidate perMill value, check canary deloyment feature.
    each one can run single.