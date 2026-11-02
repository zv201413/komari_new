package public

import (
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func trustedAgentLoginIP(ip string) bool {
	enabled, err := config.GetAs[bool](config.NodeIPTrustEnabledKey, false)
	if err != nil || !enabled {
		return false
	}
	var nodes []models.Client
	if err := dbcore.GetDBInstance().Select("uuid", "token", "ipv4", "ipv6").Where("trust_login_ip = ?", true).Find(&nodes).Error; err != nil {
		return false
	}
	for _, node := range nodes {
		if agent_runtime.MatchesLoginIP(node.UUID, node.Token, node.IPv4, node.IPv6, ip) {
			return true
		}
	}
	return false
}
