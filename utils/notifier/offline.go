package notifier

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/config"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/utils/messageSender"
	"github.com/komari-monitor/komari/utils/renewal"
)

func buildClientInfo(client models.Client) string {
	var parts []string
	if client.IPv4 != "" {
		parts = append(parts, fmt.Sprintf("IP: %s", client.IPv4))
	}
	if client.IPv6 != "" {
		parts = append(parts, fmt.Sprintf("IPv6: %s", client.IPv6))
	}
	osInfo := client.OS
	if client.Arch != "" {
		osInfo = fmt.Sprintf("%s %s", osInfo, client.Arch)
	}
	if osInfo != "" {
		parts = append(parts, fmt.Sprintf("OS: %s", osInfo))
	}
	if client.Region != "" {
		parts = append(parts, fmt.Sprintf("Region: %s", client.Region))
	}
	if client.CpuCores > 0 {
		coresStr := strconv.FormatFloat(math.Round(client.CpuCores*100)/100, 'f', -1, 64)
		cpuInfo := fmt.Sprintf("CPU: %s cores", coresStr)
		if client.CpuName != "" {
			cpuInfo = fmt.Sprintf("CPU: %s (%s cores)", client.CpuName, coresStr)
		}
		parts = append(parts, cpuInfo)
	}
	return strings.Join(parts, "\n")
}

// notificationState 保存单个客户端的通知状态。
// 通过在结构体中嵌入互斥锁，实现每个客户端细粒度的锁定，比全局锁更高效。
type notificationState struct {
	mu                  sync.Mutex // 互斥锁，保护该客户端状态
	pendingOfflineSince time.Time  // 客户端离线的时间。为零值表示客户端在线或已发送离线通知。
	isFirstConnection   bool       // 标记是否为首次上线连接。
	isConnExist         bool       // 标记是否存在连接
	connectionID        int64      // 连接ID，用于区分不同的连接会话，防止竞态条件
}

// clientStates 使用 sync.Map 实现对客户端状态的并发访问。
// 映射关系：clientID (string) -> *notificationState
var clientStates sync.Map

// getNotificationConfig 获取指定客户端的通知配置。
// 返回配置对象和一个布尔值，指示全局和该客户端是否启用通知。
func getNotificationConfig(clientID string) (*models.OfflineNotification, bool) {
	conf, err := config.GetAs[bool](config.NotificationEnabledKey, false)
	if err != nil || !conf {
		return nil, false
	}

	notiConf := models.OfflineNotification{Client: clientID}
	db := dbcore.GetDBInstance()
	if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).FirstOrCreate(&notiConf).Error; err != nil {
		log.Printf("Failed to get or create offline notification config for client %s: %v", clientID, err)
		return nil, false
	}

	return &notiConf, notiConf.Enable
}

// getOrInitState 从 sync.Map 获取客户端状态，不存在则新建并存储。
func getOrInitState(clientID string) *notificationState {
	// 原子性地加载或存储该客户端的状态。
	val, _ := clientStates.LoadOrStore(clientID, &notificationState{isFirstConnection: true})
	return val.(*notificationState)
}

// OfflineNotification 在启用通知且未在宽限期内发送的情况下，发送客户端离线通知。
func OfflineNotification(clientID string, endedConnectionID int64) {
	client, err := clients.GetClientByUUID(clientID)
	if err != nil {
		return
	}

	notiConf, enabled := getNotificationConfig(clientID)
	if !enabled {
		return
	}

	gracePeriod := time.Duration(notiConf.GracePeriod) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Minute // 默认宽限期
	}

	log.Printf("[diagnostic] OfflineNotification for %s: DB grace_period=%d, effective gracePeriod=%v", clientID, notiConf.GracePeriod, gracePeriod)

	now := time.Now()
	state := getOrInitState(clientID)

	state.mu.Lock()
	// 如果已处于待通知状态，则不做处理。
	// 只有当离线事件来自当前的连接会话时，我们才认为它有效。
	if !state.pendingOfflineSince.IsZero() || state.connectionID != endedConnectionID {
		state.mu.Unlock()
		return
	}
	// 标记该客户端为待离线。
	state.pendingOfflineSince = now
	state.mu.Unlock()

	// 新建协程，等待宽限期后判断是否需要发送通知。
	go func(startTime time.Time, expectedConnectionID int64) {
		time.Sleep(gracePeriod)

		state.mu.Lock()
		defer state.mu.Unlock()

		// 检查离线状态是否仍为本次协程启动时的状态。
		// 若为零值，说明客户端已重连。
		// 当前的 connectionID 是否还是我们触发离线时的那个ID。如果不是，说明客户端重连过，本次离线通知已失效。
		if state.pendingOfflineSince.IsZero() || state.connectionID != expectedConnectionID {
			log.Printf("%s is reconnected new connID: %d, old connID: %d", clientID, state.connectionID, expectedConnectionID)
			return
		}

		// 即将发送通知，重置待通知状态。
		// 需要多一个boolean 是因为pendingOfflineSince在offline睡眠后才修改，可能导致online判断不对
		state.pendingOfflineSince = time.Time{}
		state.isConnExist = false

		// Send notification
		message := fmt.Sprintf("🔴 %s is offline\n%s", client.Name, buildClientInfo(client))
		go func(msg string) {
			if err := messageSender.SendEvent(models.EventMessage{
				Event:   messageevent.Offline,
				Clients: []models.Client{client},
				Time:    time.Now(),
				Message: msg,
				Emoji:   "🔴",
			}); err != nil {
				log.Println("Failed to send offline notification:", err)
			}
		}(message)

		// 更新数据库中的最后通知时间
		db := dbcore.GetDBInstance()
		if err := db.Model(&models.OfflineNotification{}).Where("client = ?", clientID).Update("last_notified", now).Error; err != nil {
			log.Printf("Failed to update last_notified for client %s: %v", clientID, err)
		}
	}(now, endedConnectionID)
}

// OnlineNotification 在启用通知的情况下，发送客户端上线通知。
func OnlineNotification(clientID string, connectionID int64) {
	state := getOrInitState(clientID)

	state.mu.Lock()
	state.connectionID = connectionID

	// 规则1：首次连接 → 发送 Registered 通知。
	if state.isFirstConnection {
		state.isFirstConnection = false
		// 同时清除任何待离线状态（如服务器重启时客户端本已离线）
		state.pendingOfflineSince = time.Time{}
		state.isConnExist = true
		state.mu.Unlock()

		go func() {
			client, err := clients.GetClientByUUID(clientID)
			if err != nil {
				return
			}
			renewal.CheckAndAutoRenewal(client)
			_, enabled := getNotificationConfig(clientID)
			if !enabled {
				return
			}

			message := fmt.Sprintf("🆕 %s registered\n%s", client.Name, buildClientInfo(client))
			if err := messageSender.SendEvent(models.EventMessage{
				Event:   messageevent.Registered,
				Clients: []models.Client{client},
				Time:    time.Now(),
				Message: message,
				Emoji:   "🆕",
			}); err != nil {
				log.Println("Failed to send registered notification:", err)
			}
		}()
		return
	}

	// 检查客户端是否处于待离线状态。
	wasPending := !state.pendingOfflineSince.IsZero()
	// 上线时总是清除待离线状态。
	state.pendingOfflineSince = time.Time{}

	// 规则2：宽限期内重连，不通知。
	if wasPending {
		state.mu.Unlock()
		// 即使不通知，我们依然在后台执行续费检查
		go func() {
			client, err := clients.GetClientByUUID(clientID)
			if err == nil {
				renewal.CheckAndAutoRenewal(client)
			}
		}()
		return
	}

	// 规则3: 没断开后重连, 不通知
	if state.isConnExist {
		log.Printf("%s has connection exist: %d", clientID, connectionID)
		state.mu.Unlock()
		// 即使不通知，我们依然在后台执行续费检查
		go func() {
			client, err := clients.GetClientByUUID(clientID)
			if err == nil {
				renewal.CheckAndAutoRenewal(client)
			}
		}()
		return
	} else {
		state.isConnExist = true
	}
	state.mu.Unlock()

	// 规则4：客户端离线足够久已通知（或未待离线），现在重新上线，发送上线通知。
	go func() {
		client, err := clients.GetClientByUUID(clientID)
		if err != nil {
			return
		}
		renewal.CheckAndAutoRenewal(client)
		_, enabled := getNotificationConfig(clientID)
		if !enabled {
			return
		}

		message := fmt.Sprintf("🟢 %s is online\n%s", client.Name, buildClientInfo(client))
		if err := messageSender.SendEvent(models.EventMessage{
			Event:   messageevent.Online,
			Clients: []models.Client{client},
			Time:    time.Now(),
			Message: message,
			Emoji:   "🟢",
		}); err != nil {
			log.Println("Failed to send online notification:", err)
		}
	}()
}
