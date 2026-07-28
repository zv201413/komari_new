# Komari

Komari 是一款轻量级服务器监控工具，支持通过网页面板查看服务器状态，通过轻量 Agent 采集数据。

> [!WARNING]
> Komari 是一款自托管监控和控制程序，仅适用于您拥有或授权管理的系统。不得将 Komari 用于武器化、未经同意部署、未授权访问、持久化驻留、命令执行或其他滥用行为。有关真实滥用风险，请参考 Huntress 的分析：[Komari C2 agent abuse](https://www.huntress.com/blog/komari-c2-agent-abuse)。
> 用户应对其如何部署和运营 Komari 承担全部责任。开发人员不对未授权或滥用使用以及由此产生的任何后果负责。
> 在 Windows 上，启用远程控制时，客户端会在每次用户登录时显示 Windows 通知，提醒用户 Komari 是远程控制软件。

## 效果展示

<img width="1815" height="896" alt="image" src="https://github.com/user-attachments/assets/b43eeb64-d903-4518-9f9f-607da9248436" />

## 生态概览

| 仓库 | 作用 | 用户需要操作？ |
|------|------|:---:|
| [`komari_new`](https://github.com/zv201413/komari_new) | 监控面板服务端（Go 二进制） | ✅ 必须部署 |
| [`komari-agent_new`](https://github.com/zv201413/komari-agent_new) | 探针 Agent（安装在被监控的机器上） | ✅ 每台机器装 |
| [`Komari_ttyd`](https://github.com/zv201413/Komari_ttyd) | Docker 一体镜像（面板 + 网页终端） | ✅ 替代方案 |
| [`komari-web_new`](https://github.com/zv201413/komari-web_new) | 前端 UI 源码 | ❌ 已编译进 server |

[简体中文](./docs/README_zh.md) | [繁體中文](./docs/README_zh-TW.md) | 📖 [详细安装说明](./install.md)

## 相比上游的增强

本 Fork 在原版 Komari 基础上进行了大量增强，主要包括：

- 🔐 **登录会话**：可自定义"记住我"天数（1–365 天），不勾选则为浏览器会话级 Cookie
- 🌐 **全局时区**：后台可配置 IANA 时区，留空则使用浏览器本地时区，全站时间显示统一
- 🖼️ **图片直传**：背景图/Logo 支持直接上传到服务器，无需外部图床
- 📱 **横竖屏背景**：桌面端与移动端背景图独立设置，支持亮/暗双图
- ✅ **签到管理**：签到截止日期、间隔、提醒，Dashboard 直接显示状态
- 🌐 **NAT 类型检测**：Agent 自动检测并在面板展示节点 NAT 类型
- 🗺️ **点亮全球地图**：Dashboard 顶部地图标记所有节点地理位置
- 🖥️ **系统信息增强**：TCP 拥塞算法、CPU 浮点精度、cgroup 内存、负载显示
- 🔧 **Agent 增强**：非 Root 安装、内置自动更新、三层保活
- 📦 **Docker 一体镜像**：集成 ttyd 网页终端 + Cloudflare Tunnel + Nginx

👉 [查看完整增强特性说明](./docs/features.md)

## Quick Start

### Option A: Binary

```bash
wget -qO /opt/komari/komari https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64
chmod +x /opt/komari/komari

cat > /etc/systemd/system/komari.service << 'EOF'
[Unit]
Description=Komari Server
After=network.target

[Service]
Type=simple
ExecStart=/opt/komari/komari server -l 0.0.0.0:25774
WorkingDirectory=/opt/komari
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload && systemctl enable --now komari
```

### Option B: Docker

```bash
docker run -d --name komari \
  --restart unless-stopped \
  -p 25774:25774 \
  -v /opt/komari/data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=your_password \
  ghcr.io/zv201413/komari_ttyd:latest
```

> 首次登录的用户名密码见容器日志：`docker logs komari`

### Install Agent

```bash
curl -fsSL https://github.com/zv201413/komari-agent_new/releases/latest/download/install.sh \
  | bash -s -- -e https://your-server.com:25774 -t your_agent_secret
```

### Upgrade

```bash
# Server
wget -qO /opt/komari/komari https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64
chmod +x /opt/komari/komari && systemctl restart komari

# Agent — 重跑 install.sh 即可
```

## Releases

- [komari_new Releases](https://github.com/zv201413/komari_new/releases) — Server 二进制
- [komari-agent_new Releases](https://github.com/zv201413/komari-agent_new/releases) — Agent + install.sh
- [Komari_ttyd Packages](https://github.com/zv201413/Komari_ttyd/pkgs/container/komari_ttyd) — Docker 镜像

## Manual Build

```bash
# 前端
git clone https://github.com/zv201413/komari-web_new
cd komari-web_new && npm install && npm run build

# 服务端
git clone https://github.com/zv201413/komari_new
cd komari_new
cp -r ../komari-web_new/public/defaultTheme/dist public/defaultTheme/
go build -o komari
```
