package redis_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/boxgo/session"
	sessredis "github.com/boxgo/session/store/redis"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func BenchmarkManagerOpenRedisMulti(b *testing.B) {
	ctx := context.Background()
	manager, cleanup := newRedisBenchmarkManager(b)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, "u"+id, "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("open failed: %v", err)
		}
	}
}

func BenchmarkManagerOpenRedisMultiEventOff(b *testing.B) {
	ctx := context.Background()
	manager, cleanup := newRedisBenchmarkManagerWithEvent(b, false)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := strconv.Itoa(i)
		if _, err := manager.Open(ctx, "u"+id, "s"+id, time.Hour, 24*time.Hour, nil); err != nil {
			b.Fatalf("open failed: %v", err)
		}
	}
}

func BenchmarkManagerRefreshRedis(b *testing.B) {
	ctx := context.Background()
	manager, cleanup := newRedisBenchmarkManager(b)
	defer cleanup()

	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, 24*time.Hour, nil); err != nil {
		b.Fatalf("open failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.Refresh(ctx, "s1", time.Hour, 24*time.Hour); err != nil {
			b.Fatalf("refresh failed: %v", err)
		}
	}
}

func BenchmarkManagerRefreshRedisEventOff(b *testing.B) {
	ctx := context.Background()
	manager, cleanup := newRedisBenchmarkManagerWithEvent(b, false)
	defer cleanup()

	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, 24*time.Hour, nil); err != nil {
		b.Fatalf("open failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.Refresh(ctx, "s1", time.Hour, 24*time.Hour); err != nil {
			b.Fatalf("refresh failed: %v", err)
		}
	}
}

func BenchmarkManagerOpenRedisCodecEventOff(b *testing.B) {
	benchRedisCodecEventOffOpen(b, sessredis.JSONCodec())
	benchRedisCodecEventOffOpen(b, sessredis.MsgpackCodec())
}

func BenchmarkManagerRefreshRedisCodecEventOff(b *testing.B) {
	benchRedisCodecEventOffRefresh(b, sessredis.JSONCodec())
	benchRedisCodecEventOffRefresh(b, sessredis.MsgpackCodec())
}

func BenchmarkManagerListByUserRedis(b *testing.B) {
	ctx := context.Background()
	manager, cleanup := newRedisBenchmarkManager(b)
	defer cleanup()

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

func BenchmarkManagerListByUserRedisEventOff(b *testing.B) {
	ctx := context.Background()
	manager, cleanup := newRedisBenchmarkManagerWithEvent(b, false)
	defer cleanup()

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

func BenchmarkManagerListByUserRedisScale(b *testing.B) {
	scales := []int{10, 100, 1000, 5000}

	for _, scale := range scales {
		scale := scale
		b.Run("sessions_"+strconv.Itoa(scale), func(b *testing.B) {
			ctx := context.Background()
			manager, cleanup := newRedisBenchmarkManager(b)
			defer cleanup()
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

func BenchmarkManagerOpenRedisScale(b *testing.B) {
	scales := []int{10, 100, 1000, 5000}

	for _, scale := range scales {
		scale := scale
		b.Run("sessions_"+strconv.Itoa(scale), func(b *testing.B) {
			ctx := context.Background()
			manager, cleanup := newRedisBenchmarkManagerWithEvent(b, false)
			defer cleanup()
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

func BenchmarkManagerGetRedisScale(b *testing.B) {
	scales := []int{10, 100, 1000, 5000}

	for _, scale := range scales {
		scale := scale
		b.Run("sessions_"+strconv.Itoa(scale), func(b *testing.B) {
			ctx := context.Background()
			manager, cleanup := newRedisBenchmarkManagerWithEvent(b, false)
			defer cleanup()
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

func newRedisBenchmarkManager(b *testing.B) (*session.Manager, func()) {
	return newRedisBenchmarkManagerWithCodecAndEvent(b, sessredis.JSONCodec(), true)
}

func newRedisBenchmarkManagerWithEvent(b *testing.B, eventEnabled bool) (*session.Manager, func()) {
	return newRedisBenchmarkManagerWithCodecAndEvent(b, sessredis.JSONCodec(), eventEnabled)
}

func newRedisBenchmarkManagerWithCodecAndEvent(b *testing.B, codec sessredis.Codec, eventEnabled bool) (*session.Manager, func()) {
	b.Helper()

	svr, err := miniredis.Run()
	if err != nil {
		b.Fatalf("start miniredis failed: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: svr.Addr()})
	store := sessredis.NewRedisStoreWithCodec(client, "bench", codec)
	manager := session.NewManager(
		store,
		session.WithMode(session.ModeMulti),
		session.WithEventEnabled(eventEnabled),
	)

	cleanup := func() {
		_ = client.Close()
		svr.Close()
	}

	return manager, cleanup
}

func benchRedisCodecEventOffOpen(b *testing.B, codec sessredis.Codec) {
	b.Helper()
	b.Run(codec.Name(), func(b *testing.B) {
		ctx := context.Background()
		manager, cleanup := newRedisBenchmarkManagerWithCodecAndEvent(b, codec, false)
		defer cleanup()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id := strconv.Itoa(i)
			payload := map[string]string{
				"device":  "ios",
				"ip":      "127.0.0.1",
				"ua":      "Mozilla/5.0",
				"version": "1.0.0",
				"biz":     "session",
			}
			if _, err := manager.Open(ctx, "u"+id, "s"+id, time.Hour, 24*time.Hour, payload); err != nil {
				b.Fatalf("open failed: %v", err)
			}
		}
	})
}

func benchRedisCodecEventOffRefresh(b *testing.B, codec sessredis.Codec) {
	b.Helper()
	b.Run(codec.Name(), func(b *testing.B) {
		ctx := context.Background()
		manager, cleanup := newRedisBenchmarkManagerWithCodecAndEvent(b, codec, false)
		defer cleanup()

		payload := map[string]string{
			"device":  "ios",
			"ip":      "127.0.0.1",
			"ua":      "Mozilla/5.0",
			"version": "1.0.0",
			"biz":     "session",
		}
		if _, err := manager.Open(ctx, "u1", "s1", time.Hour, 24*time.Hour, payload); err != nil {
			b.Fatalf("open failed: %v", err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := manager.Refresh(ctx, "s1", time.Hour, 24*time.Hour); err != nil {
				b.Fatalf("refresh failed: %v", err)
			}
		}
	})
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
