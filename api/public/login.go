package public

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/komari-monitor/komari/api"
	"github.com/komari-monitor/komari/config"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/utils/loginlimiter"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TwoFa    string `json:"2fa_code"`
	Remember bool   `json:"remember"`
}

func Login(c *gin.Context) {
	DisablePasswordLogin, _ := config.GetAs[bool](config.DisablePasswordLoginKey, false)
	if DisablePasswordLogin {
		api.RespondError(c, http.StatusForbidden, "Password login is disabled")
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	var data LoginRequest
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if data.Username == "" || data.Password == "" {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: Username and password are required")
		return
	}

	// 登录失败限速：组合键 IP|用户名，锁定期内直接拒绝。
	clientIP := c.ClientIP()
	limiterKey := loginlimiter.Key(clientIP, data.Username)
	if blocked, retryAfter := loginlimiter.Default.Allow(limiterKey); blocked {
		auditlog.Log(clientIP, "", "login blocked (rate limit)", "login")
		c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		api.RespondError(c, http.StatusTooManyRequests, "Too many failed login attempts. Please try again later.")
		return
	}

	uuid, success := accounts.CheckPassword(data.Username, data.Password)
	if !success {
		loginlimiter.Default.Fail(limiterKey)
		api.RespondError(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	// 2FA
	user, _ := accounts.GetUserByUUID(uuid)
	if user.TwoFactor != "" { // 开启了2FA
		if data.TwoFa == "" {
			api.RespondError(c, http.StatusUnauthorized, "2FA code is required")
			return
		}
		if ok, err := accounts.Verify2Fa(uuid, data.TwoFa); err != nil || !ok {
			loginlimiter.Default.Fail(limiterKey)
			api.RespondError(c, http.StatusUnauthorized, "Invalid 2FA code")
			return
		}
	}
	// 登录成功，清除失败计数。
	loginlimiter.Default.Reset(limiterKey)
	// Create session (Server side session persists for 30 days)
	session, err := accounts.CreateSession(uuid, 2592000, c.Request.UserAgent(), clientIP, "password")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}
	
	// Determine Cookie Max-Age
	cookieMaxAge := 0
	if data.Remember {
		cookieMaxAge = 2592000
	}
	
	c.SetCookie("session_token", session, cookieMaxAge, "/", "", false, true)
	auditlog.Log(c.ClientIP(), uuid, "logged in (password)", "login")
	api.RespondSuccess(c, gin.H{"set-cookie": gin.H{"session_token": session}})
}
func Logout(c *gin.Context) {
	session, _ := c.Cookie("session_token")
	accounts.DeleteSession(session)
	c.SetCookie("session_token", "", -1, "/", "", false, true)
	auditlog.Log(c.ClientIP(), "", "logged out", "logout")
	c.Redirect(302, "/")
}
