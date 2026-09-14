# PikPak 多账号聚合网盘 Web 系统 (PikPak Multi-Account Manager)

[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![React](https://img.shields.io/badge/React-18.3-61dafb.svg)](https://react.dev)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ed.svg)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

一个高性能、低内存、可生产运行的 **PikPak 多账号聚合网盘与离线下载管理系统**。

专为拥有多个 PikPak 账号的用户设计：将分散在各个账号中的存储空间和每日离线下载配额聚合为统一网盘。前端仅呈现一个统一文件系统，离线下载时若某个账号今日额度耗尽，系统自动毫秒级切换下一个可用账号继续执行，实现真正的“多账号合一”。

---

## 🌟 核心特性

- **统一虚拟文件系统**：跨账号聚合所有文件列表，消除账号割裂感；基于 `account_id + file_id` 自动规避跨账号重名文件覆盖，支持搜索、排序与面包屑导航。
- **智能离线调度池 (Account Scheduler)**：
  - **优先级 + 轮询 (Priority + Round Robin)** 调度算法。
  - **额度耗尽自动转移**：精准捕获 `task_daily_create_limit` / `daily task limit reached`，自动故障转移至下一个可用账号。
  - **精细错误分流**：严格区分额度不足、Token失效、代理故障、429频控与网络抖动，网络故障自动指数退避重试，绝不误杀账号。
  - **并发安全控制**：账号级互斥锁与全局调度信号量，高并发提交任务不发生资源竞争与状态错乱。
- **独立网络代理隔离**：每个账号可单独配置直连、HTTP、HTTPS、SOCKS5 或 SOCKS5H 代理，独立实例化 HTTP Transport 与连接池，提供一键代理连通性与出口 IP 测速。
- **流畅视频在线播放 (HTTP Range 206)**：
  - 支持 **Direct (直连)** 与 **Proxy (后端中转)** 双模式。
  - 后端流式中转完整实现 `Range`, `Content-Range`, `Accept-Ranges`, `206 Partial Content`，几十 GB 4K 视频随意拖拽定位。
  - 代理播放时严格穿透对应账号的独立代理，避免直连 IP 泄露或地域受限。
  - 自动记录每个视频的历史播放进度。
- **跨账号批量删除**：多选不同账号下的文件一键批量移入回收站或永久删除，后端自动按账号分组并发调用，返回详细成功/失败清单。
- **敏感凭据高强度加密**：账号密码、Access Token、Refresh Token 均采用系统 `APP_SECRET` 进行 **AES-256-GCM** 加密持久化；日志自动脱敏。
- **外部 REST API & 幂等保障**：
  - 提供标准的 `POST /api/v1/offline` 离线下载接口。
  - 原生支持 `Idempotency-Key` 防重幂等机制。
  - 基于 SHA-256 哈希的 Bearer API Key 鉴权与细粒度权限控制。
- **单镜像极简部署**：React 前端通过 Go `embed` 打包为单一无依赖二进制，Docker 多阶段构建，内存占用仅 30MB 左右，极低资源消耗。

---

## 🚀 快速开始

### 方式一：Docker Compose 部署 (推荐)

1. 创建项目目录并编写 `docker-compose.yml`：
```yaml
version: '3.8'

services:
  pikpak-manager:
    image: pikpak-manager:latest
    build:
      context: .
      dockerfile: Dockerfile
    container_name: pikpak-manager
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      - PORT=8080
      - DATA_DIR=/data
      - APP_SECRET=your-random-32-character-secret-key!
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=admin123456
      - LOG_LEVEL=INFO
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
```

2. 启动服务：
```bash
docker compose up -d
```

3. 访问 Web 界面：浏览器打开 `http://<服务器IP>:8080`。

---

### 方式二：Docker Run 单命令运行

```bash
docker run -d \
  --name pikpak-manager \
  --restart unless-stopped \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e APP_SECRET="your-random-32-character-secret-key!" \
  -e ADMIN_USERNAME="admin" \
  -e ADMIN_PASSWORD="admin123456" \
  pikpak-manager:latest
```

---

### 方式三：本地源码运行 (开发模式)

#### 前置要求
- Go 1.22+
- Node.js 18+

#### 编译与运行
```bash
# 1. 编译前端
cd web
npm install
npm run build
cd ..

# 2. 编译并运行后端
go build -o pikpak-manager ./cmd/server
./pikpak-manager
```

---

## ⚙️ 环境变量配置

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8080` | Web 服务监听端口 |
| `DATA_DIR` | `/data` | SQLite 数据库持久化目录 |
| `APP_SECRET` | 随机生成 | **核心密钥**：用于 AES-256-GCM 加密数据库中的账密和 Token，生产环境必填且务必保密 |
| `ADMIN_USERNAME` | `admin` | 初次启动时的默认管理员账号 |
| `ADMIN_PASSWORD` | `admin123456` | 初次启动时的默认管理员密码 (入库后自动 bcrypt 不可逆加密) |
| `LOG_LEVEL` | `INFO` | 日志输出级别 (`DEBUG`, `INFO`, `WARN`, `ERROR`) |

---

## 📖 使用指南

### 1. 首次登录
打开浏览器访问 `http://localhost:8080`，使用环境变量配置的管理员用户名和密码登录（默认 `admin` / `admin123456`）。登录后可在右上角个人菜单中修改密码。

### 2. 添加 PikPak 账号
进入 **「PikPak 账号」** 页面，点击 **「添加 PikPak 账号」**：
- **账号备注**：为该账号指定易于识别的名称（如“美西1号”、“香港备用号”）。
- **认证方式**：
  - 方式 A：输入 PikPak 注册用户名/邮箱/手机号及密码。
  - 方式 B：直接输入 PikPak `Refresh Token`（免账密授权）。
- **独立网络代理 (可选)**：
  - 支持 `socks5://127.0.0.1:10808`、`http://proxy:8080` 等格式。
  - 可先点击“测试独立代理”测试出口 IP 和连通延迟。
- **调度优先级**：数值越大越优先被调度器选中。

### 3. 多账号离线下载
进入 **「离线任务」** 或点击任意页面右上角 **「新建离线下载」**：
- 支持单条或多条输入（每行一条）。
- 兼容 Magnet 磁力链接、HTTP/HTTPS 直链、ED2K 电驴链接等。
- 提交后系统将在后台自动进行多账号分配。

### 4. 视频播放与 Range 支持
在 **「统一网盘」** 文件列表中点击任意视频：
- 系统自动呼出内置在线视频播放器。
- 支持自由拖拽进度条（毫秒级 206 Partial Content 中转）。
- 可在右上角切换 **Proxy 代理流** 或 **Direct 直连**。
- 系统会在浏览器端自动记录各个视频的历史播放进度。

---

## 🔌 外部 REST API 调用

系统提供规范的 RESTful API，方便与外部自动化下载脚本、MoviePilot、Nastool 等系统集成。

### 1. 生成 API Key
在 Web 后台 **「API 密钥」** 页面生成 Key，获得形如 `pk_abcdef123456...` 的凭据。

### 2. 提交离线任务 (支持幂等)
```bash
curl -X POST http://localhost:8080/api/v1/offline \
  -H "Authorization: Bearer pk_your_api_key_here" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: custom-uuid-unique-key-12345" \
  -d '{
    "url": "magnet:?xt=urn:btih:3b827e8a9391fe785f38b61a06e541bca358b633",
    "name": "Ubuntu.iso"
  }'
```

**响应示例：**
```json
{
  "success": true,
  "task_id": "b28842f3-91be-4e72-a856-d9680cc81c82",
  "status": "RUNNING",
  "account": "Account-A"
}
```

### 3. 批量提交离线任务
```bash
curl -X POST http://localhost:8080/api/v1/offline \
  -H "Authorization: Bearer pk_your_api_key_here" \
  -H "Content-Type: application/json" \
  -d '{
    "urls": [
      "magnet:?xt=urn:btih:1111111111111111111111111111111111111111",
      "magnet:?xt=urn:btih:2222222222222222222222222222222222222222"
    ]
  }'
```

### 4. 查询任务状态
```bash
curl -X GET http://localhost:8080/api/v1/offline/b28842f3-91be-4e72-a856-d9680cc81c82 \
  -H "Authorization: Bearer pk_your_api_key_here"
```

**响应示例：**
```json
{
  "id": "b28842f3-91be-4e72-a856-d9680cc81c82",
  "source_url": "magnet:?xt=urn:btih:...",
  "file_name": "Ubuntu.iso",
  "account_id": 1,
  "account_name": "Account-A",
  "pikpak_task_id": "987654321",
  "status": "COMPLETE",
  "progress": 100,
  "created_at": "2026-09-14T12:00:00Z"
}
```

### 5. 查询账号池运行状态
```bash
curl -X GET http://localhost:8080/api/v1/accounts/status \
  -H "Authorization: Bearer pk_your_api_key_here"
```

*(注意：此接口自动屏蔽敏感密码和 Token，仅返回账号健康状态、今日使用次数及空间信息)*

---

## 🔒 安全性设计

1. **数据库凭据零明文**：管理员密码使用 bcrypt 进行单向加盐哈希，不可逆运算。PikPak 账号密码、Access Token、Refresh Token 均采用 AES-256-GCM 算法加密存储。
2. **日志自动脱敏**：日志系统自动对 Magnet 哈希、密码、Token 进行掩码处理（如 `abc***xyz`），杜绝因日志输出造成的安全泄露。
3. **Session Cookie 安全**：Web 端会话使用 `HttpOnly`, `SameSite=Lax`, 可选 `Secure` 属性，防范 XSS 与 CSRF 攻击。
4. **API Key SHA-256 存储**：外部 API Key 仅在生成时对管理员展示一次，数据库中仅存储 SHA-256 摘要与前缀。

---

## 💾 数据备份与升级

所有数据、数据库文件和配置均统一存放于 `/data` 目录（容器外部对应挂载目录 `./data`）。

- **备份**：直接复制或归档 `./data` 目录即可：
  ```bash
  tar -czvf pikpak_backup_$(date +%Y%m%d).tar.gz ./data
  ```
- **升级**：
  ```bash
  docker compose pull
  docker compose up -d
  ```
  内置的数据库 Migration 机制将在启动时自动检查并无缝应用最新的表结构迁移。

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 开源。
