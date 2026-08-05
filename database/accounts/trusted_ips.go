package accounts

import (
	"fmt"
	"net"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

// GetTrustedIPs 获取用户的 IP 白名单
func GetTrustedIPs(uuid string) ([]string, error) {
	db := dbcore.GetDBInstance()
	var user models.User
	err := db.Select("trusted_ips").Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return user.TrustedIPs, nil
}

// AddTrustedIP 添加 IP 到白名单（幂等操作，已存在则返回成功）
func AddTrustedIP(uuid, ip string) error {
	// 验证 IP 格式
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	db := dbcore.GetDBInstance()
	var user models.User
	err := db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return err
	}

	// 检查是否已存在
	for _, existingIP := range user.TrustedIPs {
		if existingIP == ip {
			return nil // 已存在，幂等返回
		}
	}

	// 追加新 IP
	user.TrustedIPs = append(user.TrustedIPs, ip)
	return db.Model(&models.User{}).Where("uuid = ?", uuid).Update("trusted_ips", user.TrustedIPs).Error
}

// RemoveTrustedIP 从白名单删除 IP（幂等操作，不存在则返回成功）
func RemoveTrustedIP(uuid, ip string) error {
	db := dbcore.GetDBInstance()
	var user models.User
	err := db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return err
	}

	// 过滤掉目标 IP
	newIPs := make([]string, 0, len(user.TrustedIPs))
	for _, existingIP := range user.TrustedIPs {
		if existingIP != ip {
			newIPs = append(newIPs, existingIP)
		}
	}

	// 如果长度没变说明原本就不存在，幂等返回
	if len(newIPs) == len(user.TrustedIPs) {
		return nil
	}

	return db.Model(&models.User{}).Where("uuid = ?", uuid).Update("trusted_ips", newIPs).Error
}

// IsTrustedIP 检查 IP 是否在用户的白名单中
func IsTrustedIP(uuid, ip string) (bool, error) {
	trustedIPs, err := GetTrustedIPs(uuid)
	if err != nil {
		return false, err
	}

	for _, trustedIP := range trustedIPs {
		if trustedIP == ip {
			return true, nil
		}
	}
	return false, nil
}
