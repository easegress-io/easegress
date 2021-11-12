
# Multi-node cluster

- [Multi-node cluster](#multi-node-cluster)
  - [Multiple instances in single node](#multiple-instances-in-single-node)
  - [Multiple nodes](#multiple-nodes)

It is easy to start multiple Easegress instances to form an Easegress cluster, using `easegress-server` binary.

##  Multiple instances in single node

To start a single writer instance and two readers, we need to run start the server in three separate terminals. It's enough to define `localhost` for `cluster-join-urls` value and leave most of the options to default values.

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
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST2:2376
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
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST2:2376
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
  --cluster-join-urls http://$HOST1:2380,http://$HOST2:2378,http://$HOST2:2376
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
