
# Multi-node cluster

- [Multi-node cluster](#multi-node-cluster)
  - [Multiple instances in single node](#multiple-instances-in-single-node)
  - [Multiple nodes](#multiple-nodes)
  - [Readers and writers](#readers-and-writers)
    - [Cluster roles in etcd terminology](#cluster-roles-in-etcd-terminology)
  - [References](#references)

It is easy to start multiple Easegress instances to form an Easegress cluster, using `easegress-server` binary.

##  Multiple instances in single node

To start a single writer instance and two readers, we need to start the server in three separate terminals. It's enough to define `localhost` for `cluster-join-urls` value and leave most of the options to default values.

Start writer instance
```bash
easegress-server \
  --cluster-name "multi-instance-cluster" \
  --cluster-role "writer" \
  --name "writer" \
  --cluster-join-urls http://localhost:2380
```
Then open a new terminal window and add first reader instance
```bash
easegress-server \
  --cluster-name "multi-instance-cluster" \
  --cluster-role "reader" \
  --name "reader-node1" \
  --cluster-join-urls http://localhost:2380
```
and third terminal for the second reader:
```bash
easegress-server \
  --cluster-name "multi-instance-cluster" \
  --cluster-role "reader" \
  --name "reader-node2" \
  --cluster-join-urls http://localhost:2380
```

We can now see three members in the cluster `egctl member list`
or to list only member names `egctl member list | grep " name"`.

## Multiple nodes
Often the cluster instances run at different machines. Let's suppose you have three nodes that are in the same network or otherwise accessible. Let's store node's private IPs to following environment variables:

```bash
export HOST1=<host1-IP>
export HOST2=<host2-IP>
export HOST3=<host3-IP>
```

Add environment variables to each machine:

Start the first instance at the first machine
```bash
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "writer" \
  --name "machine-1" \
  --api-addr $HOST1:2381 \ # or 0.0.0.0:2381 if running in docker
  --cluster-initial-advertise-peer-urls http://$HOST1:2380 \
  --cluster-listen-peer-urls http://$HOST1:2380 \
  --cluster-listen-client-urls http://$HOST1:2379 \
  --cluster-advertise-client-urls http://$HOST1:2379 \
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST3:2376
```
then the second instance at machine 2
```bash
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "writer" \
  --name "machine-2" \
  --cluster-initial-advertise-peer-urls http://$HOST2:2378 \
  --cluster-listen-peer-urls http://$HOST2:2378 \
  --cluster-listen-client-urls http://$HOST2:2377 \
  --cluster-advertise-client-urls http://$HOST2:2377 \
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST3:2376
```
and the last machine 3
```bash
easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "writer" \
  --name "machine-3" \
  --cluster-initial-advertise-peer-urls http://$HOST3:2376 \
  --cluster-listen-peer-urls http://$HOST3:2376 \
  --cluster-listen-client-urls http://$HOST3:2375 \
  --cluster-advertise-client-urls http://$HOST3:2375 \
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST3:2376
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

###  Add new node

Let's add one more node; this time a reader cluster role.

```bash
# on a new machine
export HOST1=<host1-IP>
export HOST2=<host2-IP>
export HOST3=<host3-IP>
export HOST4=<host4-IP>

easegress-server \
  --cluster-name "multi-node-cluster" \
  --cluster-role "reader" \
  --name "machine-4" \
  --cluster-initial-advertise-peer-urls http://$HOST4:2374 \
  --cluster-listen-peer-urls http://$HOST4:2374 \
  --cluster-listen-client-urls http://$HOST4:2373 \
  --cluster-advertise-client-urls http://$HOST4:2373 \
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST3:2376,http://$HOST4:2374
```


##  Reader and writer nodes

When running Easegress as a cluster, each instance has either *writer* or *reader* role. *Writer* nodes persist the Easegress state on the disk, while *readers* request this information from their peers (defined by `cluster-initial-advertise-peer-urls` parameter).

It is a best practice to choose an odd number (1,3,5,7,9) of *writers*, to tolerate failures of *writer* nodes. This way the cluster can stay in healthy state, even if the network partitions. With an even number of writer nodes, the cluster can be divided to two groups of equal size due to network partition. Then neither of the sub-cluster have the majority required for consensus. However with odd number of *writer* nodes, the cluster cannot be divided to two groups of equal size and this problem cannot occur.

For the *readers*, there is no constraints for the number of nodes. Readers do not participate consensus vote of the cluster, so their failure do not affect the cluster health. It is a good practice to have few *reader* nodes.

### Cluster roles in etcd terminology

Easegress cluster uses [etcd](https://etcd.io) distributed key-value store to synchronize the cluster state. The writer and reader cluster roles have following relation with `etcd`:


| Easegress cluster role   | writer   | reader   |
|-----|-----|-----|
| etcd term | server | client |


## References

1. https://en.wikipedia.org/wiki/High-availability_cluster
2. https://en.wikipedia.org/wiki/Raft_(algorithm)
3. https://etcd.io/docs/v3.5/faq/
