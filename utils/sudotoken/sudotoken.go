// Package sudotoken 提供管理员终端的 2FA/Sudo 令牌管理（内存实现）。
//
// 设计：
//   - 管理员通过 /api/admin/sudo-auth 验证 2FA 后获取 Sudo Token（Cookie）。
//   - 每台服务器使用独立路径的 Cookie，令牌绑定用户、登录会话和目标节点。
//   - WebSocket 终端在 Upgrade 前校验令牌；单次令牌仅在连接时消耗。
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
	session   string
	node      string
	singleUse bool
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
func (m *Manager) Create(uuid, session, node string, d time.Duration, singleUse bool) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = &entry{
		uuid:      uuid,
		session:   session,
		node:      node,
		singleUse: singleUse,
		expiresAt: time.Now().Add(d),
	}
	return token
}

// Validate checks the owner, login session, target node and expiry. Preflight
// uses consume=false; terminal authorization atomically consumes single-use grants.
func (m *Manager) Validate(token, uuid, session, node string, consume bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.tokens[token]
	if !ok {
		return false
	}
	if time.Now().After(e.expiresAt) {
		delete(m.tokens, token)
		return false
	}
	if uuid == "" || session == "" || node == "" || e.uuid != uuid || e.session != session || e.node != node {
		return false
	}
	if consume && e.singleUse {
		delete(m.tokens, token)
	}
	return true
}

const CookieName = "komari_node_sudo"

func CookiePath(node string) string { return "/api/admin/client/" + node + "/" }

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
