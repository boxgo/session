package redis

import (
	"github.com/boxgo/session"
	"github.com/bytedance/sonic"
)

// Codec 定义 Redis 会话对象的编解码接口。
type Codec interface {
	Name() string
	Marshal(sess *session.Session) ([]byte, error)
	Unmarshal(data []byte, sess *session.Session) error
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(sess *session.Session) ([]byte, error) {
	return marshalSessionJSON(sess)
}

func (jsonCodec) Unmarshal(data []byte, sess *session.Session) error {
	return unmarshalSessionJSON(data, sess)
}

type msgpackCodec struct{}

func (msgpackCodec) Name() string { return "msgpack" }

func (msgpackCodec) Marshal(sess *session.Session) ([]byte, error) {
	return marshalSessionMsgpack(sess)
}

func (msgpackCodec) Unmarshal(data []byte, sess *session.Session) error {
	return unmarshalSessionMsgpack(data, sess)
}

type sonicCodec struct{}

func (sonicCodec) Name() string { return "sonic" }

func (sonicCodec) Marshal(sess *session.Session) ([]byte, error) {
	return sonic.Marshal(sessionToWire(sess))
}

func (sonicCodec) Unmarshal(data []byte, sess *session.Session) error {
	var m map[string]interface{}
	if err := sonic.Unmarshal(data, &m); err != nil {
		return err
	}
	return applyLooseSessionMap(sess, m)
}

// JSONCodec 返回 JSON 编解码器。
func JSONCodec() Codec {
	return jsonCodec{}
}

// MsgpackCodec 返回 msgpack 编解码器。
func MsgpackCodec() Codec {
	return msgpackCodec{}
}

// SonicCodec 返回基于 github.com/bytedance/sonic 的 JSON 编解码器，与 JSONCodec 使用相同的紧凑线格式（短键 + Unix 秒），通常更快；需满足 sonic 对 Go 版本与 CPU 架构的要求。
func SonicCodec() Codec {
	return sonicCodec{}
}
