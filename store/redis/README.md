# redis（`session.Store` 的 Redis 实现）

本目录为 **独立 Go module**：`github.com/boxgo/session/store/redis`，实现 `github.com/boxgo/session` 中的 `Store` 接口，依赖 [go-redis v9](https://github.com/redis/go-redis) 与主模块；按需 `import` 时才会把 Redis 相关依赖链进业务二进制。

完整会话语义（过期 / 删除 / `Refresh` / `Purge` 等）见仓库根目录 [README.md](../../README.md)。

## 安装

```bash
go get github.com/boxgo/session@latest
go get github.com/boxgo/session/store/redis@latest
```

要求 Go **1.19+**（以本目录 `go.mod` 为准）。

`require github.com/boxgo/session` 使用 **semver 或伪版本**（勿提交 `replace => ../..`）。固定版本时子模块 Git tag 为 **`store/redis/vX.Y.Z`**（与根目录 `vX.Y.Z` 不同）。**打 tag / Release** 见根目录 [README.md](../../README.md) 中「参与开发」。**升级主模块依赖版本** 见根 README 同章「升级 redis 子模块对主模块 session 的依赖」。

## 快速使用

```go
import (
    "github.com/boxgo/session"
    sessredis "github.com/boxgo/session/store/redis"
    "github.com/redis/go-redis/v9"
)

cli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store := sessredis.NewRedisStore(cli, "myapp:session")
mgr := session.NewManager(store, session.WithMode(session.ModeMulti))
```

- `prefix`：键命名空间，**建议传入业务前缀**；若传空字符串，默认使用 `"session"`。
- 客户端类型为 `redis.UniversalClient`，单实例、Sentinel、Cluster 等均可。

## 编解码（JSON / msgpack）

默认使用 **JSON**。可切换为 msgpack 或自定义 `Codec`：

```go
store := sessredis.NewRedisStoreWithCodec(
    cli,
    "myapp:session",
    sessredis.MsgpackCodec(),
)
```

实现 `sessredis.Codec` 即可接入自定义序列化格式。

## Redis 键与索引（约定）

以下键均带 `prefix` 前缀：

| 模式 | 说明 |
|------|------|
| `{prefix}:session:{sessionID}` | 会话主数据（`STRING`，值为序列化后的 `Session`） |
| `{prefix}:user:{userID}` | 用户会话索引（`ZSET`，member 为 `sessionID`，score 为更新时间 Unix 秒） |
| `{prefix}:deleted` | 待物理清理索引（`ZSET`，member 为 `userID` + 分隔符 + `sessionID`，score 为 `DeletedAt` Unix 秒） |

有 `DeletedAt` 时会话主键可配合 Redis 过期时间做自动过期；`Purge` 会按删除索引与用户索引做清理。批量读写使用 pipeline 降低 RTT。

## 本目录开发与测试

在 monorepo 根目录（含 `go.work`）下：

```bash
go test ./store/redis/...
```

仅在本子模块目录时：

```bash
cd store/redis
go test ./...
```
