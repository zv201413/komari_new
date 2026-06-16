// Package loginlimiter 提供登录失败限速与阶梯锁定（内存实现）。
//
// 维度为 "客户端IP|用户名" 组合键：
//   - 纯 IP 会误伤同出口 NAT（公司/宿舍）下的多用户；
//   - 纯用户名会被远端字典攻击锁死真实管理员；
//   - 组合键同时兜住这两种场景。
//
// 当前为进程内内存实现，重启后计数清空（对防爆破影响有限，攻击窗口以分钟计）。
// 后续可在不改调用方的前提下替换为持久化实现（仅需实现 Allow/Fail/Reset）。
package loginlimiter

import (
	"strings"
	"sync"
	"time"
)

// Tier 表示一个阶梯：累计失败达到 Threshold 次时，锁定 Lockout 时长。
type Tier struct {
	Threshold int
	Lockout   time.Duration
}

// Config 限速策略，可按需调整阈值。
type Config struct {
	// Tiers 阶梯锁定策略，必须按 Threshold 升序排列。
	// 命中多个阶梯时取最高阶梯（最长锁定）。
	Tiers []Tier
	// RecordTTL 记录存活时间：未锁定且最后活动超过此时长的记录会被清理。
	// 应不小于最大锁定时长，避免清掉仍在锁定中的记录。
	RecordTTL time.Duration
	// CleanupInterval 后台清理间隔。
	CleanupInterval time.Duration
}

// DefaultConfig 管理员面板默认策略：5次→5min，10次→30min，15次→2h。
var DefaultConfig = Config{
	Tiers: []Tier{
		{Threshold: 5, Lockout: 5 * time.Minute},
		{Threshold: 10, Lockout: 30 * time.Minute},
		{Threshold: 15, Lockout: 2 * time.Hour},
	},
	RecordTTL:       2 * time.Hour,
	CleanupInterval: 10 * time.Minute,
}

type record struct {
	fails       int
	lockedUntil time.Time
	lastSeen    time.Time
}

// Limiter 是并发安全的登录失败限速器。
type Limiter struct {
	mu      sync.Mutex
	records map[string]*record
	cfg     Config
}

// New 创建一个限速器并启动后台清理协程。
func New(cfg Config) *Limiter {
	l := &Limiter{
		records: make(map[string]*record),
		cfg:     cfg,
	}
	go l.cleanupLoop()
	return l
}

// Default 为进程级默认限速器实例。
var Default = New(DefaultConfig)

// Key 构造组合键。用户名统一小写，防止大小写变体绕过计数。
func Key(ip, username string) string {
	return ip + "|" + strings.ToLower(username)
}

// Allow 在尝试登录前调用。若该键正处于锁定期，返回 (true, 剩余时长)。
func (l *Limiter) Allow(key string) (blocked bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.records[key]
	if !ok {
		return false, 0
	}
	now := time.Now()
	if now.Before(r.lockedUntil) {
		return true, r.lockedUntil.Sub(now)
	}
	return false, 0
}

// Fail 在一次登录失败（密码或 2FA）后调用，累加计数并按阶梯设定锁定。
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	r, ok := l.records[key]
	if !ok {
		r = &record{}
		l.records[key] = r
	}
	r.fails++
	r.lastSeen = now
	// Tiers 升序，命中的最高阶梯给出最长锁定。
	for _, t := range l.cfg.Tiers {
		if r.fails >= t.Threshold {
			r.lockedUntil = now.Add(t.Lockout)
		}
	}
}

// Reset 在登录成功后调用，清除该键的失败记录。
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, key)
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(l.cfg.CleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup()
	}
}

func (l *Limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, r := range l.records {
		if now.After(r.lockedUntil) && now.Sub(r.lastSeen) > l.cfg.RecordTTL {
			delete(l.records, k)
		}
	}
}
