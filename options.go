package session

import "time"

// Option 用于配置 Manager 的行为。
type Option func(*Manager)

// WithMode 设置会话模式（单会话/多会话）。
func WithMode(mode SessionMode) Option {
	return func(m *Manager) {
		m.mode = mode
	}
}

// WithNowFunc 注入自定义时钟函数（常用于测试）。
func WithNowFunc(nowFn func() time.Time) Option {
	return func(m *Manager) {
		if nowFn != nil {
			m.nowFn = nowFn
		}
	}
}

// WithEventEnabled 开启或关闭会话事件发布/订阅能力。
func WithEventEnabled(enabled bool) Option {
	return func(m *Manager) {
		m.eventEnabled = enabled
	}
}
