package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/api"
	"github.com/komari-monitor/komari/config"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/utils/sudotoken"
)

type sudoAuthRequest struct {
	TwoFaCode string `json:"2fa_code"`
	Duration  string `json:"duration"` // "1h" | "24h" | "always"
}

func parseSudoDuration(d string) time.Duration {
	switch d {
	case "1h":
		return 1 * time.Hour
	case "24h":
		return 24 * time.Hour
	default:
		// "always" 及其他无效值 → 10 秒短命 token（单次有效）
		return 10 * time.Second
	}
}

// SudoAuth 验证管理员 2FA 动态码，下发 Sudo Token。
// POST /api/admin/sudo-auth
func SudoAuth(c *gin.Context) {
	var req sudoAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	if req.TwoFaCode == "" {
		api.RespondError(c, http.StatusBadRequest, "2FA code is required")
		return
	}

	userUUID, _ := c.Get("uuid")
	uuidStr, _ := userUUID.(string)
	if uuidStr == "" {
		api.RespondError(c, http.StatusUnauthorized, "Not authenticated")
		return
	}

	ok, err := accounts.Verify2Fa(uuidStr, req.TwoFaCode)
	if err != nil || !ok {
		api.RespondError(c, http.StatusForbidden, "Invalid 2FA code")
		return
	}

	duration := parseSudoDuration(req.Duration)
	token := sudotoken.Default.Create(uuidStr, duration)
	maxAge := int(duration.Seconds())

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "sudo_token",
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   utils.GetScheme(c) == "https",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	api.RespondSuccess(c, gin.H{"granted": true})
}

// SudoCheck HTTP 预检：前端建立终端 WS 前先确认 Sudo Token 有效。
// GET /api/admin/sudo-check
func SudoCheck(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	sudo2FARequired, _ := config.GetAs[bool](config.Sudo2FaRequiredKey, false)
	if !sudo2FARequired {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	token, err := c.Cookie("sudo_token")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "sudo token required"})
		return
	}
	_, ok := sudotoken.Default.Validate(token)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "sudo token expired"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}