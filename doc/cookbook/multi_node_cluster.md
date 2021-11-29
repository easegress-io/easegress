
# Multi-node cluster

- [Multi-node cluster](#multi-node-cluster)
  - [Multiple members](#multiple-members)
    - [Add new secondary node](#add-new-secondary-node)
    - [YAML Configuration](#yaml-configuration)
  - [Cluster role](#cluster-role)
    - [Cluster roles in etcd terminology](#cluster-roles-in-etcd-terminology)
  - [References](#references)

It is easy to start multiple Easegress instances to form an Easegress cluster, using `easegress-server` binary.

## Multiple members
Let's suppose you have three nodes that are in the same network or otherwise accessible. We will start three instances of `easegress-server` with `primary` cluster role. Let's store node's private IPs to following environment variables:

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
  --cluster-role "primary" \
  --name "machine-1" \
  --api-addr $HOST1:2381 \
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
  --cluster-role "primary" \
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
  --cluster-role "primary" \
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
```
should print 
```bash
    name: machine-1
    name: machine-2
    name: machine-3
```

###  Add new secondary node

Let's add one more node with a *secondary* cluster role this time. Please note that it is not recommended to add additional node with `primary` cluster role, but `primary` nodes should be started at cluster start up.

```bash
# on a new machine
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "secondary" \
  --name "machine-4" \
  --primary-listen-peer-urls http://$HOST1:2380 \
  --state-flag "existing"
```

## YAML Configuration

The examples above use the *easegress-server's* command line flags, but often it is more convenient to define server parameters in a yaml configuration file. For example, store following yaml to each host machine and change the host addresses accordingly.

```yaml
# create one yaml file for each host
name: machine-1 # machine-2, machine-3
cluster-name: cluster-test
cluster-role: primary
api-addr: localhost:2381
data-dir: ./data
wal-dir: ""
cpu-profile-file:
memory-profile-file:
log-dir: ./log
debug: false
cluster:
  listen-peer-urls: # change CURRENT-HOST to current host
   - http://<CURRENT-HOST>:2380
  listen-client-urls:
   - http://<CURRENT-HOST>:2379
  advertise-client-urls:
   - http://<CURRENT-HOST>:2379
  initial-advertise-peer-urls:
   - http://<CURRENT-HOST>:2380
  initial-cluster: # initial-cluster is same for every host
   - machine-1: http://<HOST-1>:2380
   - machine-2: http://<HOST-2>:2380
   - machine-3: http://<HOST-3>:2380
```
Then apply these values on each machine, using `config-file` command line argument:
`easegress-server --config-file config.yaml`.

The configuration file for adding new secondary node looks like following:

```yaml
name: machine-4
cluster-name: cluster-test
cluster-role: secondary
data-dir: ./data
wal-dir: ""
cpu-profile-file:
memory-profile-file:
log-dir: ./log
debug: false
cluster:
  primary-listen-peer-urls: http://$HOST1:2380
```

##  Cluster role

When running Easegress as a cluster, each instance has either *primary* or *secondary* role. *Primary* nodes persist the Easegress state on the disk, while *secondary* request this information from their peers (defined by `initial-advertise-peer-urls` parameter).

It is a good practice to choose an odd number (1,3,5,7,9) of *primary* nodes, to tolerate failures of *primary* nodes. This way the cluster can stay in healthy state, even if the network partitions. With an even number of *primary* nodes, the cluster can be divided to two groups of equal size due to network partition. Then neither of the sub-cluster have the majority required for consensus (of Raft algorithm). However with odd number of *primary* nodes, the cluster cannot be divided to two groups of equal size and this problem cannot occur.

For the *secondary* nodes, there is no constraints for the number of nodes. Secondary nodes do not participate consensus vote of the cluster, so their failure do not affect the cluster health. Adding more (*secondary*) nodes does still increase the communication between nodes.

### Cluster roles in etcd terminology

Easegress cluster uses [etcd](https://etcd.io) distributed key-value store to synchronize the cluster state. The primary and secondary cluster roles have following relation with `etcd`:


| Easegress cluster role   | primary   | secondary   |
|-----|-----|-----|
| etcd term | server | client |


## References

1. https://en.wikipedia.org/wiki/High-availability_cluster
2. https://en.wikipedia.org/wiki/Raft_(algorithm)
3. https://etcd.io/docs/v3.5/faq/
