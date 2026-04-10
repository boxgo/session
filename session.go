package session

import "time"

type SessionMode uint8

const (
	// ModeMulti 多会话模式：同一用户可并存多个会话。
	ModeMulti SessionMode = iota + 1
	// ModeSingle 单会话模式：同一用户仅允许一个活跃会话。
	ModeSingle
)

type EventType uint8

const (
	// EventCreated 会话创建/打开事件。
	EventCreated EventType = iota + 1
	// EventRefreshed 会话刷新事件（过期时间与删除截止时间按 Refresh 参数更新）。
	EventRefreshed
	// EventDeleted 会话软删除事件。
	EventDeleted
	// EventReplaced 会话被替换事件。
	EventReplaced
	// EventPurged 会话物理清理事件。
	EventPurged
)

// Session 表示用户会话实体。
// DeletedAt 为零值表示未设置删除时间。
type Session struct {
	ID        string            `json:"id"`
	UserID    string            `json:"userId"`
	Payload   map[string]string `json:"payload,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	ExpiresAt time.Time         `json:"expiresAt"`
	DeletedAt time.Time         `json:"deletedAt"`
}

// ActiveAt 判断会话在指定时间点是否处于活跃状态。
func (s *Session) ActiveAt(now time.Time) bool {
	if s == nil {
		return false
	}
	if !s.DeletedAt.IsZero() && !s.DeletedAt.After(now) {
		return false
	}

	return s.ExpiresAt.After(now)
}

// Clone 返回会话的深拷贝（含 Payload）。
func (s *Session) Clone() *Session {
	if s == nil {
		return nil
	}

	cp := *s
	if s.Payload != nil {
		cp.Payload = make(map[string]string, len(s.Payload))
		for k, v := range s.Payload {
			cp.Payload[k] = v
		}
	}

	return &cp
}

// SessionEvent 描述会话生命周期变动事件。
type SessionEvent struct {
	Type      EventType `json:"type"`
	SessionID string    `json:"sessionId"`
	UserID    string    `json:"userId"`
	At        time.Time `json:"at"`
	Session   *Session  `json:"session,omitempty"`
}
