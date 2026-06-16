package loginlimiter

import (
	"testing"
	"time"
)

func newTestLimiter() *Limiter {
	// 不启动后台清理协程，避免测试泄漏；直接构造。
	return &Limiter{
		records: make(map[string]*record),
		cfg: Config{
			Tiers: []Tier{
				{Threshold: 3, Lockout: 50 * time.Millisecond},
				{Threshold: 5, Lockout: 500 * time.Millisecond},
			},
			RecordTTL:       time.Hour,
			CleanupInterval: time.Hour,
		},
	}
}

func TestLockoutAfterThreshold(t *testing.T) {
	l := newTestLimiter()
	k := Key("1.2.3.4", "admin")

	for i := 0; i < 2; i++ {
		l.Fail(k)
		if blocked, _ := l.Allow(k); blocked {
			t.Fatalf("不应在第 %d 次失败后锁定", i+1)
		}
	}
	// 第 3 次失败达到第一阶梯
	l.Fail(k)
	blocked, retry := l.Allow(k)
	if !blocked {
		t.Fatal("达到阈值后应被锁定")
	}
	if retry <= 0 || retry > 50*time.Millisecond {
		t.Fatalf("retryAfter 异常: %v", retry)
	}
}

func TestUnlockAfterExpiry(t *testing.T) {
	l := newTestLimiter()
	k := Key("1.2.3.4", "admin")
	for i := 0; i < 3; i++ {
		l.Fail(k)
	}
	if blocked, _ := l.Allow(k); !blocked {
		t.Fatal("应处于锁定")
	}
	time.Sleep(70 * time.Millisecond)
	if blocked, _ := l.Allow(k); blocked {
		t.Fatal("锁定到期后应放行")
	}
}

func TestResetOnSuccess(t *testing.T) {
	l := newTestLimiter()
	k := Key("1.2.3.4", "admin")
	for i := 0; i < 3; i++ {
		l.Fail(k)
	}
	l.Reset(k)
	if blocked, _ := l.Allow(k); blocked {
		t.Fatal("Reset 后不应锁定")
	}
	if _, ok := l.records[k]; ok {
		t.Fatal("Reset 后记录应被删除")
	}
}

func TestStaircaseEscalation(t *testing.T) {
	l := newTestLimiter()
	k := Key("1.2.3.4", "admin")
	for i := 0; i < 5; i++ {
		l.Fail(k)
	}
	_, retry := l.Allow(k)
	// 命中第二阶梯（5次→500ms），应明显长于第一阶梯
	if retry <= 50*time.Millisecond {
		t.Fatalf("应升级到更长锁定，实际 retry=%v", retry)
	}
}

func TestCompositeKeyIsolation(t *testing.T) {
	l := newTestLimiter()
	kA := Key("1.1.1.1", "admin")
	kB := Key("2.2.2.2", "admin") // 同用户名不同 IP
	for i := 0; i < 3; i++ {
		l.Fail(kA)
	}
	if blocked, _ := l.Allow(kB); blocked {
		t.Fatal("不同 IP 的同名用户不应被牵连锁定")
	}
}

func TestKeyCaseInsensitiveUsername(t *testing.T) {
	if Key("1.1.1.1", "Admin") != Key("1.1.1.1", "admin") {
		t.Fatal("用户名应大小写归一，防止变体绕过")
	}
}
