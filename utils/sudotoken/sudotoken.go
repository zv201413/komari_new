// Package sudotoken 提供管理员终端的 2FA/Sudo 令牌管理（内存实现）。
//
// 设计：
//   - 管理员通过 /api/admin/sudo-auth 验证 2FA 后获取 Sudo Token（Cookie）。
//   - WebSocket 终端在 Upgrade 前检查 Cookie 中的 sudo_token 是否有效。
//   - Token 有 TTL，过期后自动清理。
//   - 单进程内存存储，重启清空（安全策略：重启后强制重新 2FA 验证）。
package sudotoken

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type entry struct {
	uuid      string
	expiresAt time.Time
}

// Manager 是并发安全的 Sudo Token 管理器。
type Manager struct {
	mu     sync.Mutex
	tokens map[string]*entry
}

// New 创建一个管理器并启动后台清理协程。
func New() *Manager {
	m := &Manager{
		tokens: make(map[string]*entry),
	}
	go m.cleanupLoop()
	return m
}

// Default 为进程级默认实例。
var Default = New()

// Create 为用户创建一个 Sudo Token，TTL 为 d，返回 32 字节十六进制 token 字符串。
func (m *Manager) Create(uuid string, d time.Duration) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = &entry{
		uuid:      uuid,
		expiresAt: time.Now().Add(d),
	}
	return token
}

// Validate 验证 token：有效返回 (uuid, true)，无效返回 ("", false)。
// 调用时会检查过期并即时清理过期条目。
func (m *Manager) Validate(token string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.tokens[token]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expiresAt) {
		delete(m.tokens, token)
		return "", false
	}
	return e.uuid, true
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.cleanup()
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for token, e := range m.tokens {
		if now.After(e.expiresAt) {
			delete(m.tokens, token)
		}
	}
}
