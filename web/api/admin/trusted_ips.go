package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/web/api"
)

// GetTrustedIPs 获取当前用户的 IP 白名单
// GET /api/admin/trusted-ips
func GetTrustedIPs(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	uuidStr, _ := uuid.(string)

	ips, err := accounts.GetTrustedIPs(uuidStr)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to get trusted IPs: "+err.Error())
		return
	}

	api.RespondSuccess(c, gin.H{"trusted_ips": ips})
}

type addTrustedIPRequest struct {
	IP string `json:"ip"`
}

// AddTrustedIP 添加 IP 到白名单（需要 2FA 验证）
// POST /api/admin/trusted-ips/add
func AddTrustedIP(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	uuidStr, _ := uuid.(string)

	var req addTrustedIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	if req.IP == "" {
		api.RespondError(c, http.StatusBadRequest, "IP address is required")
		return
	}

	err := accounts.AddTrustedIP(uuidStr, req.IP)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Failed to add trusted IP: "+err.Error())
		return
	}

	auditlog.Log(c.ClientIP(), uuidStr, "added trusted IP: "+req.IP, "security")
	api.RespondSuccess(c, gin.H{"message": "Trusted IP added successfully"})
}

type removeTrustedIPRequest struct {
	IP string `json:"ip"`
}

// RemoveTrustedIP 从白名单删除 IP（需要 2FA 验证）
// POST /api/admin/trusted-ips/remove
func RemoveTrustedIP(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	uuidStr, _ := uuid.(string)

	var req removeTrustedIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	if req.IP == "" {
		api.RespondError(c, http.StatusBadRequest, "IP address is required")
		return
	}

	err := accounts.RemoveTrustedIP(uuidStr, req.IP)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to remove trusted IP: "+err.Error())
		return
	}

	auditlog.Log(c.ClientIP(), uuidStr, "removed trusted IP: "+req.IP, "security")
	api.RespondSuccess(c, gin.H{"message": "Trusted IP removed successfully"})
}
