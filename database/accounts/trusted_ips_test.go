package accounts

import (
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

// setupUser 把全局 SQLite 指向临时文件并建一个测试用户。
// dbcore.Initialize 用 sync.Once，故整个测试进程共用同一个库。
func setupUser(t *testing.T, uuid string, ips ...string) {
	t.Helper()
	if flags.DatabaseFile == "" {
		flags.DatabaseFile = filepath.Join(t.TempDir(), "komari.db")
	}
	db := dbcore.GetDBInstance()
	if err := db.Create(&models.User{
		UUID: uuid, Username: uuid, TrustedIPs: models.StringArray(ips),
	}).Error; err != nil {
		t.Fatalf("建测试用户失败: %v", err)
	}
}

// 回归测试：白名单剩余 ≥2 项时删除。裸 []string 会被 GORM 展开成行值
// (?,?,...)，SQLite 报 "row value misused"；必须用 models.StringArray。
func TestRemoveTrustedIPWithMultipleRemaining(t *testing.T) {
	setupUser(t, "u-multi", "1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5")

	if err := RemoveTrustedIP("u-multi", "3.3.3.3"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	got, err := GetTrustedIPs("u-multi")
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	want := []string{"1.1.1.1", "2.2.2.2", "4.4.4.4", "5.5.5.5"}
	if len(got) != len(want) {
		t.Fatalf("剩余 IP = %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("剩余 IP = %v, 期望 %v", got, want)
		}
	}
}

func TestRemoveTrustedIPToEmpty(t *testing.T) {
	setupUser(t, "u-empty", "1.1.1.1")

	if err := RemoveTrustedIP("u-empty", "1.1.1.1"); err != nil {
		t.Fatalf("删到空失败: %v", err)
	}
	got, err := GetTrustedIPs("u-empty")
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("期望空白名单，实得 %v", got)
	}
}

func TestRemoveTrustedIPIdempotent(t *testing.T) {
	setupUser(t, "u-idem", "1.1.1.1", "2.2.2.2")

	if err := RemoveTrustedIP("u-idem", "9.9.9.9"); err != nil {
		t.Fatalf("删除不存在的 IP 应幂等成功: %v", err)
	}
	got, _ := GetTrustedIPs("u-idem")
	if len(got) != 2 {
		t.Fatalf("白名单不应被改动，实得 %v", got)
	}
}

// 增删往返：确认 IsTrustedIP 在多项场景下与落库值一致
func TestAddRemoveRoundTrip(t *testing.T) {
	setupUser(t, "u-rt", "1.1.1.1", "2.2.2.2", "3.3.3.3")

	if err := AddTrustedIP("u-rt", "4.4.4.4"); err != nil {
		t.Fatalf("添加失败: %v", err)
	}
	if ok, _ := IsTrustedIP("u-rt", "4.4.4.4"); !ok {
		t.Fatal("添加后 IsTrustedIP 应为 true")
	}
	if err := RemoveTrustedIP("u-rt", "4.4.4.4"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if ok, _ := IsTrustedIP("u-rt", "4.4.4.4"); ok {
		t.Fatal("删除后 IsTrustedIP 应为 false")
	}
}
