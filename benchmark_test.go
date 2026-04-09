package session_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/boxgo/session"
	"github.com/boxgo/session/store/memory"
)

func BenchmarkManagerOpenMemoryMulti(b *testing.B) {
	ctx := context.Background()
	manager := session.NewManager(
		memory.NewMemoryStore(),
		session.WithMode(session.ModeMulti),
		session.WithEventEnabled(true),
	)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, "u"+id, "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("open failed: %v", err)
		}
	}
}

func BenchmarkManagerRefreshMemory(b *testing.B) {
	ctx := context.Background()
	manager := session.NewManager(
		memory.NewMemoryStore(),
		session.WithEventEnabled(true),
	)
	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, 24*time.Hour, nil); err != nil {
		b.Fatalf("open failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.Refresh(ctx, "s1", time.Hour); err != nil {
			b.Fatalf("refresh failed: %v", err)
		}
	}
}

func BenchmarkManagerListByUserMemory(b *testing.B) {
	ctx := context.Background()
	manager := session.NewManager(
		memory.NewMemoryStore(),
		session.WithEventEnabled(true),
	)
	for i := 0; i < 1024; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, "u1", "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("seed open failed: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.ListByUser(ctx, "u1", false); err != nil {
			b.Fatalf("list failed: %v", err)
		}
	}
}

func BenchmarkManagerOpenMemoryMultiEventOff(b *testing.B) {
	ctx := context.Background()
	manager := session.NewManager(
		memory.NewMemoryStore(),
		session.WithMode(session.ModeMulti),
		session.WithEventEnabled(false),
	)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, "u"+id, "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("open failed: %v", err)
		}
	}
}

func BenchmarkManagerRefreshMemoryEventOff(b *testing.B) {
	ctx := context.Background()
	manager := session.NewManager(
		memory.NewMemoryStore(),
		session.WithEventEnabled(false),
	)
	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, 24*time.Hour, nil); err != nil {
		b.Fatalf("open failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.Refresh(ctx, "s1", time.Hour); err != nil {
			b.Fatalf("refresh failed: %v", err)
		}
	}
}

func BenchmarkManagerListByUserMemoryEventOff(b *testing.B) {
	ctx := context.Background()
	manager := session.NewManager(
		memory.NewMemoryStore(),
		session.WithEventEnabled(false),
	)
	for i := 0; i < 1024; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, "u1", "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("seed open failed: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.ListByUser(ctx, "u1", false); err != nil {
			b.Fatalf("list failed: %v", err)
		}
	}
}

func BenchmarkManagerListByUserMemoryScale(b *testing.B) {
	scales := []int{10, 100, 1000, 5000}

	for _, scale := range scales {
		scale := scale
		b.Run("sessions_"+strconv.Itoa(scale), func(b *testing.B) {
			ctx := context.Background()
			manager := session.NewManager(
				memory.NewMemoryStore(),
				session.WithEventEnabled(true),
			)
			seedUserSessions(b, ctx, manager, "u1", scale)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := manager.ListByUser(ctx, "u1", false); err != nil {
					b.Fatalf("list failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkManagerOpenMemoryScale(b *testing.B) {
	scales := []int{10, 100, 1000, 5000}

	for _, scale := range scales {
		scale := scale
		b.Run("sessions_"+strconv.Itoa(scale), func(b *testing.B) {
			ctx := context.Background()
			manager := session.NewManager(
				memory.NewMemoryStore(),
				session.WithEventEnabled(false),
			)
			seedGlobalSessions(b, ctx, manager, scale)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				id := "bench_" + strconv.Itoa(i)
				if _, err := manager.Open(ctx, "ubench", id, time.Hour, 24*time.Hour, nil); err != nil {
					b.Fatalf("open failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkManagerGetMemoryScale(b *testing.B) {
	scales := []int{10, 100, 1000, 5000}

	for _, scale := range scales {
		scale := scale
		b.Run("sessions_"+strconv.Itoa(scale), func(b *testing.B) {
			ctx := context.Background()
			manager := session.NewManager(
				memory.NewMemoryStore(),
				session.WithEventEnabled(false),
			)
			sessionIDs := seedGlobalSessions(b, ctx, manager, scale)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				id := sessionIDs[i%len(sessionIDs)]
				if _, err := manager.Get(ctx, id, false); err != nil {
					b.Fatalf("get failed: %v", err)
				}
			}
		})
	}
}

func seedUserSessions(b *testing.B, ctx context.Context, manager *session.Manager, userID string, count int) {
	b.Helper()
	for i := 0; i < count; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, userID, "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("seed open failed: %v", err)
		}
	}
}

func seedGlobalSessions(b *testing.B, ctx context.Context, manager *session.Manager, count int) []string {
	b.Helper()
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		id := "seed_" + strconv.Itoa(i)
		userID := "u" + strconv.Itoa(i%64)
		if _, err := manager.Open(ctx, userID, id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("seed open failed: %v", err)
		}
		ids = append(ids, id)
	}

	return ids
}
