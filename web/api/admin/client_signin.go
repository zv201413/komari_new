package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

func ClientSignIn(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid or missing UUID"})
		return
	}

	db := dbcore.GetDBInstance()
	var client models.Client
	if err := db.Where("uuid = ?", uuid).First(&client).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Client not found"})
		return
	}

	if !client.RequireSignIn {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "This node does not require sign-in"})
		return
	}

	now := time.Now()
	// 从 expired_at 起算（与自动续费逻辑一致）；
	// 若已过期超过 30 天则从今天起算，避免积压太久的节点一次性顺延过多。
	baseTime := now
	if client.ExpiredAt != nil {
		expiredAt := client.ExpiredAt.In(time.Local)
		if expiredAt.After(now.AddDate(0, 0, -30)) {
			baseTime = expiredAt
		}
	}
	newExpiredAt := baseTime.AddDate(0, 0, client.SignInIntervalDays)

	updates := map[string]interface{}{
		"expired_at":            newExpiredAt,
		"last_sign_in_alert_at": now,
	}

	if err := db.Model(&client).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to sign in"})
		return
	}

	user_uuid, _ := c.Get("uuid")
	userUUIDStr, _ := user_uuid.(string)
	auditlog.Log(c.ClientIP(), userUUIDStr, "sign_in client:"+client.Name+" next:"+newExpiredAt.Format("2006-01-02"), "info")

	c.JSON(http.StatusOK, gin.H{"status": "success", "expired_at": newExpiredAt, "message": "Sign-in successful"})
}
