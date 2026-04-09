package redis

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boxgo/session"
	goredis "github.com/redis/go-redis/v9"
)

// RedisStore 是基于 Redis 的 session.Store 实现。
type RedisStore struct {
	client goredis.UniversalClient
	// prefix 作为命名空间前缀，避免不同模块键冲突。
	prefix string
	codec  Codec
}

const deletedMemberSep = "\x1f"

// NewRedisStore 使用指定前缀创建 RedisStore。
func NewRedisStore(client goredis.UniversalClient, prefix string) *RedisStore {
	return NewRedisStoreWithCodec(client, prefix, nil)
}

// NewRedisStoreWithCodec 使用指定前缀和编解码器创建 RedisStore。
// codec 为空时，默认使用 JSONCodec。
func NewRedisStoreWithCodec(client goredis.UniversalClient, prefix string, codec Codec) *RedisStore {
	if prefix == "" {
		prefix = "session"
	}
	if codec == nil {
		codec = JSONCodec()
	}

	return &RedisStore{
		client: client,
		prefix: prefix,
		codec:  codec,
	}
}

// Upsert 新增或更新会话，并同步维护二级索引。
func (s *RedisStore) Upsert(ctx context.Context, sess *session.Session) error {
	if sess == nil || sess.ID == "" || sess.UserID == "" {
		return session.ErrInvalidArgument
	}

	payload, err := s.codec.Marshal(sess)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	if sess.DeletedAt != nil {
		pipe.SetArgs(ctx, s.sessionKey(sess.ID), payload, goredis.SetArgs{ExpireAt: *sess.DeletedAt})
	} else {
		pipe.Set(ctx, s.sessionKey(sess.ID), payload, 0)
	}
	pipe.ZAdd(ctx, s.userKey(sess.UserID), goredis.Z{
		Score:  float64(sess.UpdatedAt.Unix()),
		Member: sess.ID,
	})
	if sess.DeletedAt != nil {
		pipe.ZAdd(ctx, s.deletedKey(), goredis.Z{
			Score:  float64(sess.DeletedAt.Unix()),
			Member: s.deletedMember(sess.UserID, sess.ID),
		})
	} else {
		pipe.ZRem(ctx, s.deletedKey(), s.deletedMember(sess.UserID, sess.ID))
	}
	_, err = pipe.Exec(ctx)

	return err
}

// Get 按 ID 查询单个会话。
func (s *RedisStore) Get(ctx context.Context, sessionID string) (*session.Session, error) {
	if sessionID == "" {
		return nil, session.ErrInvalidArgument
	}

	raw, err := s.client.Get(ctx, s.sessionKey(sessionID)).Bytes()
	if err == goredis.Nil {
		return nil, session.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	return s.decodeSession(raw)
}

// ListUsers 查询当前 Redis 索引中的全部用户ID。
func (s *RedisStore) ListUsers(ctx context.Context) ([]string, error) {
	pattern := s.prefix + ":user:*"
	users := make(map[string]struct{}, 256)

	collectUsers := func(keys []string) {
		for _, key := range keys {
			userID := strings.TrimPrefix(key, s.prefix+":user:")
			if userID != "" {
				users[userID] = struct{}{}
			}
		}
	}

	switch cli := s.client.(type) {
	case *goredis.ClusterClient:
		var mu sync.Mutex
		err := cli.ForEachMaster(ctx, func(ctx context.Context, master *goredis.Client) error {
			var cursor uint64
			for {
				keys, next, err := master.Scan(ctx, cursor, pattern, 256).Result()
				if err != nil {
					return err
				}
				mu.Lock()
				collectUsers(keys)
				mu.Unlock()
				cursor = next
				if cursor == 0 {
					break
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	default:
		var cursor uint64
		for {
			keys, next, err := s.client.Scan(ctx, cursor, pattern, 256).Result()
			if err != nil {
				return nil, err
			}
			collectUsers(keys)
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}

	ans := make([]string, 0, len(users))
	for userID := range users {
		ans = append(ans, userID)
	}
	sort.Strings(ans)

	return ans, nil
}

// ListByUser 按更新时间倒序查询某用户全部会话。
func (s *RedisStore) ListByUser(ctx context.Context, userID string) ([]*session.Session, error) {
	if userID == "" {
		return nil, session.ErrInvalidArgument
	}

	sessionIDs, err := s.client.ZRevRange(ctx, s.userKey(userID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	pipe := s.client.Pipeline()
	cmds := make([]*goredis.StringCmd, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		cmds = append(cmds, pipe.Get(ctx, s.sessionKey(sessionID)))
	}
	_, err = pipe.Exec(ctx)
	if err != nil && err != goredis.Nil {
		return nil, err
	}

	ans := make([]*session.Session, 0, len(cmds))
	for _, cmd := range cmds {
		raw, e := cmd.Bytes()
		if e != nil {
			continue
		}
		var rec session.Session
		if e := s.codec.Unmarshal(raw, &rec); e != nil {
			continue
		}
		cp := rec
		ans = append(ans, &cp)
	}

	return ans, nil
}

// Delete 将单个会话标记为删除态。
func (s *RedisStore) Delete(ctx context.Context, sessionID string, deletedAt time.Time) error {
	sess, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	sess.DeletedAt = &deletedAt
	sess.UpdatedAt = deletedAt

	return s.Upsert(ctx, sess)
}

// DeleteByUser 将某个用户下全部会话标记为删除态。
func (s *RedisStore) DeleteByUser(ctx context.Context, userID string, deletedAt time.Time) ([]string, error) {
	sessions, err := s.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	ans := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		sess.DeletedAt = &deletedAt
		sess.UpdatedAt = deletedAt
		if err := s.Upsert(ctx, sess); err != nil {
			return nil, err
		}
		ans = append(ans, sess.ID)
	}

	return ans, nil
}

// Purge 物理删除达到删除时间的会话。
func (s *RedisStore) Purge(ctx context.Context, now time.Time) ([]string, error) {
	members, err := s.client.ZRangeByScore(ctx, s.deletedKey(), &goredis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(now.Unix(), 10),
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}

	userToIDs := make(map[string][]string, 64)
	sessionIDs := make([]string, 0, len(members))
	for _, member := range members {
		userID, sessionID, ok := s.parseDeletedMember(member)
		if ok {
			sessionIDs = append(sessionIDs, sessionID)
			userToIDs[userID] = append(userToIDs[userID], sessionID)
		}
	}

	pipeDel := s.client.Pipeline()
	for _, sessionID := range sessionIDs {
		pipeDel.Del(ctx, s.sessionKey(sessionID))
	}
	pipeDel.ZRem(ctx, s.deletedKey(), toInterfaces(members)...)
	for userID, userSessionIDs := range userToIDs {
		pipeDel.ZRem(ctx, s.userKey(userID), toInterfaces(userSessionIDs)...)
	}
	_, err = pipeDel.Exec(ctx)

	return sessionIDs, err
}

func (s *RedisStore) sessionKey(sessionID string) string {
	return s.prefix + ":session:" + sessionID
}

func (s *RedisStore) userKey(userID string) string {
	return s.prefix + ":user:" + userID
}

func (s *RedisStore) deletedKey() string {
	return s.prefix + ":deleted"
}

func (s *RedisStore) decodeSession(raw []byte) (*session.Session, error) {
	var ans session.Session
	if err := s.codec.Unmarshal(raw, &ans); err != nil {
		return nil, err
	}

	return &ans, nil
}

func (s *RedisStore) deletedMember(userID, sessionID string) string {
	return userID + deletedMemberSep + sessionID
}

func (s *RedisStore) parseDeletedMember(member string) (userID, sessionID string, ok bool) {
	idx := strings.Index(member, deletedMemberSep)
	if idx <= 0 || idx >= len(member)-1 {
		return "", "", false
	}

	return member[:idx], member[idx+len(deletedMemberSep):], true
}

func toInterfaces(src []string) []interface{} {
	ans := make([]interface{}, 0, len(src))
	for _, item := range src {
		ans = append(ans, item)
	}

	return ans
}
