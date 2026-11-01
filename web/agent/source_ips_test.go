package agent

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/web/connection"
)

func TestObservedSourceTrustLifecycle(t *testing.T) {
	const node = "source-trust-test"
	defer DeleteConnectedClients(node)
	KeepAlivePresence(node, 1, time.Minute)
	RecordSourceIP(node, "credential", "8.8.8.8", 0, time.Minute)
	if !MatchesSourceIP(node, "credential", "::ffff:8.8.8.8") {
		t.Fatal("mapped IPv4 did not match observed address")
	}
	if MatchesSourceIP(node, "rotated", "8.8.8.8") || MatchesSourceIP(node, "credential", "8.8.4.4") {
		t.Fatal("wrong credential or source accepted")
	}
	RecordSourceIP(node, "credential", "8.8.4.4", 0, time.Minute)
	if MatchesSourceIP(node, "credential", "8.8.8.8") {
		t.Fatal("old source stayed trusted after IP change")
	}
	RecordSourceIP(node, "credential", "8.8.4.4", 0, -time.Second)
	if MatchesSourceIP(node, "credential", "8.8.4.4") {
		t.Fatal("expired HTTP source accepted")
	}
	RecordSourceIP(node, "credential", "8.8.4.4", 0, time.Minute)
	RecordSourceIP(node, "credential", "10.0.0.1", 0, time.Minute)
	if MatchesSourceIP(node, "credential", "8.8.4.4") {
		t.Fatal("private source left the old public source trusted")
	}
	RecordSourceIP(node, "credential", "8.8.4.4", 0, time.Minute)
	SetPresence(node, 1, false)
	if MatchesSourceIP(node, "credential", "8.8.4.4") {
		t.Fatal("offline source accepted")
	}

	old := &connection.SafeConn{ID: 1}
	current := &connection.SafeConn{ID: 2}
	SetConnectedClients(node, old)
	RecordSourceIP(node, "credential", "2001:4860:4860::8888", old.ID, 0)
	if !MatchesSourceIP(node, "credential", "2001:4860:4860:0:0:0:0:8888") {
		t.Fatal("equivalent IPv6 did not match")
	}
	SetConnectedClients(node, current)
	if MatchesSourceIP(node, "credential", "2001:4860:4860::8888") {
		t.Fatal("replaced connection stayed trusted")
	}
	RecordSourceIP(node, "credential", "8.8.8.8", current.ID, 0)
	RecordSourceIP(node, "credential", "8.8.4.4", old.ID, 0)
	DeleteClientConditionally(node, old)
	if !MatchesSourceIP(node, "credential", "8.8.8.8") {
		t.Fatal("old connection invalidated current source")
	}
	DeleteClientConditionally(node, current)
	if MatchesSourceIP(node, "credential", "8.8.8.8") {
		t.Fatal("disconnected websocket stayed trusted")
	}
}

func TestNonPublicSourcesNeverTrusted(t *testing.T) {
	for _, ip := range []string{"", "invalid", "127.0.0.1", "10.0.0.1", "172.16.1.1", "192.168.1.1", "169.254.1.1", "100.64.1.1", "192.0.2.1", "198.18.0.1", "203.0.113.1", "0.0.0.0", "255.255.255.255", "224.0.0.1", "::1", "::", "fc00::1", "fe80::1", "2001:db8::1", "fe80::1%eth0"} {
		if _, ok := publicAddress(ip); ok {
			t.Fatalf("nonpublic IP accepted: %s", ip)
		}
	}
}
