# EaseGateway

Port Layout:

|   Member   | Cluster Client Port | Cluster Peer Port | API Port |
|:----------:|:-------------------:|:-----------------:|:--------:|
| writer-001 |        12379        |       12380       |   12381  |
| writer-002 |        22379        |       22380       |   22381  |
| writer-003 |        32379        |       22380       |   32381  |
| reader-004 |          -          |         -         |   42381  |
| reader-005 |          -          |         -         |   52381  |

## Start EaseGateway Server

```shell
./start.sh
```

Please notice only one server will listen the port 10080 successfully
becasue the members are running on the same machine.
But it's fine for the following test.

## Operation On EaseGateway

```shell
 ./create.sh
```

The `create.sh` applies the operation creation for all the yaml files under `config/`.
It's fine that it applies objects which has already existed.

```shell
 ./update.sh
```

The `update.sh` applies the operation update for all the yaml files under `config/`.
It's fine that it applies objects which don't exist.

Other meta operation scripts:

- `list_member.sh` lists all status of members.
- `list_object.sh` lists all objects.
- `list_status.sh` lists all status of objects.

NOTICE: Install benchmark tool hey firstly: https://github.com/rakyll/hey .
Action monitor and Benchmarsh scripts:

- `curl_pipeline` curls backend pipeline.
- `curl_proxy`    curls backend proxy.
- `curl_remote`   curls backend remote.
- `hey_pipeline`  benchmarks backend pipeline.
- `hey_proxy`     benchmarks backend proxy.
- `hey_remote`    benchmarks backend remote.

## Stop EaseGateway Server

```shell
 ./stop.sh
```

## Stop and Clean EaseGateway Server

```shell
 ./clean.sh
```

It will ignore step-stop if members have stopped.

## Backend Service

```shell
go run mirror.go
go run remote.go
```

Please notice we didn't start backend service in the scripts above,
so we testers could observe the situation when the backend is not ready.
