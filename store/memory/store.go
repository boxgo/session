package memory

import (
	"context"
	"sync"
	"time"

	"github.com/boxgo/session"
)

// MemoryStore 是基于内存的 session.Store 实现，适用于低时延场景。
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*session.Session
	userIdx  map[string]map[string]struct{}
}

// NewMemoryStore 创建一个空的 MemoryStore。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*session.Session, 1024),
		userIdx:  make(map[string]map[string]struct{}, 256),
	}
}

// Upsert 在内存中新增或更新会话。
func (s *MemoryStore) Upsert(_ context.Context, sess *session.Session) error {
	if sess == nil || sess.ID == "" || sess.UserID == "" {
		return session.ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.sessions[sess.ID]
	if ok && old.UserID != sess.UserID {
		delete(s.userIdx[old.UserID], sess.ID)
		if len(s.userIdx[old.UserID]) == 0 {
			delete(s.userIdx, old.UserID)
		}
	}
	s.sessions[sess.ID] = sess.Clone()

	idx := s.userIdx[sess.UserID]
	if idx == nil {
		idx = make(map[string]struct{}, 8)
		s.userIdx[sess.UserID] = idx
	}
	idx[sess.ID] = struct{}{}

	return nil
}

// Get 按 ID 查询单个会话。
func (s *MemoryStore) Get(_ context.Context, sessionID string) (*session.Session, error) {
	if sessionID == "" {
		return nil, session.ErrInvalidArgument
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, session.ErrSessionNotFound
	}

	return sess.Clone(), nil
}

// ListUsers 查询当前内存中的全部用户ID。
func (s *MemoryStore) ListUsers(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ans := make([]string, 0, len(s.userIdx))
	for userID := range s.userIdx {
		ans = append(ans, userID)
	}

	return ans, nil
}

// ListByUser 查询某个用户的全部会话。
func (s *MemoryStore) ListByUser(_ context.Context, userID string) ([]*session.Session, error) {
	if userID == "" {
		return nil, session.ErrInvalidArgument
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	idx := s.userIdx[userID]
	if idx == nil {
		return nil, nil
	}

	ans := make([]*session.Session, 0, len(idx))
	for sessionID := range idx {
		if sess, ok := s.sessions[sessionID]; ok {
			ans = append(ans, sess.Clone())
		}
	}

	return ans, nil
}

// Delete 将单个会话标记为删除态（软删除）。
func (s *MemoryStore) Delete(_ context.Context, sessionID string, deletedAt time.Time) error {
	if sessionID == "" {
		return session.ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return session.ErrSessionNotFound
	}
	// 这里只做软删除，物理清理在 Purge 中完成。
	sess.DeletedAt = &deletedAt
	sess.UpdatedAt = deletedAt
	s.sessions[sessionID] = sess

	return nil
}

// DeleteByUser 将某个用户下全部会话标记为删除态。
func (s *MemoryStore) DeleteByUser(_ context.Context, userID string, deletedAt time.Time) ([]string, error) {
	if userID == "" {
		return nil, session.ErrInvalidArgument
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.userIdx[userID]
	if idx == nil {
		return nil, nil
	}
	ans := make([]string, 0, len(idx))
	for sessionID := range idx {
		if sess, ok := s.sessions[sessionID]; ok {
			sess.DeletedAt = &deletedAt
			sess.UpdatedAt = deletedAt
			s.sessions[sessionID] = sess
			ans = append(ans, sessionID)
		}
	}

	return ans, nil
}

// Purge 物理删除达到删除时间的会话。
func (s *MemoryStore) Purge(_ context.Context, now time.Time) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := make([]string, 0, 64)
	for sessionID, sess := range s.sessions {
		if sess.DeletedAt != nil && !sess.DeletedAt.After(now) {
			delete(s.sessions, sessionID)
			if idx := s.userIdx[sess.UserID]; idx != nil {
				delete(idx, sessionID)
				if len(idx) == 0 {
					delete(s.userIdx, sess.UserID)
				}
			}
			removed = append(removed, sessionID)
		}
	}

	return removed, nil
}
