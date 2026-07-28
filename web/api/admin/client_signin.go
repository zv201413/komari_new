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
	// 快捷签到统一从当前时间起算：今天 + 顺延天数 + 偏移天数(可正可负)。
	// 偏移值让用户微调最终落到哪一天，不受 sign_in_target_date 残留值影响。
	newExpiredAt := now.AddDate(0, 0, client.SignInIntervalDays+client.SignInOffsetDays)

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
