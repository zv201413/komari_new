package sudotoken

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNodeAndSessionIsolation(t *testing.T) {
	m := &Manager{tokens: make(map[string]*entry)}
	a := m.Create("user", "session", "node-a", time.Hour, false)
	b := m.Create("user", "session", "node-b", 24*time.Hour, false)
	for _, token := range []string{a, b} {
		for _, bad := range [][3]string{{"other", "session", "node-a"}, {"user", "other", "node-a"}, {"user", "session", ""}} {
			if m.Validate(token, bad[0], bad[1], bad[2], false) {
				t.Fatal("grant accepted outside its owner/session/node")
			}
		}
	}
	if m.Validate(a, "user", "session", "node-b", true) {
		t.Fatal("node A grant authorized node B")
	}
	if !m.Validate(a, "user", "session", "node-a", true) || !m.Validate(b, "user", "session", "node-b", true) {
		t.Fatal("independent node grants did not coexist")
	}
	m.mu.Lock()
	m.tokens[a].expiresAt = time.Now().Add(-time.Second)
	m.mu.Unlock()
	if m.Validate(a, "user", "session", "node-a", false) || !m.Validate(b, "user", "session", "node-b", false) {
		t.Fatal("expiration was not isolated to the target node")
	}
}

func TestSingleUseIsConsumedAtomicallyAfterPreflight(t *testing.T) {
	m := &Manager{tokens: make(map[string]*entry)}
	token := m.Create("user", "session", "node-a", time.Minute, true)
	for i := 0; i < 2; i++ {
		if !m.Validate(token, "user", "session", "node-a", false) {
			t.Fatal("preflight consumed single-use grant")
		}
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.Validate(token, "user", "session", "node-a", true) {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("single-use grant accepted %d times", accepted.Load())
	}
}
