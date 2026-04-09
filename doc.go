// Package session 提供高性能用户会话管理能力。
//
// 特性：
//   - 支持单会话与多会话模式
//   - 支持过期时间与删除时间双生命周期
//   - 支持“已过期但未删除”会话刷新恢复
//   - 支持会话事件发布/订阅（可开关）
//   - 支持可替换存储后端（内存实现见子包 session/store/memory；Redis 见 session/store/redis）
package session
