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
	var newExpiredAt time.Time

	if client.SignInTargetDate != nil && !client.SignInTargetDate.ToTime().IsZero() {
		newExpiredAt = client.SignInTargetDate.ToTime()
	} else {
		newExpiredAt = now.AddDate(0, 0, client.SignInIntervalDays)
	}

	updates := map[string]interface{}{
		"expired_at":            models.FromTime(newExpiredAt),
		"last_sign_in_alert_at": models.FromTime(now),
	}

	if err := db.Model(&client).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to sign in"})
		return
	}

	user_uuid, _ := c.Get("uuid")
	userUUIDStr, _ := user_uuid.(string)
	auditlog.Log(c.ClientIP(), userUUIDStr, "sign_in client:"+client.Name+" next:"+newExpiredAt.Format("2006-01-02"), "info")

	c.JSON(http.StatusOK, gin.H{"status": "success", "expired_at": models.FromTime(newExpiredAt), "message": "Sign-in successful"})
}
