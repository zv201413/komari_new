package public

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/utils/loginlimiter"
	"github.com/komari-monitor/komari/web/api"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	TwoFa       string `json:"2fa_code"`
	Remember    bool   `json:"remember"`
	RememberDays int   `json:"remember_days"`
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session_token",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   utils.GetScheme(c) == "https",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
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
	// 白名单豁免：clientIP 命中用户 IP 白名单时，跳过登录环节的动态码校验。
	// IP 取自 c.ClientIP()，Gin 已收窄 trustedProxies 至本机并强制读 X-Real-IP
	// （见 internal/server/runtime.go），客户端无法通过伪造 X-Forwarded-For 冒充白名单 IP。
	// 注意：本豁免只作用于登录，终端仍需经 /api/admin/sudo-auth 重新验证 2FA 换取 sudo_token。
	user, _ := accounts.GetUserByUUID(uuid)
	loginMethod := "password"
	if user.TwoFactor != "" { // 开启了2FA
		trusted, terr := accounts.IsTrustedIP(uuid, clientIP)
		if terr != nil {
			// 查询失败时不豁免，回退到强制 2FA，避免因数据库异常导致鉴权被削弱。
			trusted = false
		}
		if trusted {
			loginMethod = "password+trusted_ip"
		} else {
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
	}
	// 登录成功，清除失败计数。
	loginlimiter.Default.Reset(limiterKey)
	// 计算 session 持久时长：优先用前端传入的天数，默认 30 天
	rememberDays := data.RememberDays
	if rememberDays <= 0 {
		rememberDays = 30
	}
	sessionMaxAge := rememberDays * 86400
	// Create session
	session, err := accounts.CreateSession(uuid, sessionMaxAge, c.Request.UserAgent(), clientIP, loginMethod)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}
	// Cookie Max-Age：勾选"记住我"才持久化，否则浏览器会话级
	cookieMaxAge := 0
	if data.Remember {
		cookieMaxAge = sessionMaxAge
	}
	setSessionCookie(c, session, cookieMaxAge)
	auditlog.Log(clientIP, uuid, "logged in ("+loginMethod+")", "login")
	api.RespondSuccess(c, gin.H{"set-cookie": gin.H{"session_token": session}})
}
func Logout(c *gin.Context) {
	session, _ := c.Cookie("session_token")
	accounts.DeleteSession(session)
	setSessionCookie(c, "", -1)
	auditlog.Log(c.ClientIP(), "", "logged out", "logout")
	c.Redirect(302, "/")
}
