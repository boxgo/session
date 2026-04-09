package session

import (
	"context"
	"time"
)

// Store 定义可替换的会话存储后端接口。
type Store interface {
	// Upsert 新增或更新会话记录。
	Upsert(ctx context.Context, session *Session) error
	// Get 按 ID 查询单个会话。
	Get(ctx context.Context, sessionID string) (*Session, error)
	// ListUsers 查询当前存储中的全部用户ID（去重）。
	ListUsers(ctx context.Context) ([]string, error)
	// ListByUser 查询某个用户的全部会话。
	ListByUser(ctx context.Context, userID string) ([]*Session, error)
	// Delete 将单个会话在 deletedAt 标记为删除态。
	Delete(ctx context.Context, sessionID string, deletedAt time.Time) error
	// DeleteByUser 将某个用户下全部会话标记为删除态。
	DeleteByUser(ctx context.Context, userID string, deletedAt time.Time) ([]string, error)
	// Purge 物理删除达到删除时间的会话。
	Purge(ctx context.Context, now time.Time) ([]string, error)
}
