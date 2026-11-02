package admin

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/utils/sudotoken"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
)

const ipTrustCookie = "komari_ip_trust_admin"

var ipTrustGrants = sudotoken.New()
var ipTrustPolicyMu sync.Mutex

func ipTrustUnlocked(c *gin.Context) bool {
	enabled, err := config.GetAs[bool](config.NodeIPTrustEnabledKey, false)
	if err != nil || !enabled {
		return false
	}
	epoch, err := config.GetAs[string](config.NodeIPTrustEpochKey, "")
	if err != nil || epoch == "" {
		return false
	}
	token, _ := c.Cookie(ipTrustCookie)
	session, _ := c.Cookie("session_token")
	return ipTrustGrants.Validate(token, c.GetString("uuid"), session, "ip-trust:"+epoch, false)
}

func GetNodeIPTrustPolicy(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	ipTrustPolicyMu.Lock()
	defer ipTrustPolicyMu.Unlock()
	enabled, err := config.GetAs[bool](config.NodeIPTrustEnabledKey, false)
	if err != nil {
		api.RespondError(c, 500, "Failed to read IP trust policy")
		return
	}
	api.RespondSuccess(c, gin.H{"enabled": enabled, "unlocked": ipTrustUnlocked(c)})
}

// SetNodeIPTrustPolicy verifies the master switch once and issues an editing
// grant bound to this user's login session. Disabling rotates the policy epoch
// so old editing grants cannot become valid again after re-enabling.
func SetNodeIPTrustPolicy(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		api.RespondError(c, 400, "enabled must be a boolean")
		return
	}
	actor := c.GetString("uuid")
	session, _ := c.Cookie("session_token")
	if session == "" {
		api.RespondError(c, 401, "A user login session is required")
		return
	}
	if err := api.VerifySensitive2FACore(actor, c.GetHeader("X-2FA-Code"), false); err != nil {
		api.RespondError(c, 401, err.Error())
		return
	}
	var loginSession models.Session
	if err := dbcore.GetDBInstance().Where("session = ? AND uuid = ?", session, actor).First(&loginSession).Error; err != nil || !loginSession.Expires.After(time.Now()) {
		api.RespondError(c, 401, "Login session expired")
		return
	}
	ipTrustPolicyMu.Lock()
	defer ipTrustPolicyMu.Unlock()
	wasOn, err := config.GetAs[bool](config.NodeIPTrustEnabledKey, false)
	if err != nil {
		api.RespondError(c, 500, "Failed to read IP trust policy")
		return
	}
	epoch, err := config.GetAs[string](config.NodeIPTrustEpochKey, "")
	if err != nil {
		api.RespondError(c, 500, "Failed to read IP trust policy")
		return
	}
	if !*req.Enabled || !wasOn || epoch == "" {
		epoch = utils.GenerateToken()
	}
	if err := config.SetMany(map[string]any{config.NodeIPTrustEnabledKey: *req.Enabled, config.NodeIPTrustEpochKey: epoch}); err != nil {
		api.RespondError(c, 500, "Failed to save IP trust policy")
		return
	}
	value, maxAge := "", -1
	if *req.Enabled {
		value = ipTrustGrants.Create(actor, session, "ip-trust:"+epoch, time.Until(loginSession.Expires), false)
		maxAge = 0 // Editing unlock lasts for this browser session; never persist it.
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: ipTrustCookie, Value: value, Path: "/api/admin/client/", MaxAge: maxAge, HttpOnly: true, Secure: utils.GetScheme(c) == "https", SameSite: http.SameSiteLaxMode})
	auditlog.Log(c.ClientIP(), actor, fmt.Sprintf("node IP trust master verified: %t", *req.Enabled), "security")
	api.RespondSuccess(c, gin.H{"enabled": *req.Enabled, "unlocked": *req.Enabled})
}

// SetClientLoginIPTrust checks the master switch and session grant when adding
// trust. Revoking a node remains available even when the master valve is closed.
func SetClientLoginIPTrust(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		api.RespondError(c, http.StatusBadRequest, "enabled must be a boolean")
		return
	}
	uuid := c.Param("uuid")
	node, err := clients.GetClientByUUID(uuid)
	if err != nil {
		api.RespondError(c, http.StatusNotFound, "Client not found")
		return
	}
	ipTrustPolicyMu.Lock()
	defer ipTrustPolicyMu.Unlock()
	if *req.Enabled && !ipTrustUnlocked(c) {
		api.RespondError(c, http.StatusUnauthorized, "Verify the IP trust master switch before changing nodes")
		return
	}
	agent_runtime.PopulateClientIPTrust(&node)
	if *req.Enabled && !node.IPTrustEligible {
		api.RespondError(c, http.StatusConflict, "IP trust requires a displayed node IP matching its live connection source; NAT64/proxy exits are not trusted")
		return
	}
	if err := dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", uuid).
		Update("trust_login_ip", *req.Enabled).Error; err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to update node IP trust")
		return
	}
	auditlog.Log(c.ClientIP(), c.GetString("uuid"), fmt.Sprintf("node %s login IP trust: %t", uuid, *req.Enabled), "security")
	api.RespondSuccess(c, gin.H{"trust_login_ip": *req.Enabled})
}
