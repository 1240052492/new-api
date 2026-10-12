# 本地开发环境设置

## 前置要求

- **Docker** 和 **Docker Compose** — 用于 PostgreSQL 容器化
- **Go 1.25.1+** — 后端编译和运行
- **Node.js 24+** 和 **Bun** — 前端开发
- **Git** — 版本控制

## 快速开始

### 1. 克隆代码并同步到最新

```bash
# 如果还没有克隆
git clone https://github.com/1240052492/new-api.git
cd new-api

# 确保本地代码同步到最新
git fetch upstream main
git checkout main
git reset --hard upstream/main

# 创建或切换到 develop 分支
git checkout develop
git pull origin develop
```

### 2. 配置本地环境变量

```bash
# 复制模板
cp .env.example .env.local

# 编辑 .env.local，填入本地配置（参考下面的配置示例）
vim .env.local
```

**.env.local 配置示例:**

```
# 数据库 - PostgreSQL（从 docker-compose 容器）
SQL_DSN=postgres:localpass123@127.0.0.1:5432/new_api_dev?sslmode=disable
LOG_SQL_DSN=postgres:localpass123@127.0.0.1:5433/new_api_log?sslmode=disable

# 服务
PORT=3000
LOG_LEVEL=info
DEFAULT_LANGUAGE=zh-CN

# Redis（可选，如果使用内存缓存可忽略）
# REDIS_CONN_STRING=redis://localhost:6379/0

# OAuth 配置（可选，使用真实值或留空）
GITHUB_CLIENT_ID=your_github_client_id_here
GITHUB_CLIENT_SECRET=your_github_client_secret_here

# 邮件配置（可选）
# MAIL_SMTP_HOST=smtp.gmail.com
# MAIL_SMTP_PORT=587
# MAIL_SMTP_USERNAME=your_email@gmail.com
# MAIL_SMTP_PASSWORD=your_app_password
```

### 3. 启动 PostgreSQL 容器

```bash
# 启动容器
docker-compose up -d

# 验证容器运行
docker ps | grep new-api-postgres

# 验证数据库连接（使用上面 .env.local 中的密码）
PGPASSWORD=localpass123 psql -h localhost -U postgres -d postgres -c "SELECT 1;"
```

Expected output: `1` (连接成功)

### 4. 创建本地数据库（首次）

使用项目内提供的数据库初始化脚本，或通过应用自动迁移：

```bash
# 如果项目提供了初始化脚本
# bash scripts/init-db.sh

# 或者通过应用启动时自动迁移（大多数情况）
```

### 5. 启动后端服务

```bash
# 后端编译和运行
go run main.go

# 或构建后运行
go build -o new-api
./new-api

# 预期输出：服务监听在 http://localhost:3000
```

### 6. 启动前端开发服务器（新终端）

```bash
cd web

# 安装依赖（首次）
bun install

# 启动开发服务器
bun run dev

# 预期输出：前端运行在 http://localhost:5173（或类似端口）
```

### 7. 验证应用

打开浏览器访问：

- **后端 API**: `http://localhost:3000`
- **前端**: `http://localhost:5173` (或显示的端口)
- **API 健康检查**: `curl http://localhost:3000/api/health`

## 常用命令

### 停止开发环境

```bash
# 停止容器但保留数据
docker-compose down

# 或停止所有容器
docker stop new-api-postgres new-api-postgres-log
```

### 重置数据库（清空所有数据）

```bash
# 停止并删除容器和卷
docker-compose down -v

# 重新启动（重新初始化数据库）
docker-compose up -d
```

### 查看容器日志

```bash
# 主数据库日志
docker logs -f new-api-postgres

# 日志数据库日志
docker logs -f new-api-postgres-log

# 所有日志
docker-compose logs -f
```

### 直接访问 PostgreSQL CLI

```bash
# 连接到主数据库
PGPASSWORD=localpass123 psql -h localhost -U postgres -d new_api_dev

# 连接到日志数据库
PGPASSWORD=localpass123 psql -h localhost -p 5433 -U postgres -d new_api_log
```

## 故障排查

### 问题：端口已被占用

```bash
# 检查端口占用情况
lsof -i :5432  # PostgreSQL 主库
lsof -i :5433  # PostgreSQL 日志库
lsof -i :3000  # 后端服务

# 修改 docker-compose.yml 中的端口映射
# 或杀死占用进程：
kill -9 <PID>
```

### 问题：无法连接到 PostgreSQL

```bash
# 检查容器是否运行
docker ps | grep postgres

# 重启容器
docker-compose restart

# 检查容器健康状态
docker inspect new-api-postgres | grep -A 10 '"Health"'
```

### 问题：Go 编译失败

```bash
# 清理模块缓存
go clean -modcache

# 重新下载依赖
go mod download

# 或重新初始化
go mod tidy
```

## 提交代码时的注意事项

- ✅ **提交** `.env.example` — 配置模板
- ❌ **不提交** `.env.local` — 本地敏感数据（已在 .gitignore 中）
- ✅ **提交** `docker-compose.yml` — 开发环境配置
- ✅ **提交** 项目代码

验证：

```bash
# 检查 .env.local 是否被 git 追踪
git status | grep .env

# 应该无输出（或只显示 .env.example）
```

## 更多信息

- 数据库配置：见 `DEPLOYMENT.md`
- 部署流程：见 `DEPLOYMENT.md`
- 升级官方版本：见 `UPGRADE_CHECKLIST.md`
