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

- Go 1.23+
- macOS / Linux（Windows 可通过 WSL2）
- Docker（可选）

---

## 单节点启动

```bash
# 克隆仓库并编译
git clone https://github.com/HSn0918/tinyredis
cd tinyredis
make build

# 启动单节点
./bin/tinyredis \
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

## 多节点集群

### 1. 引导第一个节点

```bash
./bin/tinyredis \
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
./bin/tinyredis \
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

### 自动故障转移代理（实验特性）

如果希望客户端始终连接当前的 leader，可运行内置的轻量级代理：

```bash
./bin/tinyredis failover-proxy \
  --listen 127.0.0.1:7390 \
  --nodes 127.0.0.1:6379,127.0.0.1:6380,127.0.0.1:6381
```

代理会轮询指定节点，使用 `INFO replication` 判断谁是 leader，并把进入代理的连接转发到该节点。leader 发生切换时，代理会在下次连接建立时自动跳转。  
（如果客户端本身支持自动重连，例如 `redis-cli` 默认行为，断开后会自动连回代理，从而连到新的 leader。）

> 新版 `INFO replication` 会输出 `role`、`leader_id`、`leader_raft_addr`、`known_peers` 等字段，可用于快速定位 leader 与集群状态。

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
| `--logdir` | JSON 日志输出目录。 |
| `--loglevel` | `debug`/`info`/`warn`/`error`。 |
| `--log-sampling` | 是否开启日志采样（默认开启）。 |

使用 `./bin/tinyredis --help` 查看全部选项。

---

## Docker 快速体验

```bash
make docker-build
docker run -d --rm \
  --name tinyredis \
  -p 6379:6379 \
  -v $PWD/data/node1:/data \
  tinyredis:latest \
  ./tinyredis --host 0.0.0.0 --node-id node-1 \
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
