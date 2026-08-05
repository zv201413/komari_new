# 增强特性详情 (vs 上游 Komari)

## 🔐 登录会话

- **不勾选"记住我"**：浏览器会话级 Cookie（无 `Max-Age`），关闭浏览器后失效；服务端 Session 保留 30 天防刷新失效
- **勾选"记住我"**：Cookie 和 Session 均持久化，默认 30 天；登录框右侧可自定义天数（1–365）
- **IP 白名单**：后台「设置 → 登录」可添加受信任的 IP；白名单内 IP 登录时免输 2FA 动态码（增删白名单本身强制 2FA 验证；终端不受白名单影响，仍走 `sudo_token`）

## 🌐 全局时区

- 后台「站点设置」配置 IANA 时区字符串（如 `Asia/Shanghai`、`America/New_York`）
- 留空则前后端均使用浏览器本地时区
- 所有时间显示（图表、日志、会话、节点时间戳等）统一遵循此设置

## 🖼️ 图片直传

- 主题管理中背景图和 Logo 支持直接上传文件到服务器，自动填充 URL
- 支持 `image/*`（WebP/PNG/JPG/GIF/AVIF 等）、`video/*`、`.svg`，≤ 10MB

## 📱 横竖屏不同背景

- 桌面端与移动端背景图独立设置，各自支持亮/暗模式双图（`|` 分隔）
- 可选视频背景，同样区分桌面端和移动端

## ✅ 签到管理

- 签到目标日期 / 间隔天数 / 提前提醒天数 / 提醒间隔（小时）
- Dashboard 卡片直接显示签到截止状态（正常 / 即将到期 / 逾期）
- 签到日志记录每次操作

## 🌐 NAT 类型检测

- Agent 自动检测节点 NAT 类型：公网 IP / 锥型 NAT / 对称 NAT / STUN 不可达 / UDP 阻断
- 基于 STUN 协议（RFC 5389），支持 Google/Cloudflare/Voipbuster fallback

## 🗺️ 点亮全球地图

- Dashboard 顶部展示点亮地图，在线/离线节点不同颜色标记
- 支持当前在线 / 点亮地区 / 流量概览 / 网络速率等统计卡片

## 🖥️ 系统信息增强

- TCP 拥塞算法显示（bbr/cubic 等）
- CPU 浮点精度（如 0.5 核）
- 容器环境下正确读取 cgroup 内存限额
- 1/5/15 分钟平均负载展示
- OpenVZ/LXC 环境自动 fallback

## 🔧 Agent 增强

- 非 Root 安装：自动降级到 `~/.komari` + nohup
- 内置自动更新（每 6h 检查），自动替换二进制并重启
- 三层保活：Go 无限循环重连 + 系统服务重启 + 自动更新触发重启
- 支持 supervisord 管理

## 📦 Docker 一体镜像 (Komari_ttyd)

- 集成 ttyd 网页终端（多端口，独立用户/密码）
- 内置 cloudflared，一行配置接入 Cloudflare Tunnel
- Nginx 反向代理 + Supervisor 进程管理
- 环境变量快速配置：`TTYD_P1`、`TTYD_P2`、`KOMARI_LISTEN` 等

## ⚙️ 主题管理

- 支持上传自定义主题包（`.zip`，需含 `komari-theme.json`）
- 动态加载主题字段并渲染配置界面
- `select-with-custom` 字段类型：下拉 + 自定义输入 + 图片上传

## 📊 离线通知修复

- 修复原版偶发离线误报
- 支持自定义通知渠道（Telegram、WebHook 等）
