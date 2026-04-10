package session

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Manager 提供用户会话生命周期管理能力。
type Manager struct {
	store Store
	mode  SessionMode
	nowFn func() time.Time
	// eventEnabled 控制是否开启会话事件发布与订阅。
	eventEnabled bool

	listenerSeq atomic.Uint64
	listenerMu  sync.RWMutex
	listeners   map[uint64]chan SessionEvent
}

// NewManager 创建会话管理器。
// 默认值：
//   - 模式：ModeMulti
//   - 事件：eventEnabled=false（关闭）
func NewManager(store Store, opts ...Option) *Manager {
	m := &Manager{
		store:        store,
		mode:         ModeMulti,
		nowFn:        time.Now,
		eventEnabled: false,
		listeners:    make(map[uint64]chan SessionEvent),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Open 创建或更新会话。
// ttl 控制过期时间，deleteAfter 控制删除时间（到点后不可刷新）。
func (m *Manager) Open(
	ctx context.Context,
	userID, sessionID string,
	ttl, deleteAfter time.Duration,
	payload map[string]string,
) (*Session, error) {
	if userID == "" || sessionID == "" || ttl <= 0 {
		return nil, ErrInvalidArgument
	}

	now := m.nowFn()
	if m.mode == ModeSingle {
		// 单会话模式下，先软删除同用户其他会话。
		replacedIDs, err := m.deleteOtherSessions(ctx, userID, sessionID, now)
		if err != nil {
			return nil, err
		}
		for _, id := range replacedIDs {
			m.publish(SessionEvent{
				Type:      EventReplaced,
				SessionID: id,
				UserID:    userID,
				At:        now,
			})
		}
	}

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Payload:   cloneMap(payload),
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(ttl),
		DeletedAt: calcDeletedAt(now, deleteAfter),
	}

	if current, err := m.store.Get(ctx, sessionID); err == nil && current != nil {
		if current.UserID != userID {
			// 同一个 sessionID 被其他用户占用时，先下线旧会话归属。
			if err := m.store.Delete(ctx, sessionID, now); err != nil {
				return nil, err
			}
			if m.eventEnabled {
				m.publish(SessionEvent{
					Type:      EventReplaced,
					SessionID: sessionID,
					UserID:    current.UserID,
					At:        now,
					Session:   current.Clone(),
				})
			}
		} else {
			session.CreatedAt = current.CreatedAt
		}
	}

	if err := m.store.Upsert(ctx, session); err != nil {
		return nil, err
	}

	if m.eventEnabled {
		m.publish(SessionEvent{
			Type:      EventCreated,
			SessionID: session.ID,
			UserID:    session.UserID,
			At:        now,
			Session:   session.Clone(),
		})
	}

	return session.Clone(), nil
}

// Refresh 刷新会话：用当前时间与 Open 相同的语义重算 ExpiresAt（now+ttl）与 DeletedAt（calcDeletedAt(now, deleteAfter)）；仅未到删除时间的会话可刷新。
func (m *Manager) Refresh(ctx context.Context, sessionID string, ttl, deleteAfter time.Duration) (*Session, error) {
	if sessionID == "" || ttl <= 0 {
		return nil, ErrInvalidArgument
	}

	now := m.nowFn()
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if !session.DeletedAt.IsZero() && !session.DeletedAt.After(now) {
		return nil, ErrSessionDeleted
	}

	session.UpdatedAt = now
	session.ExpiresAt = now.Add(ttl)
	session.DeletedAt = calcDeletedAt(now, deleteAfter)
	if err := m.store.Upsert(ctx, session); err != nil {
		return nil, err
	}

	if m.eventEnabled {
		m.publish(SessionEvent{
			Type:      EventRefreshed,
			SessionID: session.ID,
			UserID:    session.UserID,
			At:        now,
			Session:   session.Clone(),
		})
	}

	return session.Clone(), nil
}

// Get 按 sessionID 查询会话。
// activeOnly=true 时，会过滤掉过期或已删除会话。
func (m *Manager) Get(ctx context.Context, sessionID string, activeOnly bool) (*Session, error) {
	if sessionID == "" {
		return nil, ErrInvalidArgument
	}

	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	if activeOnly && !session.ActiveAt(m.nowFn()) {
		return nil, ErrSessionNotFound
	}

	return session.Clone(), nil
}

// ListByUser 查询某用户下的会话列表。
// activeOnly=true 时，仅返回活跃会话。
func (m *Manager) ListByUser(ctx context.Context, userID string, activeOnly bool) ([]*Session, error) {
	if userID == "" {
		return nil, ErrInvalidArgument
	}

	sessions, err := m.store.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := m.nowFn()
	ans := make([]*Session, 0, len(sessions))
	for _, session := range sessions {
		if !activeOnly || session.ActiveAt(now) {
			ans = append(ans, session.Clone())
		}
	}

	return ans, nil
}

// ListActiveUsers 查询当前活跃用户列表（至少存在一个活跃会话）。
func (m *Manager) ListActiveUsers(ctx context.Context) ([]string, error) {
	userIDs, err := m.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	ans := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		sessions, e := m.ListByUser(ctx, userID, true)
		if e != nil {
			return nil, e
		}
		if len(sessions) > 0 {
			ans = append(ans, userID)
		}
	}

	return ans, nil
}

// Delete 将单个会话标记为已删除（软删除）。
func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ErrInvalidArgument
	}

	now := m.nowFn()
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	if err := m.store.Delete(ctx, sessionID, now); err != nil {
		return err
	}

	if m.eventEnabled {
		m.publish(SessionEvent{
			Type:      EventDeleted,
			SessionID: sessionID,
			UserID:    session.UserID,
			At:        now,
			Session:   session.Clone(),
		})
	}

	return nil
}

// DeleteByUser 将某个用户下全部会话标记为已删除。
func (m *Manager) DeleteByUser(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalidArgument
	}

	now := m.nowFn()
	ids, err := m.store.DeleteByUser(ctx, userID, now)
	if err != nil {
		return err
	}

	for _, id := range ids {
		m.publish(SessionEvent{
			Type:      EventDeleted,
			SessionID: id,
			UserID:    userID,
			At:        now,
		})
	}

	return nil
}

// Purge 物理清理达到删除时间的会话。
func (m *Manager) Purge(ctx context.Context) ([]string, error) {
	now := m.nowFn()
	ids, err := m.store.Purge(ctx, now)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		m.publish(SessionEvent{
			Type:      EventPurged,
			SessionID: id,
			At:        now,
		})
	}

	return ids, nil
}

// Subscribe 注册会话事件监听器。
// 返回值依次是：监听器ID、事件通道、取消订阅函数。
func (m *Manager) Subscribe(buffer int) (uint64, <-chan SessionEvent, func()) {
	if !m.eventEnabled {
		// 事件关闭时返回一个已关闭通道，调用方可安全 range 退出。
		ch := make(chan SessionEvent)
		close(ch)
		return 0, ch, func() {}
	}

	if buffer < 0 {
		buffer = 0
	}
	ch := make(chan SessionEvent, buffer)
	id := m.listenerSeq.Add(1)

	m.listenerMu.Lock()
	m.listeners[id] = ch
	m.listenerMu.Unlock()

	cancel := func() {
		m.listenerMu.Lock()
		if c, ok := m.listeners[id]; ok {
			delete(m.listeners, id)
			close(c)
		}
		m.listenerMu.Unlock()
	}

	return id, ch, cancel
}

// ListenerCount 返回当前活跃监听器数量。
func (m *Manager) ListenerCount() int {
	if !m.eventEnabled {
		return 0
	}

	m.listenerMu.RLock()
	defer m.listenerMu.RUnlock()

	return len(m.listeners)
}

func (m *Manager) publish(evt SessionEvent) {
	if !m.eventEnabled {
		return
	}

	m.listenerMu.RLock()
	defer m.listenerMu.RUnlock()

	for _, ch := range m.listeners {
		// 非阻塞投递：慢消费者会被丢弃，避免影响主链路性能。
		select {
		case ch <- evt:
		default:
		}
	}
}

func (m *Manager) deleteOtherSessions(ctx context.Context, userID, keepSessionID string, now time.Time) ([]string, error) {
	sessions, err := m.store.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	replaced := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if session.ID == keepSessionID {
			continue
		}
		if err := m.store.Delete(ctx, session.ID, now); err != nil {
			return nil, err
		}
		replaced = append(replaced, session.ID)
	}

	return replaced, nil
}

func calcDeletedAt(now time.Time, deleteAfter time.Duration) time.Time {
	if deleteAfter <= 0 {
		return time.Time{}
	}
	return now.Add(deleteAfter)
}

func cloneMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	cp := make(map[string]string, len(src))
	for k, v := range src {
		cp[k] = v
	}

	return cp
}
