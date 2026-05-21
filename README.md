# Komari

Komari 是一款轻量级服务器监控工具，支持通过网页面板查看服务器状态，通过轻量 Agent 采集数据。

## 生态概览

本组织维护以下四个仓库，它们共同组成 Komari 监控体系：

| 仓库 | 作用 | 用户需要操作？ |
|------|------|:---:|
| [`komari_new`](https://github.com/zv201413/komari_new) | 监控面板服务端（Go 二进制） | ✅ 必须部署 |
| [`komari-agent_new`](https://github.com/zv201413/komari-agent_new) | 探针 Agent（安装在被监控的机器上） | ✅ 每台机器装 |
| [`Komari_ttyd`](https://github.com/zv201413/Komari_ttyd) | Docker 一体镜像（面板 + 网页终端） | ✅ 替代方案 |
| [`komari-web_new`](https://github.com/zv201413/komari-web_new) | 前端 UI 源码 | ❌ 已编译进 server |

[简体中文](./docs/README_zh.md) | [繁體中文](./docs/README_zh-TW.md) 

📖 **详细安装说明**: [`install.md`](./install.md)
效果展示：
<img width="652" height="637" alt="image" src="https://github.com/user-attachments/assets/5afc92ac-1d3d-40fa-9666-8be11f02b9c8" />

## Quick Start

### Option A: Deploy Server (Binary)

```bash
# 1. 下载最新 server 二进制（如果 wget 失败，改 curl -sL）
wget -qO /opt/komari/komari https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64
chmod +x /opt/komari/komari

# 2. 创建 systemd 服务
cat > /etc/systemd/system/komari.service << 'SERVICE'
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
SERVICE

systemctl daemon-reload && systemctl enable --now komari
```

### Option B: Deploy Server (Docker)

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

### Install Agent (on every server to monitor)

```bash
curl -fsSL https://github.com/zv201413/komari-agent_new/releases/latest/download/install.sh \
  | bash -s -- -e https://your-server.com:25774 -t your_agent_secret
```

### Upgrade

```bash
# Server（如果 wget 失败，改 curl -sL）
wget -qO /opt/komari/komari https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64
chmod +x /opt/komari/komari && systemctl restart komari

# Agent — 重跑 install.sh 即可自动更新
```

## Releases

- [komari_new Releases](https://github.com/zv201413/komari_new/releases) — Server 二进制
- [komari-agent_new Releases](https://github.com/zv201413/komari-agent_new/releases) — Agent + install.sh
- [Komari_ttyd Packages](https://github.com/zv201413/Komari_ttyd/pkgs/container/komari_ttyd) — Docker 镜像

## Manual Build

```bash
# 前端（komari-web_new）
git clone https://github.com/zv201413/komari-web_new
cd komari-web_new
npm install && npm run build
# 编译产物在 public/defaultTheme/dist/

# 服务端
git clone https://github.com/zv201413/komari_new
cd komari_new
cp -r ../komari-web_new/public/defaultTheme/dist public/defaultTheme/
go build -o komari
```
