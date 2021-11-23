
# Multi-node cluster

- [Multi-node cluster](#multi-node-cluster)
  - [Multiple writers](#multiple-writers)
    - [Add new node](#add-new-node)
    - [YAML Configuration](#yaml-configuration)
  - [Readers and writers](#readers-and-writers)
    - [Cluster roles in etcd terminology](#cluster-roles-in-etcd-terminology)
  - [References](#references)

It is easy to start multiple Easegress instances to form an Easegress cluster, using `easegress-server` binary.

## Multiple writers
Let's suppose you have three nodes that are in the same network or otherwise accessible. Let's store node's private IPs to following environment variables:

```bash
export HOST1=<host1-IP>
export HOST2=<host2-IP>
export HOST3=<host3-IP>
export CLUSTER=machine-1=http://$HOST1:2380,machine-2=http://$HOST2:2380,machine-3=http://$HOST3:2380
```

Set the environment variables to each machine. Start the first instance at the first machine
```bash
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "writer" \
  --name "machine-1" \
  --api-addr $HOST1:2381 \ # or 0.0.0.0:2381 if running in docker
  --initial-advertise-peer-urls http://$HOST1:2380 \
  --listen-peer-urls http://$HOST1:2380 \
  --listen-client-urls http://$HOST1:2379 \
  --advertise-client-urls http://$HOST1:2379 \
  --initial-cluster $CLUSTER
```
then the second instance at machine 2
```bash
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "writer" \
  --name "machine-2" \
  --initial-advertise-peer-urls http://$HOST2:2380 \
  --listen-peer-urls http://$HOST2:2380 \
  --listen-client-urls http://$HOST2:2379 \
  --advertise-client-urls http://$HOST2:2379 \
  --initial-cluster $CLUSTER
```
and the last machine 3.
```bash
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "writer" \
  --name "machine-3" \
  --initial-advertise-peer-urls http://$HOST3:2380 \
  --listen-peer-urls http://$HOST3:2380 \
  --listen-client-urls http://$HOST3:2379 \
  --advertise-client-urls http://$HOST3:2379 \
  --initial-cluster $CLUSTER
```

Now list cluster members
```bash
egctl --server $HOST1:2381 member list | grep " name"
# or if running docker
# egctl --server 0.0.0.0:2381 member list | grep " name"
```
should print 
```bash
    name: machine-1
    name: machine-2
    name: machine-3
```

###  Add new reader node

Let's add one more node with a *reader* cluster role this time. Please note that it is not recommended to add additional writers, that were not created at cluster start up.

```bash
# on a new machine
export HOST1=<host1-IP>
export HOST2=<host2-IP>
export HOST3=<host3-IP>
export HOST4=<host4-IP>
export CLUSTER=machine-1=http://$HOST1:2380,machine-2=http://$HOST2:2380,machine-3=http://$HOST3:2380,machine-4=http://$HOST4:2380


easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "reader" \
  --name "machine-4" \
  --initial-advertise-peer-urls http://$HOST4:2380 \
  --listen-peer-urls http://$HOST4:2380 \
  --listen-client-urls http://$HOST4:2379 \
  --advertise-client-urls http://$HOST4:2379 \
  --initial-cluster $CLUSTER
  --state-flag "existing"
```

## YAML Configuration

The examples above use the easegress-server's command line flags, but often it is more convenient to define server parameters in a yaml configuration file. For example, store following yaml to each host machine and change the host addresses accordingly.

```yaml
# create one yaml file for each host
name: writer-1 # writer-2, writer-3
cluster-name: cluster-test
cluster-role: writer
api-addr: localhost:2381
data-dir: ./data
wal-dir: ""
cpu-profile-file:
memory-profile-file:
log-dir: ./log
debug: false
cluster:
  listen-peer-urls:
   - http://<HOST-1>:2380
  listen-client-urls:
   - http://<HOST-1>:2379
  advertise-client-urls:
   - http://<HOST-1>:2379
  initial-advertise-peer-urls:
   - http://<HOST-1>:2380
  initial-cluster:
   - writer-1: http://<HOST-1>:2380
   - writer-2: http://<HOST-1>:2380
   - writer-3: http://<HOST-1>:2380
```
Then apply these values on each machine, using `config-file` command line argument:
`easegress-server --config-file config.yaml`.

##  Reader and writer nodes

When running Easegress as a cluster, each instance has either *writer* or *reader* role. *Writer* nodes persist the Easegress state on the disk, while *readers* request this information from their peers (defined by `initial-advertise-peer-urls` parameter).

It is a good practice to choose an odd number (1,3,5,7,9) of *writers*, to tolerate failures of *writer* nodes. This way the cluster can stay in healthy state, even if the network partitions. With an even number of writer nodes, the cluster can be divided to two groups of equal size due to network partition. Then neither of the sub-cluster have the majority required for consensus. However with odd number of *writer* nodes, the cluster cannot be divided to two groups of equal size and this problem cannot occur.

For the *readers*, there is no constraints for the number of nodes. Readers do not participate consensus vote of the cluster, so their failure do not affect the cluster health. Adding more (*reader*) nodes does still increase the communication between nodes.

### Cluster roles in etcd terminology

Easegress cluster uses [etcd](https://etcd.io) distributed key-value store to synchronize the cluster state. The writer and reader cluster roles have following relation with `etcd`:


| Easegress cluster role   | writer   | reader   |
|-----|-----|-----|
| etcd term | server | client |


## References

1. https://en.wikipedia.org/wiki/High-availability_cluster
2. https://en.wikipedia.org/wiki/Raft_(algorithm)
3. https://etcd.io/docs/v3.5/faq/
