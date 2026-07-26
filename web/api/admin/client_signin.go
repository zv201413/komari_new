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
	// 快捷签到 = 按签到周期天数顺延，从当前时间起算。
	// 不受 sign_in_target_date 影响：该字段属于「签到至指定日期」模式，
	// 若切回「按天数顺延」后仍有残留值，也不能封顶快捷签到的顺延结果。
	newExpiredAt := now.AddDate(0, 0, client.SignInIntervalDays)

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
