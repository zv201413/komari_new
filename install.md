# Install / Update Guide

> Fork 版本：Server `v1.4.0` | Agent `v1.4.1` | Docker `komari-ttyd:latest`

---

## Server (komari_new)

### 首次安装

```bash
# 下载 v1.4.0 二进制（如果 wget 失败，改 curl -sL）
wget -qO /opt/komari/komari https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64
chmod +x /opt/komari/komari

# systemd 服务 (示例)
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

systemctl daemon-reload
systemctl enable --now komari
```

### 从旧版升级

```bash
# 备份 + 替换
cp /opt/komari/komari /opt/komari/komari.bak
wget -qO /opt/komari/komari https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64
chmod +x /opt/komari/komari
systemctl restart komari
```

### 验证

```bash
systemctl status komari --no-pager -l
/opt/komari/komari --version   # 应显示 v1.4.0
```

---

## Agent (komari-agent_new)

### 首次安装

```bash
# 用安装脚本自动部署
wget -qO /tmp/install.sh https://github.com/zv201413/komari-agent_new/releases/latest/download/install.sh
chmod +x /tmp/install.sh
bash /tmp/install.sh
```

> 支持非 root 环境（容器/PaaS）：自动降级到 `~/.komari` 目录，使用 nohup 后台模式运行。

### 从旧版升级

同上，重新跑一遍安装脚本即可自动替换为最新版。

### 手动安装

```bash
wget -qO /opt/komari-agent/komari-agent https://github.com/zv201413/komari-agent_new/releases/latest/download/komari-agent-linux-amd64
chmod +x /opt/komari-agent/komari-agent
systemctl restart komari-agent
```

### 验证

```bash
systemctl status komari-agent --no-pager -l
```

---

## Docker (Komari_ttyd)

```bash
docker pull ghcr.io/zv201413/komari_ttyd:latest
docker rm -f komari-ttyd
docker run -d \
  --name komari-ttyd \
  --restart always \
  -p 7681:7681 \
  ghcr.io/zv201413/komari_ttyd:latest
```

> Docker 镜像已配置多架构支持 (linux/amd64 + linux/arm64)，构建时自动从 GitHub Release 拉取最新 Server 二进制。

---

## Release 下载链接

| 组件 | 平台 | 下载 |
|------|------|------|
| Server | linux/amd64 | [komari-linux-amd64](https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-amd64) |
| Server | linux/arm64 | [komari-linux-arm64](https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-arm64) |
| Server | linux/386 | [komari-linux-386](https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-386) |
| Server | linux/riscv64 | [komari-linux-riscv64](https://github.com/zv201413/komari_new/releases/latest/download/komari-linux-riscv64) |
| Agent | linux/amd64 | [komari-agent-linux-amd64](https://github.com/zv201413/komari-agent_new/releases/latest/download/komari-agent-linux-amd64) |
| Agent | linux/arm64 | [komari-agent-linux-arm64](https://github.com/zv201413/komari-agent_new/releases/latest/download/komari-agent-linux-arm64) |
| Agent | linux/386 | [komari-agent-linux-386](https://github.com/zv201413/komari-agent_new/releases/latest/download/komari-agent-linux-386) |
| Agent | linux/riscv64 | [komari-agent-linux-riscv64](https://github.com/zv201413/komari-agent_new/releases/latest/download/komari-agent-linux-riscv64) |
| Agent | windows/amd64 | [komari-agent-windows-amd64.exe](https://github.com/zv201413/komari-agent_new/releases/latest/download/komari-agent-windows-amd64.exe) |

> 完整列表见 Releases: [komari_new](https://github.com/zv201413/komari_new/releases) · [komari-agent_new](https://github.com/zv201413/komari-agent_new/releases)
