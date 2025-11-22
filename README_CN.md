# tiny-redis

[English](./README.md)

`tiny-redis` 是一个使用 Go 开发的 Redis 协议兼容缓存服务器。它在内存中存储数据，使用 BoltDB 持久化 Raft 元数据，并通过 Raft 完成多节点复制和故障切换。

---

## 核心特性

- **RESP 协议兼容**：支持 `redis-cli`、Medis、AnotherRedisDesktopManager 等客户端。
- **丰富的数据结构**：字符串、哈希、列表、集合、有序集合、TTL 等常用命令。
- **Raft 集群**：自动选主，日志复制，持久化 `raft-log.bolt`、`raft-stable.bolt` 与快照。
- **日志系统**：基于 Go 1.21 `log/slog`，支持 JSON 文件输出与日志采样。
- **灵活配置**：命令行参数或配置文件，支持热插拔节点。

---

## 环境要求

- Go 1.25+
- macOS / Linux（Windows 可通过 WSL2）
- Docker（可选）

---

## 单节点启动

```bash
# 克隆仓库并编译（禁用 greentea GC）
git clone https://github.com/HSn0918/tinyredis
cd tinyredis
GOEXPERIMENT=greenteagc make build

# 启动单个 Raft 节点
./bin/tinyredis node \
  --host 127.0.0.1 --port 6379 \
  --node-id node-1 \
  --raft-dir ./data/node1 \
  --raft-bind 127.0.0.1:7000 \
  --raft-http 127.0.0.1:17000 \
  --raft-bootstrap
```

然后可通过 `redis-cli` 连接验证：

```bash
redis-cli -p 6379 ping
```

> 数据存储在内存中；Raft 状态保存在 `./data/node1/`。

---

## 独立模式（不使用 Raft）

如果不需要集群或复制能力，可以使用独立模式启动：

```bash
./bin/tinyredis node \
  --host 127.0.0.1 --port 6379 \
  --standalone
```

独立模式下，写入直接作用于本地内存数据库，不进行选主与日志复制。

---

## 多节点集群

### 1. 引导第一个节点

```bash
./bin/tinyredis node \
  --host 127.0.0.1 --port 6379 \
  --node-id node-1 \
  --raft-dir ./data/node1 \
  --raft-bind 127.0.0.1:7000 \
  --raft-http 127.0.0.1:17000 \
  --raft-bootstrap
```
等待日志显示成为 leader。

### 2. 新节点加入集群

每个新节点需要空的 `raft-dir`，并在启动时指定 `--raft-join` 指向 leader 的 HTTP 地址：

```bash
./bin/tinyredis node \
  --host 127.0.0.1 --port 6380 \
  --node-id node-2 \
  --raft-dir ./data/node2 \
  --raft-bind 127.0.0.1:7001 \
  --raft-http 127.0.0.1:17001 \
  --raft-join 127.0.0.1:17000
```

按同样方式启动 node-3 等更多节点。

### 3. 重启已有节点

- 保留原来的 `--node-id`、`--raft-dir`、`--raft-bind`、`--raft-http`。
- **不要**再使用 `--raft-join` 或 `--raft-bootstrap`。
- 先启动之前的 leader，再启动其他节点。

### 4. 新增全新的节点

- 为新节点准备独立目录。
- 启动时使用 `--raft-join` 指向当前 leader。

### 故障转移观察流程

1. 按上文的 `bootstrap` / `join` 命令启动三节点后，分别查看：
   ```bash
   redis-cli -p 6379 INFO replication
   redis-cli -p 6380 INFO replication
   redis-cli -p 6381 INFO replication
   ```
   只有一个节点会是 `role:master`，其他节点显示 `role:slave`，并指向同一个 `leader_id`。
2. 在 leader 上写入数据：
   ```bash
   redis-cli -p 6379 SET failover demo
   ```
3. 使用 Ctrl+C 停掉 leader 进程，稍等几秒后，再次查看剩余节点的 `INFO replication`，会看到新的 leader 升任（`role:master`、`leader_id` 更新）。
4. 在新 leader 上读取验证：
   ```bash
   redis-cli -p 6380 GET failover
   ```
5. 需要让旧节点重新上线时，保持原来的数据目录与 ID，不要再加 `--raft-join/--raft-bootstrap`：
   ```bash
   ./bin/tinyredis node --host 127.0.0.1 --port 6379 \
     --node-id node-1 \
     --raft-dir ./data/node1 \
     --raft-bind 127.0.0.1:7000 \
     --raft-http 127.0.0.1:17000
   ```
   也可以使用新的快捷命令：
   ```bash
   make rejoin REJOIN_NODE=node-1 REJOIN_PORT=6379 \
     REJOIN_RAFT=127.0.0.1:7000 REJOIN_HTTP=127.0.0.1:17000 \
     REJOIN_HOST=127.0.0.1
   ```

> `INFO replication` 输出提供 `role`、`leader_id`、`leader_raft_addr`、`known_peers` 等字段，便于脚本化探测当前 leader。

> 小贴士：`make cluster-up` 可以一键拉起三节点开发集群（数据/日志位于 `.devcluster`），`make cluster-down` 可快速清理。

### 统一代理入口

如果希望客户端（例如 `redis-cli`）始终连接到一个固定地址，可使用内置代理：

```bash
./bin/tinyredis proxy \
  --listen 127.0.0.1:7390 \
  --nodes 127.0.0.1:6379,127.0.0.1:6380,127.0.0.1:6381
```

代理会定期使用 `INFO replication` 探测 leader，并自动把进入代理的连接转发到当前 leader 的 RESP 端口；当 leader 发生切换时，新的连接将自动落到新的 leader。

---

## 常见命令行参数

| 参数 | 描述 |
|------|------|
| `--host`, `--port` | RESP 服务监听地址与端口。 |
| `--node-id` | 唯一的 Raft 节点 ID。 |
| `--raft-dir` | Raft 数据目录（log/stable/snapshot）。 |
| `--raft-bind` | Raft 复制通信地址。 |
| `--raft-http` | 接受 `join` 请求的 HTTP 地址。 |
| `--raft-join` | 指定 leader 的 `host:port` 加入集群。 |
| `--raft-bootstrap` | 初始化新集群（仅首次使用）。 |
| `--standalone` | 启动为独立模式，不启用 Raft。 |
| `--logdir` | JSON 日志输出目录。 |
| `--loglevel` | `debug`/`info`/`warn`/`error`。 |
| `--log-sampling` | 是否开启日志采样（默认开启）。 |

使用 `./bin/tinyredis node --help` 查看全部选项。

---

## Docker 快速体验

```bash
make docker-build
docker run -d --rm \
  --name tinyredis \
  -p 6379:6379 \
  -v $PWD/data/node1:/data \
  tinyredis:latest \
  ./tinyredis node --host 0.0.0.0 --node-id node-1 \
  --raft-dir /data --raft-bind 0.0.0.0:7000 --raft-http 0.0.0.0:17000 --raft-bootstrap
```

多节点部署可使用多个容器或 docker-compose，每个节点需要不同的端口与数据卷。

---

## 日志说明

- 默认以文本格式输出到 stdout。
- 指定 `--logdir` 后，会在目录下生成 JSON 格式的 `redis.log`。
- `--log-sampling-*` 控制重复日志的采样，可关闭以获得完整日志。

---

## 测试与质量

```bash
make test
make lint
make fmt
```

第一次运行 `make lint` 时会自动安装 `golangci-lint`。

想要评估性能表现，可参考 [性能测试全流程教程](./docs/performance_tutorial_CN.md)。

---

## 常用命令支持

- **String**：`GET`、`SET`、`MSET`、`MGET`、`INCR`、`DECR`、`SETEX`、`SETNX`、`APPEND` 等。
- **Hash**：`HSET`、`HGET`、`HDEL`、`HMGET`、`HINCRBY`、`HRANDFIELD`、`HSTRLEN` 等。
- **List**：`LPUSH`、`RPUSH`、`LPOP`、`RPOP`、`LRANGE`、`LSET`、`LTRIM` 等。
- **Set**：`SADD`、`SREM`、`SMEMBERS`、`SINTER`、`SUNION`、`SRANDMEMBER` 等。
- **Sorted Set**：`ZADD`、`ZREM`、`ZINCRBY`、`ZPOPMAX`、`ZCOUNT` 等。
- **Key/Admin**：`DEL`、`EXPIRE`、`TTL`、`TYPE`、`PING`、`INFO` 等。

详细命令实现位于 `pkg/memdb` 目录。

---

## 目录结构

```
.
├── cmd/                 # CLI 入口
├── pkg/
│   ├── RESP/            # RESP 编码解码
│   ├── cluster/         # Raft 节点封装
│   ├── logger/          # slog 日志封装
│   ├── memdb/           # 内存数据库及命令实现
│   └── server/          # TCP 服务监听/连接处理
├── data/                # 运行时生成的 Raft 数据目录
├── Makefile             # 常用构建命令
├── Justfile             # 常用脚本别名
├── go.mod / go.sum
└── README.md / README_CN.md
```

---

## 未来规划

- 覆盖更多 Redis 命令（事务、Pub/Sub）。
- follower 只读请求的转发。
- 监控指标与管理接口。
- 支持 RDB/AOF 导出。

欢迎提交 Issue 与 PR！
Docker 构建（Dockerfile 默认使用 `GOEXPERIMENT=greenteagc`）

```bash
docker build -t tinyredis:latest .
# 或在构建时显式指定：
docker build --build-arg GOEXPERIMENT=greenteagc -t tinyredis:latest .
```
