## How to set sensible parallelism for pipeline

## How to set sensible cross_pipeline_request_backlog for pipeline

## How to see whether the changes for pipeline or plugins has been successfully applied or not?
1. You can check the exit code of the easegateway-client whether it exited successfully.
2. You can use `pipeline ls` or `plugin ls` to check the latest configuration manually. 

NOTE: when you are operating cluster, you need to specify option `--consistent`. That option indicates request is performed in the group as consistency or availability first.

## Why {QUERY_STRING} is empty in HTTPOutput plugin?

Mostly, this will happen when you are using HTTPOutput plugin in a upstream pipeline (cross-pipeline request), please check the request_data_keys option of your upstream output plugin. We need to specify the key names that you want to passed to target upstream pipeline. You need to take care response_data_keys option of downstream input plugin as well if you whant to put the data back to the downstream pipeline.

## Difference between admin/adminc
adminc is cluster administration interface
admin adminstration interface for standalone EaseGateway

## How to debug in cluster mode?
1. Use `-stage test`, this will let the EaseGateway node print logs in DEBUG level
2. Check the max sequence

## How to use cpu profile
1. Use option `cpuprofile` to specify cpu profile output file and start the EaseGateway node
2. Stop the EaseGateway node after profiling, then pprof will stop the cpu profile and dump the profile buffer to your cpu profile output file

## How to use memory profile
1. Use option `memprofile` to specify cpu profile output file and start the EaseGateway node
2. Send `SIGQUIT` signal to EaseGateway by using `kill -SIGQUIT eg-pid`, then EaseGateway will dump the memprofile file
