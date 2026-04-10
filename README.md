# session

高性能用户会话模块（Go 包名 **`session`**，模块路径 `github.com/boxgo/session`），支持以下能力：

- 单会话 / 多会话模式
- 过期时间（`ExpiresAt`）与删除时间（`DeletedAt`）双生命周期
- 已过期但未删除会话可 `Refresh` 恢复生效
- 会话变动事件监听（创建、刷新、删除、替换、清理）
- 事件监听支持开关（默认关闭）
- 存储后端可替换（`memory`；Redis 见子包 `github.com/boxgo/session/store/redis`，按需 `import`，主包不依赖 go-redis）
- 用户维度会话管理（查询、删除、批量删除）

## 安装

### 环境要求

- Go **1.19** 或更高版本

### 作为依赖引入

仅使用主包与 **Memory** 后端时，在业务模块目录执行：

```bash
go get github.com/boxgo/session@latest
```

需要使用 **Redis** 后端时，`store/redis` 为独立子模块，需额外拉取（会带入 `go-redis` 等依赖）：

```bash
go get github.com/boxgo/session@latest
go get github.com/boxgo/session/store/redis@latest
```

也可在 `go.mod` 中手写 `require` 后执行 `go mod tidy`。代码里未 `import .../store/redis` 时，不会把 Redis 客户端链进最终二进制。

## 核心概念

- `ExpiresAt`：会话过期时间，过期后不再是活跃会话
- `DeletedAt`：会话删除时间，达到该时间后会话视为删除态，不能刷新
- `Purge`：物理清理 `DeletedAt <= now` 的会话，释放存储空间

> 语义说明：过期和删除是独立状态。会话可以“过期但未删除”，此时可调用 `Refresh(ctx, id, ttl, deleteAfter)` 重新生效：`ttl`、`deleteAfter` 与 `Open` 含义相同，会从**当前时间**重算 `ExpiresAt` 与 `DeletedAt`（`deleteAfter <= 0` 时 `DeletedAt` 置空）。若已到删除时间，则 `Refresh` 返回 `ErrSessionDeleted`。

## 快速开始

### 1) Memory 后端（`github.com/boxgo/session/store/memory`）

```go
import (
    "github.com/boxgo/session"
    "github.com/boxgo/session/store/memory"
)

store := memory.NewMemoryStore()
mgr := session.NewManager(store, session.WithMode(session.ModeSingle))

sess, err := mgr.Open(ctx, "user-1", "session-1", 30*time.Minute, 24*time.Hour, map[string]string{
    "device": "ios",
})
```

### 2) Redis 后端（`github.com/boxgo/session/store/redis`）

```go
import (
    "github.com/boxgo/session"
    sessredis "github.com/boxgo/session/store/redis"
    "github.com/redis/go-redis/v9"
)

cli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store := sessredis.NewRedisStore(cli, "gfkit:session")
mgr := session.NewManager(store, session.WithMode(session.ModeMulti))
```

> 单仓库多包场景下，`go.mod` 仍会收录 `go-redis` 等依赖；**编译层面**仅 `import` `github.com/boxgo/session/store/redis` 的代码会链接 Redis 实现。`store` 下可继续增加其他存储子包。本仓库中 `store/redis` 已是独立 Go module。

## 事件监听

```go
mgr := session.NewManager(
    store,
    session.WithEventEnabled(true),
)

_, ch, cancel := mgr.Subscribe(128)
defer cancel()

go func() {
    for evt := range ch {
        // evt.Type: EventCreated / EventRefreshed / EventDeleted / EventReplaced / EventPurged
    }
}()
```

如需开启事件监听：

```go
mgr := session.NewManager(
    store,
    session.WithEventEnabled(true),
)
```

## API 概览

以下为 `Manager` 对外方法；复杂度针对 **`store/redis` 包中的 `RedisStore`** 后端说明（不含网络 RTT 常数因子）。

**符号约定**

| 符号 | 含义                                                                             |
| ---- | -------------------------------------------------------------------------------- |
| `n`  | 当前存储中的会话总数                                                             |
| `u`  | 存在 `user` 索引键的用户数                                                       |
| `k`  | 指定 `userID` 下的会话数量                                                       |
| `p`  | `payload` 的键值对数量（或序列化后的体量，二者同阶时可合并记为「会话对象大小」） |
| `L`  | 开启事件时，当前活跃监听器数量（`ListenerCount()`）                              |
| `d`  | 一次 `Purge` 中，删除索引里「已到删除时间」的待清理条目数                        |
| `m`  | 一次调用返回或过滤后保留的会话条数（≤ `k`）                                      |

**Manager 方法（`RedisStore`，包路径 `.../store/redis`）**

| 方法                          | 时间复杂度                                       | 空间复杂度（额外）                     | 说明                                                           |
| ----------------------------- | ------------------------------------------------ | -------------------------------------- | -------------------------------------------------------------- |
| `Open`（`ModeMulti`）         | **O(1)**（常数次 Redis 命令，含 1 次 pipeline）  | **O(p)**（序列化与返回会话拷贝）       | 含一次按 `sessionID` 的 `Get`；可能 `Upsert`；单会话模式见下行 |
| `Open`（`ModeSingle`）        | **O(k)**（`ListByUser` + `k` 次量级的 `Upsert`） | **O(k)**（中间会话 ID 列表等）         | 需列出该用户全部会话并软删除「非当前」会话                     |
| `Refresh`                     | **O(1)**（`Get` + 1 次 pipeline `Upsert`）       | **O(p)**                               |                                                                |
| `Get`                         | **O(1)**（`Get` + 反序列化）                     | **O(p)**                               | `activeOnly` 仅多常数次时间判断                                |
| `ListByUser`                  | **O(k)**（`ZRevRange` + 批量 `Get`）             | **O(m)**（返回切片；`m` 为过滤后条数） | `activeOnly=true` 时在 Manager 侧 **O(k)** 扫描过滤            |
| `ListActiveUsers`             | **O(n)**（对每个用户再 `ListByUser(..., true)`） | **O(u)**（活跃用户 ID 列表）           | 会扫遍所有用户；会话总量大时成本明显，适合管理端或低频任务     |
| `Delete`                      | **O(1)**（`Get` + pipeline）                     | **O(p)**（事件开启且带会话快照时同阶） |                                                                |
| `DeleteByUser`                | **O(k)**（先列再逐条 `Upsert`）                  | **O(k)**（返回 ID 列表等）             |                                                                |
| `Purge`                       | **O(d)**（删除索引区间查询 + pipeline 清理）     | **O(d)**（返回 ID 列表等）             | 与本次待清理条数 `d` 相关                                      |
| `Subscribe` / `ListenerCount` | **O(1)**                                         | **O(1)**                               | 与存储后端无关                                                 |
| `publish`（事件开启时）       | **O(L)**                                         | **O(1)**                               | 非阻塞投递，不分配与监听器数成正比的大对象                     |

> **空间**：表中「空间复杂度（额外）」指本次调用在返回切片、序列化缓冲、事件快照等上**新分配**的体量；Redis 侧持久占用约为 **O(n)** 量级的会话主键与 ZSet 索引，不在逐调用表中重复展开。

## 导出函数速查（名称 + 用法）

- `NewManager(store, opts...)`：创建会话管理器，默认多会话、事件关闭
- `Refresh(ctx, sessionID, ttl, deleteAfter)`：从未删除会话刷新；`ttl`、`deleteAfter` 与 `Open` 一致，从当前时刻重算 `ExpiresAt` 与 `DeletedAt`
- `WithMode(ModeSingle|ModeMulti)`：配置单会话或多会话模式
- `WithEventEnabled(bool)`：开启/关闭事件监听能力（默认 `false`）
- `WithNowFunc(fn)`：注入时钟函数（测试场景常用）
- `memory.NewMemoryStore()`（子包 `github.com/boxgo/session/store/memory`）：使用内存后端
- `Subscribe(buffer)`：订阅会话变动事件，返回 `(listenerID, ch, cancel)`
- `ListenerCount()`：查看当前监听器数量（事件关闭时恒为 `0`）

**子包 `github.com/boxgo/session/store/memory`**

- `NewMemoryStore()`：内存 `Store` 实现

**子包 `github.com/boxgo/session/store/redis`**

- `NewRedisStore(redisClient, prefix)`：Redis `Store` 实现（建议传业务前缀）
- `NewRedisStoreWithCodec(redisClient, prefix, codec)`：指定编解码器
- `JSONCodec()` / `MsgpackCodec()`：内置编解码器
- `Codec`：自定义序列化时可实现该接口

## 导出类型速查（名称 + 用法）

- `Session`：会话实体，核心字段包含 `ID`、`UserID`、`ExpiresAt`、`DeletedAt`
- `SessionMode`：会话模式枚举，`ModeSingle`（单会话）/`ModeMulti`（多会话）
- `EventType`：事件类型枚举，`EventCreated` / `EventRefreshed` / `EventDeleted` / `EventReplaced` / `EventPurged`
- `SessionEvent`：事件结构体，包含事件类型、用户、会话ID、时间与可选会话快照
- `Store`：存储后端接口，可自行实现并注入到 `NewManager`
- `Option`：管理器配置函数类型，通过 `opts...` 传入 `NewManager`

示例：

```go
import (
    "github.com/boxgo/session"
    "github.com/boxgo/session/store/memory"
)

store := memory.NewMemoryStore()
mgr := session.NewManager(
    store,
    session.WithMode(session.ModeSingle),
    session.WithEventEnabled(true),
)

_, evtCh, cancel := mgr.Subscribe(128)
defer cancel()

sess, err := mgr.Open(ctx, "u1", "s1", 30*time.Minute, 24*time.Hour, nil)
if err != nil {
    return err
}

_, _ = sess, evtCh
```

使用 msgpack 编码 Redis 会话：

```go
redisStore := sessredis.NewRedisStoreWithCodec(
    redisClient,
    "gfkit:session",
    sessredis.MsgpackCodec(),
)
mgr := session.NewManager(redisStore)
```

## 性能设计

- `memory` 后端：
  - `map + RWMutex`，用户维度二级索引（`user -> sessionIDs`）
  - 读路径无额外序列化，写路径最小锁粒度
- `store/redis`（Redis）后端：
  - 混合清理方案：`sessionKey` 到 `DeletedAt` 自动过期 + 删除索引驱动用户索引清理
  - 会话对象（JSON/msgpack）+ 用户索引 ZSet + 删除索引 ZSet
  - 批量查询与清理使用 pipeline 降低 RTT
- 事件分发：
  - 非阻塞投递（慢消费者不拖累主流程）

## 测试

```bash
go test . -count=1
go test ./store/redis/... -count=1
go test . -bench . -benchmem -run '^$'
go test ./store/redis/... -bench . -benchmem -run '^$'
```

## 参与开发

本仓库为 **多模块 monorepo**（根目录模块 + `store/redis`），根目录已配置 `go.work`。克隆后在**仓库根目录**构建或测试即可：

```bash
git clone https://github.com/boxgo/session.git
cd session
go test ./...
```

若只改 Redis 子模块，也可 `cd store/redis` 后在该目录单独执行 `go test`。

### 发版与 tag（自动化）

同一提交需两个 Git tag 才能分别解析两个 module：根模块为 `vX.Y.Z`，Redis 子模块为 `store/redis/vX.Y.Z`（`go get github.com/boxgo/session/store/redis@vX.Y.Z` 依赖后者，勿仅用根目录 `vX.Y.Z`）。

- **GitHub**：打开仓库 **Actions** → **Tag release** → **Run workflow**，填写版本如 `0.1.0`，工作流会在当前默认分支 HEAD 上创建并推送 `v0.1.0` 与 `store/redis/v0.1.0`，并**自动创建**以 `v0.1.0` 为 tag 的 **GitHub Release**（含两个 module 的 `go get` 说明；定义见 [.github/workflows/tag-release.yml](.github/workflows/tag-release.yml)）。
- **本地**：在目标 commit 上执行 `bash scripts/tag-release.sh 0.1.0`，再按脚本提示 `git push origin …`。

发版后可将 `store/redis/go.mod` 里对主模块的 `require` 升为对应 `v0.1.0`（与根模块 tag 一致），再打一版子模块 tag 或沿用同一次双 tag 流程。
