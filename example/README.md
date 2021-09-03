# Easegress

Port Layout:

|   Member   | Cluster Client Port | Cluster Peer Port | API Port |
| :--------: | :-----------------: | :---------------: | :------: |
| writer-001 |        12379        |       12380       |  12381   |
| writer-002 |        22379        |       22380       |  22381   |
| writer-003 |        32379        |       22380       |  32381   |
| reader-004 |          -          |         -         |  42381   |
| reader-005 |          -          |         -         |  52381   |

## Start Easegress Cluster

```shell
./start_cluster.sh
```

Please notice only one server will listen the port 10080 successfully
because the members are running on the same machine.
But it's fine for the following demoing.

## Operation On Easegress Cluster

```shell
 ./create_objects.sh
```

The `create_objects.sh` applies the operation creation for all the yaml files under `config/`.
It's fine that it applies objects which has already existed.

```shell
 ./update_objects.sh
```

The `update_objects.sh` applies the operation update for all the yaml files under `config/`.
It's fine that it applies objects which don't exist.

The `check_cluster_status.sh` will list:

- all status of members.
- all objects.

1. Using `curl` to test the access-ability of applied services.
2. Using HTTP-based tool [hey](https://github.com/rakyll/hey) for stress-testing the applied services.


## Stop Easegress Cluster

```shell
 ./stop_cluster.sh
```

## Stop and Clean Easegress Cluster

```shell
 ./clean_cluster.sh
```

It will ignore step-stop if members have stopped.

## Backend Service

```shell
go run mirror.go
go run remote.go
```

Please notice we didn't start backend service in the scripts above,
so we testers could observe the situation when the backend is not ready.
