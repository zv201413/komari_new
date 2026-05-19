# 🔒 komari_new 技术深度总结

**项目状态**: Completed & Verified (v1.3.0)
**更新日期**: 2026-05-19
**技术栈**: Go → GORM → SQLite → gRPC → Telegram API

---

## 1. 项目背景与目标

Komari 面板服务端，修复了关键的离线通知 Bug，并支持接收和存储来自容器的小数/分数 CPU 核心数据

---

## 2. 核心架构设计

- Go
- GORM
- SQLite/MySQL

---

## 3. 开发日志 (最近更新)

### [2026-05-19] patch - 支持分数/小数 CPU 核心接收，修复 RPC Error -32603

| 字段 | 内容 |
|:---|:---|
| 问题 | 接收 Agent 上报的 0.5 核时，服务端 models.Client.CpuCores 是 int，导致 GORM 写入报错及 RPC -32603 失败，且通知格式化溢出 |
| 解法 | 修改 models.go, modules.go, nezha_compat.go, client.go 将 CpuCores 数据类型改为 float64，数据库列类型适配为 real/double；修改 offline.go 通知逻辑，浮点核心数剔除尾随 .0 兼容常规展示 |

### [2026-05-18] patch - 修复离线通知与脚本组件 Bug

| 字段 | 内容 |
|:---|:---|
| 问题 | Telegram 通知静默失败，消息没有携带基本环境信息，且首次上线连接不会触发 Registered 消息 |
| 解法 | 在 messageSender/loader.go 抛出初始化错误；修改 notify 触发流程，在离线与上线通知中通过 buildClientInfo 组装 IP/CPU/内核等基础环境信息；修复 test.go 中的多个 bug |

---

**文档性质**: 本地私密归档
**生成时间**: 2026-05-19 15:31:32
