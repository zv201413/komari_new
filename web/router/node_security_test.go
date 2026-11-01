package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils/sudotoken"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/api/public"
	"github.com/pquerna/otp/totp"
)

func TestNodeSecurityPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	flags.DatabaseFile = filepath.Join(t.TempDir(), "komari.db")
	db := dbcore.GetDBInstance()
	const secret = "JBSWY3DPEHPK3PXP"
	if _, err := accounts.CreateAccount("node-security-admin", "node-security-password"); err != nil {
		t.Fatal(err)
	}
	var user models.User
	if err := db.Where("username = ?", "node-security-admin").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	db.Model(&user).Update("two_factor", secret)
	for _, s := range []string{"node-security-session", "node-security-other-session"} {
		if err := db.Create(&models.Session{UUID: user.UUID, Session: s, Expires: time.Now().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, node := range []models.Client{{UUID: "security-node-a", Token: "security-token-a", Name: "A"}, {UUID: "security-node-b", Token: "security-token-b", Name: "B"}} {
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := config.Set(config.Sudo2FaRequiredKey, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		config.Set(config.Sudo2FaRequiredKey, false)
		config.Set(config.NodeIPTrustEnabledKey, false)
		db.Where("uuid = ?", user.UUID).Delete(&models.Session{})
		accounts.DeleteAccountByUsername(user.Username)
		clients.DeleteClient("security-node-a")
		clients.DeleteClient("security-node-b")
		agent_runtime.DeleteConnectedClients("security-node-a")
	})
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(api.IdentityMiddleware())
	registerAdminRoutes(r)
	r.POST("/api/login", public.Login)
	call := func(method, path, session, body, code string, cookie *http.Cookie, upgrade bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.RemoteAddr = "8.8.8.8:12345"
		req.Header.Set("Content-Type", "application/json")
		if code != "" {
			req.Header.Set("X-2FA-Code", code)
		}
		if session != "" {
			req.AddCookie(&http.Cookie{Name: "session_token", Value: session})
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if upgrade {
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	want := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body.String())
		}
	}
	const path = "/api/admin/client/security-node-a/trust-login-ip"
	want(call("POST", path, "", `{"enabled":true}`, "", nil, false), 401)
	want(call("POST", path, "node-security-session", `{"enabled":true}`, "", nil, false), 401)
	want(call("POST", path, "node-security-session", `{"enabled":true}`, "invalid", nil, false), 401)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	const masterPath = "/api/admin/client/ip-trust-policy"
	want(call("POST", masterPath, "node-security-session", `{"enabled":true}`, "", nil, false), 401)
	want(call("POST", masterPath, "node-security-session", `{"enabled":true}`, "invalid", nil, false), 401)
	master := call("POST", masterPath, "node-security-session", `{"enabled":true}`, code, nil, false)
	want(master, 200)
	var masterGrant *http.Cookie
	for _, cookie := range master.Result().Cookies() {
		if cookie.Name == "komari_ip_trust_admin" {
			masterGrant = cookie
		}
	}
	if masterGrant == nil {
		t.Fatal("verified master did not issue an editing grant")
	}
	want(call("POST", path, "node-security-session", `{"enabled":true}`, "", masterGrant, false), 200)
	a, _ := clients.GetClientByUUID("security-node-a")
	b, _ := clients.GetClientByUUID("security-node-b")
	if !a.TrustLoginIP || b.TrustLoginIP {
		t.Fatal("trust did not stay isolated to selected node")
	}
	const nodeBPath = "/api/admin/client/security-node-b/trust-login-ip"
	want(call("POST", nodeBPath, "node-security-session", `{"enabled":true}`, "", masterGrant, false), 200)
	want(call("POST", nodeBPath, "node-security-session", `{"enabled":false}`, "", nil, false), 200)
	want(call("POST", "/api/admin/settings/", "node-security-session", `{"node_ip_trust_enabled":true}`, "", nil, false), 401)
	// Neither ordinary edits nor agent metadata can bypass the security endpoint.
	want(call("POST", "/api/admin/client/security-node-b/edit", "node-security-session", `{"trust_login_ip":true}`, "", nil, false), 200)
	if err := clients.SaveClientInfo(map[string]interface{}{"uuid": "security-node-b", "trust_login_ip": true}); err != nil {
		t.Fatal(err)
	}
	b, _ = clients.GetClientByUUID("security-node-b")
	if b.TrustLoginIP {
		t.Fatal("unverified trust mutation accepted")
	}

	agent_runtime.KeepAlivePresence(a.UUID, 1, time.Minute)
	agent_runtime.RecordSourceIP(a.UUID, a.Token, "8.8.8.8", 0, time.Minute)
	login, _ := json.Marshal(public.LoginRequest{Username: user.Username, Password: "node-security-password"})
	want(call("POST", "/api/login", "", string(login), "", nil, false), 200)
	wrongPassword, _ := json.Marshal(public.LoginRequest{Username: user.Username, Password: "wrong"})
	want(call("POST", "/api/login", "", string(wrongPassword), "", nil, false), 401)
	// Closing the valve suspends all exemptions and preserves node choices.
	want(call("POST", masterPath, "node-security-session", `{"enabled":false}`, "", masterGrant, false), 401)
	want(call("POST", masterPath, "node-security-session", `{"enabled":false}`, code, masterGrant, false), 200)
	a, _ = clients.GetClientByUUID("security-node-a")
	if !a.TrustLoginIP {
		t.Fatal("closing master erased node choice")
	}
	want(call("POST", "/api/login", "", string(login), "", nil, false), 401)
	want(call("POST", nodeBPath, "node-security-session", `{"enabled":true}`, "", masterGrant, false), 401)
	// Revocation remains available with the valve closed and without a grant.
	want(call("POST", path, "node-security-session", `{"enabled":false}`, "", nil, false), 200)
	want(call("POST", "/api/login", "", string(login), "", nil, false), 401)
	master = call("POST", masterPath, "node-security-session", `{"enabled":true}`, code, nil, false)
	want(master, 200)
	want(call("POST", path, "node-security-session", `{"enabled":true}`, "", masterGrant, false), 401)
	for _, cookie := range master.Result().Cookies() {
		if cookie.Name == "komari_ip_trust_admin" {
			masterGrant = cookie
		}
	}
	want(call("POST", path, "node-security-other-session", `{"enabled":true}`, "", masterGrant, false), 401)
	// The manually reported display IP is never sufficient to grant trust.
	want(call("POST", path, "node-security-session", `{"enabled":true}`, "", masterGrant, false), 200)
	agent_runtime.SetPresence(a.UUID, 1, false)
	want(call("POST", "/api/login", "", string(login), "", nil, false), 401)

	want(call("POST", "/api/admin/sudo-auth", "node-security-session", `{"2fa_code":"`+code+`","duration":"1h"}`, "", nil, false), 400)
	auth := call("POST", "/api/admin/sudo-auth", "node-security-session", `{"uuid":"security-node-a","2fa_code":"`+code+`","duration":"1h"}`, "", nil, false)
	want(auth, 200)
	var grant *http.Cookie
	for _, cookie := range auth.Result().Cookies() {
		if cookie.Name == sudotoken.CookieName {
			grant = cookie
		}
	}
	if grant == nil || grant.Path != sudotoken.CookiePath(a.UUID) {
		t.Fatal("grant cookie was not scoped to node A")
	}
	want(call("GET", "/api/admin/client/security-node-a/sudo-check", "node-security-session", "", "", grant, false), 200)
	want(call("GET", "/api/admin/client/security-node-b/sudo-check", "node-security-session", "", "", grant, false), 401)
	want(call("GET", "/api/admin/client/security-node-a/sudo-check", "node-security-other-session", "", "", grant, false), 401)
	want(call("GET", "/api/admin/client/security-node-b/terminal", "node-security-session", "", "", grant, true), 401)
	legacy := &http.Cookie{Name: "sudo_token", Value: grant.Value}
	want(call("GET", "/api/admin/client/security-node-a/sudo-check", "node-security-session", "", "", legacy, false), 401)
}
