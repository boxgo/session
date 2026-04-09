package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/boxgo/session"
	sessredis "github.com/boxgo/session/store/redis"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisStoreLifecycle(t *testing.T) {
	cases := []struct {
		name  string
		codec sessredis.Codec
	}{
		{name: "json", codec: sessredis.JSONCodec()},
		{name: "msgpack", codec: sessredis.MsgpackCodec()},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svr, err := miniredis.Run()
			if err != nil {
				t.Fatalf("start miniredis failed: %v", err)
			}
			defer svr.Close()

			client := goredis.NewClient(&goredis.Options{Addr: svr.Addr()})
			defer client.Close()

			store := sessredis.NewRedisStoreWithCodec(client, "testsession", tc.codec)
			manager := session.NewManager(store)
			ctx := context.Background()

			created, err := manager.Open(ctx, "u1", "s1", time.Minute, time.Hour, map[string]string{"d": "1"})
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			if created.ID != "s1" {
				t.Fatalf("unexpected session id: %s", created.ID)
			}

			got, err := manager.Get(ctx, "s1", true)
			if err != nil {
				t.Fatalf("get failed: %v", err)
			}
			if got.Payload["d"] != "1" {
				t.Fatalf("unexpected payload")
			}

			list, err := manager.ListByUser(ctx, "u1", false)
			if err != nil {
				t.Fatalf("list failed: %v", err)
			}
			if len(list) != 1 || list[0].ID != "s1" {
				t.Fatalf("list mismatch: %+v", list)
			}

			if err := manager.Delete(ctx, "s1"); err != nil {
				t.Fatalf("delete failed: %v", err)
			}

			time.Sleep(10 * time.Millisecond)
			if _, err := manager.Purge(ctx); err != nil {
				t.Fatalf("purge failed: %v", err)
			}
		})
	}
}

func TestRedisStoreHybridExpireAndPurgeIndex(t *testing.T) {
	svr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	defer svr.Close()

	client := goredis.NewClient(&goredis.Options{Addr: svr.Addr()})
	defer client.Close()

	now := time.Now()
	store := sessredis.NewRedisStore(client, "hybrid")
	manager := session.NewManager(
		store,
		session.WithNowFunc(func() time.Time { return now }),
	)
	ctx := context.Background()

	if _, err := manager.Open(ctx, "u1", "s1", time.Hour, 2*time.Second, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("open failed: %v", err)
	}

	if exists, err := client.Exists(ctx, "hybrid:session:s1").Result(); err != nil || exists != 1 {
		t.Fatalf("session key should exist, exists=%d err=%v", exists, err)
	}
	if cnt, err := client.ZCard(ctx, "hybrid:user:u1").Result(); err != nil || cnt != 1 {
		t.Fatalf("user index should contain one session, cnt=%d err=%v", cnt, err)
	}

	svr.FastForward(3 * time.Second)
	if exists, err := client.Exists(ctx, "hybrid:session:s1").Result(); err != nil || exists != 0 {
		t.Fatalf("session key should auto expire, exists=%d err=%v", exists, err)
	}

	now = now.Add(3 * time.Second)
	if _, err := manager.Get(ctx, "s1", false); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after key expire, got %v", err)
	}

	if _, err := manager.Purge(ctx); err != nil {
		t.Fatalf("purge failed: %v", err)
	}
	if cnt, err := client.ZCard(ctx, "hybrid:user:u1").Result(); err != nil || cnt != 0 {
		t.Fatalf("user index should be cleaned, cnt=%d err=%v", cnt, err)
	}
	if cnt, err := client.ZCard(ctx, "hybrid:deleted").Result(); err != nil || cnt != 0 {
		t.Fatalf("deleted index should be cleaned, cnt=%d err=%v", cnt, err)
	}
}
