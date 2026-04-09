package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/boxgo/session"
	"github.com/boxgo/session/store/memory"
)

func TestManagerSingleMode(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	manager := session.NewManager(
		store,
		session.WithMode(session.ModeSingle),
		session.WithNowFunc(func() time.Time { return now }),
	)

	if _, err := manager.Open(ctx, "u1", "s1", 10*time.Minute, time.Hour, map[string]string{"ip": "1.1.1.1"}); err != nil {
		t.Fatalf("open s1 failed: %v", err)
	}

	now = now.Add(10 * time.Second)
	if _, err := manager.Open(ctx, "u1", "s2", 10*time.Minute, time.Hour, map[string]string{"ip": "2.2.2.2"}); err != nil {
		t.Fatalf("open s2 failed: %v", err)
	}

	if _, err := manager.Get(ctx, "s1", true); err == nil {
		t.Fatalf("s1 should not be active in single mode")
	}
	s1, err := manager.Get(ctx, "s1", false)
	if err != nil {
		t.Fatalf("get s1 failed: %v", err)
	}
	if s1.DeletedAt == nil {
		t.Fatalf("s1 should be marked deleted")
	}

	s2, err := manager.Get(ctx, "s2", true)
	if err != nil {
		t.Fatalf("get s2 failed: %v", err)
	}
	if s2.UserID != "u1" {
		t.Fatalf("unexpected user id: %s", s2.UserID)
	}
}

func TestManagerRefreshExpiredNotDeleted(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	manager := session.NewManager(
		store,
		session.WithNowFunc(func() time.Time { return now }),
	)

	if _, err := manager.Open(ctx, "u1", "s1", time.Second, time.Hour, nil); err != nil {
		t.Fatalf("open failed: %v", err)
	}

	now = now.Add(2 * time.Second)
	if _, err := manager.Get(ctx, "s1", true); err == nil {
		t.Fatalf("s1 should be expired")
	}

	session, err := manager.Refresh(ctx, "s1", 10*time.Second)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if !session.ExpiresAt.After(now) {
		t.Fatalf("refresh should extend expiration")
	}

	if _, err := manager.Get(ctx, "s1", true); err != nil {
		t.Fatalf("s1 should be active after refresh: %v", err)
	}
}

func TestManagerRefreshDeletedSession(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	manager := session.NewManager(
		store,
		session.WithNowFunc(func() time.Time { return now }),
	)

	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, time.Second, nil); err != nil {
		t.Fatalf("open failed: %v", err)
	}

	now = now.Add(2 * time.Second)
	if _, err := manager.Refresh(ctx, "s1", time.Minute); err != session.ErrSessionDeleted {
		t.Fatalf("expected ErrSessionDeleted, got: %v", err)
	}
}

func TestManagerListenerAndPurge(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	manager := session.NewManager(
		store,
		session.WithNowFunc(func() time.Time { return now }),
		session.WithEventEnabled(true),
	)
	_, ch, cancel := manager.Subscribe(16)
	defer cancel()

	if _, err := manager.Open(ctx, "u1", "s1", 10*time.Minute, time.Hour, nil); err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if _, err := manager.Refresh(ctx, "s1", time.Hour); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if err := manager.Delete(ctx, "s1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := manager.Purge(ctx); err != nil {
		t.Fatalf("purge failed: %v", err)
	}

	want := map[session.EventType]bool{
		session.EventCreated:   false,
		session.EventRefreshed: false,
		session.EventDeleted:   false,
		session.EventPurged:    false,
	}
	for i := 0; i < 4; i++ {
		select {
		case evt := <-ch:
			want[evt.Type] = true
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting event %d", i)
		}
	}
	for typ, got := range want {
		if !got {
			t.Fatalf("event type %v not received", typ)
		}
	}
}

func TestManagerDeleteByUser(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	manager := session.NewManager(
		store,
		session.WithNowFunc(func() time.Time { return now }),
	)

	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, time.Hour, nil); err != nil {
		t.Fatalf("open s1 failed: %v", err)
	}
	if _, err := manager.Open(ctx, "u1", "s2", time.Hour, time.Hour, nil); err != nil {
		t.Fatalf("open s2 failed: %v", err)
	}
	if _, err := manager.Open(ctx, "u2", "s3", time.Hour, time.Hour, nil); err != nil {
		t.Fatalf("open s3 failed: %v", err)
	}

	if err := manager.DeleteByUser(ctx, "u1"); err != nil {
		t.Fatalf("delete by user failed: %v", err)
	}

	aliveU1, err := manager.ListByUser(ctx, "u1", true)
	if err != nil {
		t.Fatalf("list u1 failed: %v", err)
	}
	if len(aliveU1) != 0 {
		t.Fatalf("u1 should have no active sessions, got %d", len(aliveU1))
	}

	aliveU2, err := manager.ListByUser(ctx, "u2", true)
	if err != nil {
		t.Fatalf("list u2 failed: %v", err)
	}
	if len(aliveU2) != 1 || aliveU2[0].ID != "s3" {
		t.Fatalf("u2 should keep s3")
	}
}

func TestManagerEventDisabled(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()
	manager := session.NewManager(
		store,
		session.WithEventEnabled(false),
	)

	id, ch, cancel := manager.Subscribe(8)
	defer cancel()
	if id != 0 {
		t.Fatalf("disabled event subscribe id should be 0, got %d", id)
	}
	if manager.ListenerCount() != 0 {
		t.Fatalf("listener count should be 0 when disabled")
	}

	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, time.Hour, nil); err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if _, err := manager.Refresh(ctx, "s1", time.Hour); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if err := manager.Delete(ctx, "s1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("channel should be closed when events disabled")
		}
	default:
		t.Fatalf("channel should be closed and readable immediately")
	}
}

func TestManagerListActiveUsers(t *testing.T) {
	ctx := context.Background()
	store := memory.NewMemoryStore()

	now := time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC)
	manager := session.NewManager(
		store,
		session.WithNowFunc(func() time.Time { return now }),
	)

	if _, err := manager.Open(ctx, "u1", "s1", time.Minute, time.Hour, nil); err != nil {
		t.Fatalf("open s1 failed: %v", err)
	}
	if _, err := manager.Open(ctx, "u2", "s2", time.Second, time.Hour, nil); err != nil {
		t.Fatalf("open s2 failed: %v", err)
	}

	now = now.Add(2 * time.Second) // u2 expired
	if err := manager.Delete(ctx, "s1"); err != nil {
		t.Fatalf("delete s1 failed: %v", err)
	}

	users, err := manager.ListActiveUsers(ctx)
	if err != nil {
		t.Fatalf("list active users failed: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected no active users, got %v", users)
	}

	if _, err := manager.Refresh(ctx, "s2", time.Minute); err != nil {
		t.Fatalf("refresh s2 failed: %v", err)
	}
	users, err = manager.ListActiveUsers(ctx)
	if err != nil {
		t.Fatalf("list active users failed: %v", err)
	}
	hasU2 := false
	for _, u := range users {
		if u == "u2" {
			hasU2 = true
			break
		}
	}
	if len(users) != 1 || !hasU2 {
		t.Fatalf("expected active user u2, got %v", users)
	}
}
