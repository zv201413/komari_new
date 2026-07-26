# Komari

Komari 是一款轻量级服务器监控工具，支持通过网页面板查看服务器状态，通过轻量 Agent 采集数据。

> [!WARNING]
> Komari 是一款自托管监控和控制程序，仅适用于您拥有或授权管理的系统。不得将 Komari 用于武器化、未经同意部署、未授权访问、持久化驻留、命令执行或其他滥用行为。有关真实滥用风险，请参考 Huntress 的分析：[Komari C2 agent abuse](https://www.huntress.com/blog/komari-c2-agent-abuse)。
> 用户应对其如何部署和运营 Komari 承担全部责任。开发人员不对未授权或滥用使用以及由此产生的任何后果负责。
> 在 Windows 上，启用远程控制时，客户端会在每次用户登录时显示 Windows 通知，提醒用户 Komari 是远程控制软件。

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
<img width="1815" height="896" alt="image" src="https://github.com/user-attachments/assets/b43eeb64-d903-4518-9f9f-607da9248436" />

## 增强特性 (vs 上游)

本 Fork 在原版 Komari 基础上进行了大量增强，涵盖面板功能、Agent 能力和部署体验：

### 🖼️ 图片直传
- 在主题管理设置中，背景图和 Logo 支持**直接上传图片文件**到服务器
- 上传后自动填充 URL，无需手动上传图床再复制链接
- 支持格式：`image/*`（WebP、PNG、JPG、GIF、AVIF 等）、`video/*`、`.svg`
- 上传文件 ≤ 10MB，自动检测 MIME 类型
<img width="1485" height="519" alt="image" src="https://github.com/user-attachments/assets/934e005d-2de2-49cf-8739-bb03fd242a68" />


### 📱 横竖屏不同背景
- **桌面端背景图** 和 **移动端背景图** 独立设置
- 桌面端建议使用宽屏横图，移动端建议使用竖屏长图
- 各自支持亮/暗模式双图（`|` 分隔）
- 可选用视频背景，同样区分桌面端和移动端

### ✅ 签到管理
- **签到目标日期**：设置具体的签到截止日期（如 2026-05-27），到期自动提醒
- **签到间隔天数**：按周期签到（如每 30 天）
- **提前提醒天数**：到期前 N 天开始推送签到提醒
- **提醒间隔**：控制提醒消息的重复频率（小时）
- 前端 Dashboard 卡片直接显示签到截止状态（正常/即将到期/逾期）
- 签到日志记录每次签到操作

### 🌐 NAT 类型检测
- Agent 自动检测节点的 NAT 类型，面板直接显示
- 检测结果分类：**公网 IP** / **锥型 NAT** / **对称 NAT** / **STUN 不可达** / **UDP 阻断**
- 基于 STUN 协议（RFC 5389），支持多服务器 fallback（Google/Cloudflare/Voipbuster）
- 首次检测同步阻塞（~3-6s），后续保持异步更新

### 🗺️ 点亮全球地图
- Dashboard 顶部展示**点亮地图**，标记所有节点的地理位置
- 在线节点与离线节点用不同颜色区分，直观查看全球覆盖
- 地图显示在 Dashboard 最上方，支持 **当前在线** / **点亮地区** / **流量概览** / **网络速率** 等统计卡片

### 🖥️ 系统信息增强
- **TCP 拥塞算法显示**：在节点卡片上显示当前拥塞控制算法（bbr、cubic 等）
- **CPU 精确显示**：支持浮点数精度，不再整数截断（如 0.5 核）
- **Cgroup 内存**：容器环境下正确读取 cgroup 内存限额，而非物理内存总量
- **负载显示**：1/5/15 分钟平均负载标量展示
- **容器兼容**：OpenVZ/LXC 环境下自动 fallback 检测方法，避免读取被屏蔽的 proc 文件

### 🔧 Agent 增强
- **非 Root 安装**：自动降级到 `~/.komari` + nohup 后台模式
- **自动更新**：内置 `go-github-selfupdate`，每 6h 检查新版本，自动替换二进制并重启
- **三层保活**：Go 无限循环重连 + 系统服务重启 + 自动更新触发重启
- **supervisord 兼容**：支持 Docker 容器内通过 supervisord 管理进程

### 📦 Docker 一体镜像 (Komari_ttyd)
- **网页终端**：集成 ttyd，支持多端口 Web 终端（设置不同用户/密码）
- **Cloudflare Tunnel**：内置 cloudflared，一行配置即可通过 CF Tunnel 暴露服务
- **Nginx 反向代理**：自动代理 komari 面板和 ttyd 终端
- **Supervisor 进程管理**：多进程统一管理
- **环境变量配置**：通过 `TTYD_P1`、`TTYD_P2`、`KOMARI_LISTEN` 等快速配置

### ⚙️ 主题管理
- 支持上传自定义主题包（`.zip`，需包含 `komari-theme.json`）
- 主题设置页（主题管理）动态加载主题字段并渲染配置界面
- `select-with-custom` 字段类型：下拉选择 + 自定义输入 + 图片上传

### 📊 离线通知修复
- 修复了原版偶发的离线误报问题
- 支持自定义通知渠道（Telegram、WebHook 等）

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
