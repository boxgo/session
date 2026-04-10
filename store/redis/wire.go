package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/boxgo/session"
	"github.com/vmihailenco/msgpack/v5"
)

// sessionWire 为 Redis 中 Session 的紧凑形态：短键 + 时间为 UTC Unix 秒（纳秒在编解码时丢弃）。
type sessionWire struct {
	I string            `json:"i" msgpack:"i"`
	U string            `json:"u" msgpack:"u"`
	P map[string]string `json:"p,omitempty" msgpack:"p,omitempty"`
	C int64             `json:"c" msgpack:"c"`
	M int64             `json:"m" msgpack:"m"`
	E int64             `json:"e" msgpack:"e"`
	D *int64            `json:"d,omitempty" msgpack:"d,omitempty"`
}

func sessionToWire(s *session.Session) sessionWire {
	if s == nil {
		return sessionWire{}
	}
	w := sessionWire{
		I: s.ID,
		U: s.UserID,
		P: s.Payload,
		C: s.CreatedAt.Unix(),
		M: s.UpdatedAt.Unix(),
		E: s.ExpiresAt.Unix(),
	}
	if !s.DeletedAt.IsZero() {
		d := s.DeletedAt.Unix()
		w.D = &d
	}
	return w
}

func wireApplyToSession(s *session.Session, w sessionWire) {
	s.ID = w.I
	s.UserID = w.U
	s.Payload = w.P
	s.CreatedAt = time.Unix(w.C, 0).UTC()
	s.UpdatedAt = time.Unix(w.M, 0).UTC()
	s.ExpiresAt = time.Unix(w.E, 0).UTC()
	if w.D != nil {
		s.DeletedAt = time.Unix(*w.D, 0).UTC()
	} else {
		s.DeletedAt = time.Time{}
	}
}

func marshalSessionJSON(s *session.Session) ([]byte, error) {
	return json.Marshal(sessionToWire(s))
}

func unmarshalSessionJSON(data []byte, s *session.Session) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	return applyLooseSessionMap(s, m)
}

func marshalSessionMsgpack(s *session.Session) ([]byte, error) {
	return msgpack.Marshal(sessionToWire(s))
}

func unmarshalSessionMsgpack(data []byte, s *session.Session) error {
	var w sessionWire
	if err := msgpack.Unmarshal(data, &w); err != nil {
		return err
	}
	wireApplyToSession(s, w)
	return nil
}

func applyLooseSessionMap(s *session.Session, m map[string]interface{}) error {
	s.ID = pickStr(m, "i", "id")
	s.UserID = pickStr(m, "u", "userId")
	s.Payload = pickPayload(m)

	c, ok := pickTime(m, "c", "createdAt")
	if !ok {
		return fmt.Errorf("redis: session missing time field c/createdAt")
	}
	mu, ok := pickTime(m, "m", "updatedAt")
	if !ok {
		return fmt.Errorf("redis: session missing time field m/updatedAt")
	}
	e, ok := pickTime(m, "e", "expiresAt")
	if !ok {
		return fmt.Errorf("redis: session missing time field e/expiresAt")
	}
	s.CreatedAt = c
	s.UpdatedAt = mu
	s.ExpiresAt = e

	if t, ok := pickTime(m, "d", "deletedAt"); ok {
		s.DeletedAt = t
	} else {
		s.DeletedAt = time.Time{}
	}
	return nil
}

func pickStr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if str, ok := v.(string); ok {
				return str
			}
		}
	}
	return ""
}

func pickPayload(m map[string]interface{}) map[string]string {
	for _, key := range []string{"p", "payload"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case map[string]interface{}:
			out := make(map[string]string, len(v))
			for k, val := range v {
				if str, ok := val.(string); ok {
					out[k] = str
				} else if val != nil {
					out[k] = fmt.Sprint(val)
				}
			}
			return out
		case map[string]string:
			return v
		}
	}
	return nil
}

func pickTime(m map[string]interface{}, keys ...string) (time.Time, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return time.Unix(int64(x), 0).UTC(), true
		case string:
			t, err := time.Parse(time.RFC3339Nano, x)
			if err != nil {
				t, err = time.Parse(time.RFC3339, x)
			}
			if err == nil {
				return t.UTC(), true
			}
		}
	}
	return time.Time{}, false
}
