package redis

import (
	"encoding/json"

	"github.com/boxgo/session"
	"github.com/vmihailenco/msgpack/v5"
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
	return json.Marshal(sess)
}

func (jsonCodec) Unmarshal(data []byte, sess *session.Session) error {
	return json.Unmarshal(data, sess)
}

type msgpackCodec struct{}

func (msgpackCodec) Name() string { return "msgpack" }

func (msgpackCodec) Marshal(sess *session.Session) ([]byte, error) {
	return msgpack.Marshal(sess)
}

func (msgpackCodec) Unmarshal(data []byte, sess *session.Session) error {
	return msgpack.Unmarshal(data, sess)
}

// JSONCodec 返回 JSON 编解码器。
func JSONCodec() Codec {
	return jsonCodec{}
}

// MsgpackCodec 返回 msgpack 编解码器。
func MsgpackCodec() Codec {
	return msgpackCodec{}
}
