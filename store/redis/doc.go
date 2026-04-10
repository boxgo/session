// Package redis 提供基于 Redis 的 session.Store 实现及可选编解码器（encoding/json、sonic、msgpack）。
// 按需 import 本包即可；主包 session 不依赖 go-redis（独立 Go module 下可选依赖更清晰，同仓库仍由根 go.mod 统一收录）。
package redis
