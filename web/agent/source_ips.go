package agent

import (
	"crypto/sha256"
	"net/netip"
	"time"
)

type observedSource struct {
	ip           netip.Addr
	tokenHash    [32]byte
	connectionID int64
	expiresAt    time.Time
}

var sourceIPs = make(map[string]observedSource)

var nonPublicRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func publicAddress(raw string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(raw)
	if err != nil || ip.Zone() != "" {
		return netip.Addr{}, false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() {
		return netip.Addr{}, false
	}
	for _, prefix := range nonPublicRanges {
		if prefix.Contains(ip) {
			return netip.Addr{}, false
		}
	}
	return ip, true
}

// RecordSourceIP accepts only the server-observed address of an authenticated
// agent. A websocket source is tied to its exact connection; HTTP expires with
// presence. The credential fingerprint also invalidates trust on token rotation.
func RecordSourceIP(uuid, token, rawIP string, connectionID int64, ttl time.Duration) {
	ip, ok := publicAddress(rawIP)
	if uuid == "" || token == "" {
		return
	}
	source := observedSource{ip: ip, tokenHash: sha256.Sum256([]byte(token)), connectionID: connectionID}
	if connectionID == 0 {
		source.expiresAt = time.Now().Add(ttl)
	}
	mu.Lock()
	defer mu.Unlock()
	if connectionID != 0 {
		conn := connectedClients[uuid]
		if conn == nil || conn.ID != connectionID {
			return
		}
	}
	if !ok {
		delete(sourceIPs, uuid)
		return
	}
	sourceIPs[uuid] = source
}

// MatchesSourceIP never trusts an offline node, a stale HTTP source, or a new
// websocket that replaced the connection which established trust.
func MatchesSourceIP(uuid, currentToken, rawIP string) bool {
	ip, ok := publicAddress(rawIP)
	if !ok {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	source, ok := sourceIPs[uuid]
	if !ok || source.ip != ip || source.tokenHash != sha256.Sum256([]byte(currentToken)) {
		return false
	}
	if source.connectionID != 0 {
		conn := connectedClients[uuid]
		return conn != nil && conn.ID == source.connectionID
	}
	presence, ok := presenceOnly[uuid]
	now := time.Now()
	return ok && presence.expire.After(now) && source.expiresAt.After(now)
}
