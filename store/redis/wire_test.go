package redis_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/boxgo/session"
	sessredis "github.com/boxgo/session/store/redis"
	"github.com/vmihailenco/msgpack/v5"
)

func TestWireJSONRoundTrip(t *testing.T) {
	codec := sessredis.JSONCodec()
	now := time.Date(2026, 4, 9, 12, 0, 0, 123456789, time.UTC)
	del := now.Add(time.Hour)
	s := &session.Session{
		ID:        "sid",
		UserID:    "uid",
		Payload:   map[string]string{"k": "v"},
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute),
		DeletedAt: del,
	}
	b, err := codec.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var got session.Session
	if err := codec.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || got.UserID != s.UserID || got.Payload["k"] != "v" {
		t.Fatalf("meta mismatch: %+v", got)
	}
	if got.CreatedAt.Unix() != now.Unix() || got.UpdatedAt.Unix() != now.Unix() {
		t.Fatalf("time unix: got c=%v u=%v", got.CreatedAt, got.UpdatedAt)
	}
	if got.ExpiresAt.Unix() != s.ExpiresAt.Unix() || got.DeletedAt.Unix() != del.Unix() {
		t.Fatal("expires/deleted unix mismatch")
	}
}

func TestWireJSONLegacyRFC3339Keys(t *testing.T) {
	codec := sessredis.JSONCodec()
	raw := `{"id":"a","userId":"b","payload":{"x":"y"},"createdAt":"2026-04-09T10:00:00Z","updatedAt":"2026-04-09T10:00:00Z","expiresAt":"2026-04-09T11:00:00Z"}`
	var s session.Session
	if err := codec.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if s.ID != "a" || s.UserID != "b" || s.Payload["x"] != "y" {
		t.Fatalf("got %+v", s)
	}
	if s.CreatedAt.IsZero() || s.ExpiresAt.IsZero() {
		t.Fatal("times not set")
	}
}

func TestWireSonicRoundTrip(t *testing.T) {
	codec := sessredis.SonicCodec()
	now := time.Unix(1712650800, 0).UTC()
	s := &session.Session{
		ID: "sid", UserID: "uid",
		Payload:   map[string]string{"k": "v"},
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	b, err := codec.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var got session.Session
	if err := codec.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || got.Payload["k"] != "v" || got.ExpiresAt.Unix() != s.ExpiresAt.Unix() {
		t.Fatalf("got %+v", got)
	}
}

func TestWireMsgpackRoundTrip(t *testing.T) {
	codec := sessredis.MsgpackCodec()
	now := time.Unix(1712650800, 0).UTC()
	s := &session.Session{
		ID: "i", UserID: "u",
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	b, err := codec.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var got session.Session
	if err := codec.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || got.ExpiresAt.Unix() != s.ExpiresAt.Unix() {
		t.Fatalf("got %+v", got)
	}
}

func TestSessionDefaultJSONNotCompact(t *testing.T) {
	s := &session.Session{ID: "x", UserID: "y", CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(), ExpiresAt: time.Unix(3, 0).UTC()}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == `null` {
		t.Fatal("unexpected null")
	}
	// 默认 json 使用导出字段名，不应为短键 i,u,c...
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["ID"]; !ok {
		t.Fatalf("expected default JSON field ID, got keys %v", m)
	}
}

func TestWireMsgpackMatchesManual(t *testing.T) {
	s := &session.Session{ID: "a", UserID: "b", CreatedAt: time.Unix(10, 0).UTC(), UpdatedAt: time.Unix(11, 0).UTC(), ExpiresAt: time.Unix(12, 0).UTC()}
	c := sessredis.MsgpackCodec()
	cb, err := c.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	// 与 sessionWire 形态一致
	wb, err := msgpack.Marshal(struct {
		I string            `msgpack:"i"`
		U string            `msgpack:"u"`
		P map[string]string `msgpack:"p,omitempty"`
		C int64             `msgpack:"c"`
		M int64             `msgpack:"m"`
		E int64             `msgpack:"e"`
		D *int64            `msgpack:"d,omitempty"`
	}{I: "a", U: "b", C: 10, M: 11, E: 12})
	if err != nil {
		t.Fatal(err)
	}
	if string(cb) != string(wb) {
		t.Fatalf("msgpack bytes differ: codec=%q want=%q", cb, wb)
	}
}
