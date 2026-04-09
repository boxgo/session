package session

import "errors"

var (
	// ErrInvalidArgument 表示参数缺失或不合法。
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrSessionNotFound 表示会话不存在。
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionDeleted 表示会话已到删除时间，无法继续刷新。
	ErrSessionDeleted = errors.New("session deleted")
)
