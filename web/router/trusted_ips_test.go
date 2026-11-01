package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/web/api"
	"github.com/pquerna/otp/totp"
)

func TestTrustedIPAuthenticationPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	flags.DatabaseFile = filepath.Join(t.TempDir(), "komari.db")
	db := dbcore.GetDBInstance()
	const secret = "JBSWY3DPEHPK3PXP"
	user := models.User{
		UUID: "trusted-ip-admin", Username: "trusted-ip-admin", TwoFactor: secret,
		TrustedIPs: models.StringArray{"192.0.2.1", "192.0.2.2", "192.0.2.3"},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for _, session := range []models.Session{
		{UUID: user.UUID, Session: "valid-session", Expires: time.Now().Add(time.Hour)},
		{UUID: user.UUID, Session: "expired-session", Expires: time.Now().Add(-time.Hour)},
	} {
		if err := db.Create(&session).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	r.Use(api.IdentityMiddleware())
	registerAdminRoutes(r)

	post := func(path, session, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if session != "" {
			req.AddCookie(&http.Cookie{Name: "session_token", Value: session})
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("POST %s session=%q: %d %s, want %d", path, session, res.Code, res.Body.String(), want)
		}
	}
	post("/api/admin/trusted-ips/remove", "", `{"ip":"192.0.2.1"}`, http.StatusUnauthorized)
	post("/api/admin/trusted-ips/remove", "expired-session", `{"ip":"192.0.2.1"}`, http.StatusUnauthorized)
	post("/api/admin/trusted-ips/add", "valid-session", `{"ip":"192.0.2.4"}`, http.StatusUnauthorized)
	post("/api/admin/trusted-ips/add", "valid-session", `{"ip":"192.0.2.4","2fa_code":"invalid"}`, http.StatusUnauthorized)
	post("/api/admin/trusted-ips/remove", "valid-session", `{"ip":"192.0.2.1"}`, http.StatusOK)
	if trusted, err := accounts.IsTrustedIP(user.UUID, "192.0.2.1"); err != nil || trusted {
		t.Fatalf("removed IP remains trusted: %v, %v", trusted, err)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	post("/api/admin/trusted-ips/add", "valid-session", `{"ip":"192.0.2.4","2fa_code":"`+code+`"}`, http.StatusOK)
	if trusted, err := accounts.IsTrustedIP(user.UUID, "192.0.2.4"); err != nil || !trusted {
		t.Fatalf("verified IP was not trusted: %v, %v", trusted, err)
	}
}
