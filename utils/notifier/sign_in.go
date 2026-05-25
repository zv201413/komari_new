package notifier

import (
	"fmt"
	"time"

	"github.com/komari-monitor/komari/config"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/utils/messageSender"
)

func CheckSignInScheduledWork() {
	for {
		cfg, err := config.GetMany(map[string]any{
			config.ExpireNotificationEnabledKey: false,
		})
		if err != nil {
			time.Sleep(1 * time.Hour)
			continue
		}

		if !cfg[config.ExpireNotificationEnabledKey].(bool) {
			time.Sleep(1 * time.Hour)
			continue
		}

		clients_all, err := clients.GetAllClientBasicInfo()
		if err != nil {
			time.Sleep(1 * time.Hour)
			continue
		}

		now := time.Now()
		var notifyClients []models.Client

		for _, client := range clients_all {
			if !client.RequireSignIn {
				continue
			}

			// Use sign_in_target_date as fallback when expired_at is not set
			expiry := client.ExpiredAt.ToTime()
			if client.ExpiredAt.IsEffectivelyZero() {
				if client.SignInTargetDate == nil || client.SignInTargetDate.IsEffectivelyZero() {
					continue
				}
				expiry = client.SignInTargetDate.ToTime()
			}

			// Calculate absolute days left
			daysLeft := float64(expiry.Sub(now).Hours() / 24)

			if daysLeft <= float64(client.SignInAlertDaysBefore) || client.LastSignInAlertAt.ToTime().IsZero() {
				// Check interval
				intervalHours := float64(client.SignInAlertIntervalHours)
				if intervalHours <= 0 {
					intervalHours = 12 // fallback
				}

				if now.Sub(client.LastSignInAlertAt.ToTime()).Hours() >= intervalHours {
					// Need to notify
					notifyClients = append(notifyClients, client)
					
					// Update LastSignInAlertAt
					dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", client.UUID).Update("last_sign_in_alert_at", models.FromTime(now))
				}
			}
		}

		if len(notifyClients) > 0 {
			for _, client := range notifyClients {
				expiry := client.ExpiredAt.ToTime()
				if client.ExpiredAt.IsEffectivelyZero() && client.SignInTargetDate != nil && !client.SignInTargetDate.IsEffectivelyZero() {
					expiry = client.SignInTargetDate.ToTime()
				}
				daysLeft := int(expiry.Sub(now).Hours() / 24)
				message := fmt.Sprintf("Node %s needs sign-in! It will expire in %d day(s) on %s. Please sign-in.", client.Name, daysLeft, expiry.Format("2006-01-02"))
				
				messageSender.SendEvent(models.EventMessage{
					Event:   messageevent.SignIn,
					Clients: []models.Client{client},
					Time:    now,
					Message: message,
					Emoji:   "📝",
				})
			}
		}

		time.Sleep(1 * time.Hour)
	}
}
