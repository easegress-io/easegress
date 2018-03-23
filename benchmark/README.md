## This directory contains Benchmark facilities and benchmark results

### Scripts

#### Benchamrk easegateway

log rt distribution into csv file and log result into file
for example:

'scripts/benchmark_eg.sh --title nethttp --family keepalive -c 3000'

#### Monitor cpu and memory usage
monitor cpu and memory usage and log into csv file

'scripts/log_eg_cpu_memory.sh'

#### Process benchmark result
##### Abstract mean qps and rt from multiple files into csv format in one file
'scripts/extract_ab_qps_rt.sh ./keepalive/fasthttp/*.ab'

##### Merge multiple ab csv result into one file
'scripts/merge_multiple_ab_csv.sh ./csv/*.csv
