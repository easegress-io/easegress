## Background
It's error prone to start/stop the easegress executable binary directly in a multi-node easegress group. This is because the embedded etcd cluster may be corrupted if one peer node starts more than one instances.

This **sbin** script suite specifies and regulates the basic directory structure, config file path, binary path, and provide **start.sh** and **stop.sh** to assist the admin does not start/stop the **easegress process** in any wrong way.

## Usage

1. Let's define the **APPDIR** is the absolute path to the directory containing **start.sh** and **stop.sh**.
2. Copy the executable binary **easegress-server** and **egctl** into **$APPDIR/bin**.
3. Edit **$APPDIR/conf/config.yaml**, to specify the starting parameters. Do not change the path and filename. In eg.yaml, the **./** directory is relative to **APPDIR**, instead of the location of eg.yaml.
4. Run **start.sh** to launch the new easegress process. **start.sh** will ignore if there has been an instance running.
5. Run **stop.sh** to terminate the process. stop.sh will warn and quit if there's not enough etcd members alive. run **stop.sh -f** if you really want to stop the node.
6. Run **status.sh** to check the overview status of the **easegress** group. Run **status.sh -l** to check the detailed status of the **easegress** group.
7. Run **egctl.sh** with parameters, this will delegate to **APPDIR/bin/egctl** and send request to the API server defined by **api-addr** in $APPDIR/conf/eg.yaml

