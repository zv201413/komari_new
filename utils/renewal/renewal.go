package renewal

import (
	"fmt"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/pkg/timeutil"
	"github.com/komari-monitor/komari/utils/messageSender"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func CheckAndAutoRenewal(client models.Client) {
	// 自动续费检查
	//type renewedClient struct {
	//	Name          string
	//	NewExpireTime time.Time
	//}
	//var renewedClients []renewedClient

	if !client.AutoRenewal {
		return
	}
	// 不在线则不续费
	if _, ok := agent_runtime.GetConnectedClients()[client.UUID]; !ok {
		return
	}
	if client.ExpiredAt == nil {
		return
	}

	clientExpireTime := client.ExpiredAt.UTC()
	checkTime := time.Now().UTC()

	// 如果到期时间小于0002年，跳过
	if clientExpireTime.Year() < 2 {
		return
	}

	// 检查是否已过期或当天过期
	if clientExpireTime.Before(checkTime) || timeutil.SameSystemDate(clientExpireTime, checkTime) {
		// 计算过期时间距离创建时间的总天数，判断是否为长期账单
		now := checkTime
		localNow := now.In(time.Local)
		hundredYearsFromNow := localNow.AddDate(100, 0, 0).UTC()

		// 如果过期时间超过当前时间100年，视为长期/一次性账单，不续费
		if clientExpireTime.After(hundredYearsFromNow) {
			return
		}

		// 如果有账单周期且不为0，进行自动续费
		if client.BillingCycle > 0 {
			// 根据账单周期计算新的过期时间
			var newExpireTime time.Time
			billingCycle := client.BillingCycle

			// 如果服务器的过期时间太早了，那么直接设置为从当前时间算的下一个到期时间
			baseTime := clientExpireTime.In(time.Local)
			if clientExpireTime.Before(localNow.AddDate(0, 0, -30).UTC()) { // 过期时间超过30天前
				baseTime = localNow
			}

			if billingCycle >= 27 && billingCycle <= 32 {
				// 月度计费 - 加1个自然月(按目标月实际天数夹取)
				newExpireTime = addMonthsClamped(baseTime, 1)
			} else if billingCycle >= 87 && billingCycle <= 95 {
				// 季度计费 - 加3个自然月
				newExpireTime = addMonthsClamped(baseTime, 3)
			} else if billingCycle >= 175 && billingCycle <= 185 {
				// 半年计费 - 加6个自然月
				newExpireTime = addMonthsClamped(baseTime, 6)
			} else if billingCycle >= 360 && billingCycle <= 370 {
				// 年度计费 - 加12个自然月
				newExpireTime = addMonthsClamped(baseTime, 12)
			} else if billingCycle >= 720 && billingCycle <= 750 {
				// 两年计费 - 加24个自然月
				newExpireTime = addMonthsClamped(baseTime, 24)
			} else if billingCycle >= 1080 && billingCycle <= 1150 {
				// 三年计费 - 加36个自然月
				newExpireTime = addMonthsClamped(baseTime, 36)
			} else if billingCycle >= 1800 && billingCycle <= 1850 {
				// 五年计费 - 加60个自然月
				newExpireTime = addMonthsClamped(baseTime, 60)
			} else {
				// 其他情况，按字面账单周期天数累加
				newExpireTime = baseTime.AddDate(0, 0, billingCycle)
			}

			// 更新客户端过期时间
			updates := map[string]interface{}{
				"uuid":       client.UUID,
				"expired_at": newExpireTime.UTC(),
			}

			err := clients.SaveClient(updates)
			if err != nil {
				auditlog.EventLog("renewal", fmt.Sprintf("Failed to renew client %s (%s): %v", client.Name, client.UUID, err))
				return
			}

			//renewedClients = append(renewedClients, renewedClient{
			//	Name:          client.Name,
			//	NewExpireTime: newExpireTime,
			//})

			auditlog.EventLog("renewal", fmt.Sprintf("Auto-renewed client: %s until %s",
				client.Name, timeutil.FormatSystemDate(newExpireTime)))

			messageSender.SendEvent(models.EventMessage{
				Event:   messageevent.Renew,
				Clients: []models.Client{client},
				Time:    time.Now().UTC(),
				Emoji:   "🔄",
				Message: fmt.Sprintf("• %s until %s\n", client.Name, timeutil.FormatSystemDate(newExpireTime)),
			})
		}
	}

	// 发送续费通知
	// if len(renewedClients) > 0 {
	// 	message := ""
	// 	for _, clientInfo := range renewedClients {
	// 		message += fmt.Sprintf("• %s until %s\n", clientInfo.Name, clientInfo.NewExpireTime.Format("2006-01-02"))
	// 	}
	// 	messageSender.SendEvent(models.EventMessage{
	// 		Event:   messageevent.Renew,
	// 		Clients: []models.Client{client},
	// 		Time:    time.Now(),
	// 		Emoji:   "🔄",
	// 		Message: message,
	// 	})
	// }
}

// addMonthsClamped 在 t 上增加 months 个自然月，并把"日"夹到目标月的实际最后一天，
// 规避 Go AddDate 的月末进位问题：
//
//	1/31 + 1月 用 AddDate 会变成 3/3(闰年 3/2)，这里夹回 2/28(闰年 2/29)；
//	3/31 + 1月 → 4/30；2/29 + 12月 → 2/28。
//
// 其余日期(如 19 号)行为不变。months 恒为正(1/3/6/12/24/36/60)。
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	// 先定位目标年月(日置 1 避免本步进位)，再读取规范化后的年/月
	target := time.Date(y, m+time.Month(months), 1, 0, 0, 0, 0, t.Location())
	// 目标月最后一天：下个月 0 号即上月末日
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(target.Year(), target.Month(), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}
