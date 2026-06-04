package admin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/records"
	"github.com/komari-monitor/komari/ws"
)

func AddClient(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		uuid, token, err := clients.CreateClient()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "uuid": uuid, "token": token})
		return
	}
	uuid, token, err := clients.CreateClientWithName(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	user_uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), user_uuid.(string), "create client:"+uuid, "info")
	c.JSON(http.StatusOK, gin.H{"status": "success", "uuid": uuid, "token": token, "message": ""})
}

func EditClient(c *gin.Context) {
	var req = make(map[string]interface{})
	uuid := c.Param("uuid")
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid or missing UUID"})
		return
	}
	req["uuid"] = uuid
	
	if v, ok := req["expired_at"]; ok {
		if str, ok := v.(string); ok {
			str = strings.TrimSpace(str)
			if str == "" || str == "0001-01-01" || strings.HasPrefix(str, "0001-01-01") {
				req["expired_at"] = nil
			}
		}
	}

	// 流量校正：前端传入"真实已用流量"绝对值(字节)，换算为相对当前实测的偏移量后存储。
	// 展示端会把偏移量加回实测值，因此校正后面板会从该真实值继续累加。
	if _, hasU := req["set_traffic_used_up"]; hasU {
		if _, hasD := req["set_traffic_used_down"]; hasD {
			latestMap := ws.GetLatestReport()
			var rawUp, rawDown int64
			if rep := latestMap[uuid]; rep != nil {
				rawUp = rep.Network.TotalUp
				rawDown = rep.Network.TotalDown
			}
			if f, ok := req["set_traffic_used_up"].(float64); ok {
				req["traffic_offset_up"] = int64(f) - rawUp
			}
			if f, ok := req["set_traffic_used_down"].(float64); ok {
				req["traffic_offset_down"] = int64(f) - rawDown
			}
		}
		delete(req, "set_traffic_used_up")
		delete(req, "set_traffic_used_down")
	}

	err := clients.SaveClient(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	user_uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), user_uuid.(string), "edit client:"+uuid, "info")
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func RemoveClient(c *gin.Context) {
	uuid := c.Param("uuid")
	err := clients.DeleteClient(uuid)
	if err != nil {
		c.JSON(500, gin.H{
			"status": "error",
			"error":  "Failed to delete client" + err.Error(),
		})
		return
	}
	user_uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), user_uuid.(string), "delete client:"+uuid, "warn")
	c.JSON(200, gin.H{"status": "success"})
	ws.DeleteConnectedClients(uuid)
	ws.DeleteLatestReport(uuid)
}

func ClearRecord(c *gin.Context) {
	if err := records.DeleteAll(); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to delete Record" + err.Error(),
		})
		return
	}
	user_uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), user_uuid.(string), "clear records", "warn")
	c.JSON(200, gin.H{"status": "success"})
}

func GetClient(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid or missing UUID",
		})
		return
	}

	result, err := clients.GetClientByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func ListClients(c *gin.Context) {
	cls, err := clients.GetAllClientBasicInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cls)
}

func GetClientToken(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid or missing UUID",
		})
		return
	}

	token, err := clients.GetClientTokenByUUID(uuid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "token": token, "message:": ""})
}
