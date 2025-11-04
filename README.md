# tiny-redis

[中文](./README_CN.md)

`tiny-redis` is a Redis-compatible cache server written in Go. It implements the RESP protocol, stores data in memory, persists Raft state with BoltDB, and supports log-based replication between multiple nodes.

---

## Features

- RESP protocol compatibility – works with `redis-cli`, Medis, AnotherRedisDesktopManager, etc.
- In-memory data structures covering strings, hashes, lists, sets, sorted sets, TTL, and more.
- Raft-based clustering with automatic leader election and log replication.
- Durable metadata: Raft log, stable store, and snapshots persisted to disk.
- Pluggable logging using Go 1.21+ `log/slog` with optional sampling and JSON file output.
- CLI configuration via flags or config file.

---

## Requirements

- Go 1.23+
- macOS/Linux (tested) – Windows via WSL2
- Docker 24+ (optional)

---

## Build & Run (single node)

```bash
git clone https://github.com/HSn0918/tinyredis
cd tinyredis
go build ./cmd/tinyredis

# start a standalone node on 127.0.0.1:6379
./tinyredis node \
  --host 127.0.0.1 \
  --port 6379 \
  --node-id node-1 \
  --raft-dir ./data/node1 \
  --raft-bind 127.0.0.1:7000 \
  --raft-http 127.0.0.1:17000 \
  --raft-bootstrap
```

Then connect with `redis-cli`:

```bash
redis-cli -p 6379 ping
```

Data lives in memory, while Raft metadata is written to `./data/node1/`.

---

## Multi-node cluster

1. **Bootstrap the first node**
   ```bash
   ./tinyredis node --node-id node-1 \
     --host 127.0.0.1 --port 6379 \
     --raft-dir ./data/node1 \
     --raft-bind 127.0.0.1:7000 \
     --raft-http 127.0.0.1:17000 \
     --raft-bootstrap
   ```
   Wait until logs show it as leader.

2. **Join additional nodes**
   Each new node needs an empty raft directory and `--raft-join` pointing to the leader’s HTTP address:
   ```bash
   ./tinyredis node --node-id node-2 \
     --host 127.0.0.1 --port 6380 \
     --raft-dir ./data/node2 \
     --raft-bind 127.0.0.1:7001 \
     --raft-http 127.0.0.1:17001 \
     --raft-join 127.0.0.1:17000
   ```
   Repeat for node-3, etc.

3. **Restarting existing nodes**
   - Keep their `--node-id`, `--raft-dir`, `--raft-bind`, `--raft-http`.
   - Do **not** pass `--raft-join` or `--raft-bootstrap` again.
   - Start the former leader first, then the followers.

4. **Adding a brand new node**
   - Create a fresh directory for its raft state.
   - Start with `--raft-join` pointed at the current leader.

> Quick start: `make cluster-up` launches a preconfigured three-node cluster (logs/PIDs under `.devcluster`); `make cluster-down` stops it.

---

## Command-line flags (common)

| Flag | Description |
|------|-------------|
| `--host`, `--port` | RESP listening address. |
| `--node-id` | Unique Raft server ID (string). |
| `--raft-dir` | Directory for Raft log/stable/snapshots. |
| `--raft-bind` | TCP address for Raft transport (peer replication). |
| `--raft-http` | HTTP address for join requests. |
| `--raft-join` | `leader-host:leader-http-port` to join an existing cluster. |
| `--raft-bootstrap` | Bootstrap a new cluster when no state exists. |
| `--logdir` | Directory for `redis.log` (JSON). |
| `--loglevel` | `debug`, `info`, `warn`, `error`. |
| `--log-sampling` | Enable log sampling (default true). |
| `--log-sampling-interval` | Sampling window (default 1s). |

Run `./tinyredis node --help` for the full list.

---

## Docker quick start

Build locally:
```bash
docker build -t tinyredis:latest .
```

Run single node:
```bash
docker run -d --name tinyredis \
  -p 6379:6379 \
  -v $PWD/data/node1:/data \
  tinyredis:latest \
  ./tinyredis node --host 0.0.0.0 --node-id node-1 \
  --raft-dir /data --raft-bind 0.0.0.0:7000 \
  --raft-http 0.0.0.0:17000 --raft-bootstrap
```

For multi-node clusters you need multiple containers (or compose) with different node IDs, ports, and persistent volumes.

### Observing Failover Locally

1. Start three nodes (see the `bootstrap`/`join` helpers above) and inspect each node:
   ```bash
   redis-cli -p 6379 INFO replication
   redis-cli -p 6380 INFO replication
   redis-cli -p 6381 INFO replication
   ```
   Exactly one node reports `role:master`; the others report `role:slave` and point to the same `leader_id`.
2. Write a value against the leader:
   ```bash
   redis-cli -p 6379 SET failover demo
   ```
3. Press Ctrl+C in the leader’s terminal to stop it. After a few seconds, rerun `INFO replication` on the remaining nodes to see a new leader elected (`role:master`, updated `leader_id`).
4. Read the value back from the new leader:
   ```bash
   redis-cli -p 6380 GET failover
   ```
5. When bringing the old node back, reuse its original configuration **without** `--raft-join`/`--raft-bootstrap`. For example:
   ```bash
   ./bin/tinyredis node --host 127.0.0.1 --port 6379 \
     --node-id node-1 \
     --raft-dir ./data/node1 \
     --raft-bind 127.0.0.1:7000 \
     --raft-http 127.0.0.1:17000
   ```
   You can also rely on the helper target:
   ```bash
   make rejoin REJOIN_NODE=node-1 REJOIN_PORT=6379 \
     REJOIN_RAFT=127.0.0.1:7000 REJOIN_HTTP=127.0.0.1:17000 \
     REJOIN_HOST=127.0.0.1
   ```

> `INFO replication` now includes `role`, `leader_id`, `leader_raft_addr`, and `known_peers`, making it easy to script health/leader checks.

### Unified Proxy Endpoint

Need a single stable RESP endpoint for tools like `redis-cli`? Run the built-in proxy:

```bash
./bin/tinyredis proxy \
  --listen 127.0.0.1:7390 \
  --nodes 127.0.0.1:6379,127.0.0.1:6380,127.0.0.1:6381
```

The proxy probes each node with `INFO replication`, forwards incoming connections to the current leader, and redirects new connections automatically when leadership changes—no client reconfiguration required.

---

## Logging

By default logs go to stdout in text format. When `--logdir` is set, a JSON `redis.log` is also created. Sampling (enabled by default) throttles repeated messages; adjust via `--log-sampling-*` flags or disable entirely with `--log-sampling=false`.

---

## Testing

```bash
go test ./...
golangci-lint run
```

Some packages require write access to `$GOCACHE`; set `GOCACHE` if your environment is read-only.

---

## Commands overview

`tiny-redis` implements a large subset of Redis commands. Highlights per data type:

- **String:** `GET`, `SET`, `MSET`, `MGET`, `INCR`, `DECR`, `SETEX`, `SETNX`, `APPEND`, etc.
- **Hash:** `HSET`, `HGET`, `HDEL`, `HMGET`, `HINCRBY`, `HRANDFIELD`, `HSTRLEN`.
- **List:** `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LRANGE`, `LSET`, `LTRIM`.
- **Set:** `SADD`, `SREM`, `SMEMBERS`, `SINTER`, `SUNION`, `SDIFF`, `SRANDMEMBER`.
- **Sorted Set:** `ZADD`, `ZREM`, `ZINCRBY`, `ZPOPMAX`, `ZCOUNT`.
- **Key / Admin:** `DEL`, `EXPIRE`, `TTL`, `TYPE`, `PING`, `INFO`.

See the actual command registration under `pkg/memdb` for the authoritative list.

---

## Project structure

```
.
├── cmd/               # CLI entrypoint
├── pkg/
│   ├── RESP/          # RESP parser/encoder
│   ├── cluster/       # Raft node orchestration
│   ├── logger/        # slog-based logging helpers
│   ├── memdb/         # in-memory database & commands
│   └── server/        # TCP server & connection handler
├── data/              # default Raft data directory (created at runtime)
├── Dockerfile
├── go.mod / go.sum
└── README.md
```

---

## Roadmap ideas

- More complete Redis command coverage (transactions, pub/sub).
- Replication read forwarding for followers.
- Metrics endpoints and admin tools.
- Optional RDB/AOF export.

Contributions & issues are welcome!
