# 部署流程文档

## 版本管理策略

### 分支规划

- **main 分支** — 生产环境代码，每个提交对应一个 tag，部署到服务器
- **develop 分支** — 开发分支，本地工作在这里
- **upstream/main** — 官方最新代码，定期同步

### Tag 命名规范

格式：`v1.0.0-aiplantform-YYYYMMDD-N`

- `1.0.0` — 版本号（参考官方版本，如 rc.44、rc.43 等）
- `aiplantform` — 项目标识
- `YYYYMMDD` — 发布日期（例如 20250112）
- `N` — 该日期的第 N 个发布版本

**示例：**
```
v1.0.0-aiplantform-20250112-1  # 2025年1月12日第一次发布
v1.0.0-aiplantform-20250112-2  # 2025年1月12日第二次发布（同日热修复）
v1.0.0-aiplantform-20250113-1  # 2025年1月13日第一次发布
```

## 本地开发流程

### 日常开发

1. **更新本地代码**

```bash
git checkout develop
git pull origin develop
```

2. **本地开发和测试**

```bash
# 在 develop 分支上开发
# 使用 docker-compose 启动 PostgreSQL
docker-compose up -d

# 启动后端
go run main.go

# 新终端启动前端
cd web && bun run dev

# 在 http://localhost:3000 和 http://localhost:5173 测试
```

3. **提交代码**

```bash
git add .
git commit -m "feat: 描述你的改动

详细说明（可选）:
- 修改了哪些文件
- 为什么要改
- 有没有风险点"

git push origin develop
```

### 代码审查（可选）

由于是单人维护，通常不需要 PR。如果有特殊需求可以自己创建 PR 做最后检查。

## 部署到生产环境

### 前置条件检查

- [ ] 在本地 develop 分支上的代码已充分测试
- [ ] 所有新功能已通过本地 SQLite 和 Docker PostgreSQL 验证
- [ ] 无未提交的更改
- [ ] git status 输出为干净状态

```bash
cd /path/to/new-api
git status
# Expected: 工作目录干净，无未跟踪文件
```

### 部署步骤

#### 第一步：准备 main 分支（本地执行）

```bash
git checkout main
git pull origin main

# 验证 main 分支是否与 develop 同步（如果第一次部署）
# 或检查是否需要合并 develop
git log --oneline main -1
```

#### 第二步：合并 develop 到 main（本地执行）

```bash
git merge develop --no-ff -m "Release: 合并 develop 到 main"

# --no-ff 确保创建一个合并提交，保持历史清晰
```

#### 第三步：创建版本 Tag（本地执行）

```bash
# 获取当前日期（YYYYMMDD 格式）
DATE=$(date +%Y%m%d)

# 检查是否已经有该日期的 tag
git tag -l "v1.0.0-aiplantform-${DATE}-*" | sort

# 确定版本号（如果已有多个，则递增）
# 例如如果已有 -1 和 -2，则新创建 -3

TAG="v1.0.0-aiplantform-${DATE}-1"  # 通常从 -1 开始，除非同日多次发布

# 创建带注解的 tag（推荐）
git tag -a "$TAG" -m "Release $TAG - new-api production deployment"

# 验证 tag 创建成功
git tag -l | grep "$TAG"
git show "$TAG"
```

#### 第四步：推送代码和 Tag 到 GitHub（本地执行）

```bash
git push origin main
git push origin "$TAG"

# 验证推送成功
# 访问 https://github.com/1240052492/new-api/releases 查看 tag
```

#### 第五步：服务器部署（在服务器上执行）

```bash
# 1. 连接到服务器
ssh user@your-server-ip

# 2. 进入项目目录
cd /opt/new-api

# 3. 停止当前服务
sudo systemctl stop new-api
# 或如果使用 Docker：
# docker stop new-api-container

# 4. 拉取最新代码和 tag
git fetch origin main
git fetch origin --tags

# 5. 检出新的 tag
git checkout "$TAG"  # 例如：v1.0.0-aiplantform-20250112-1

# 6. 确保 .env.production 文件存在且配置正确
# 如果没有，根据 .env.example 创建
if [ ! -f .env.production ]; then
    cp .env.example .env.production
    echo "⚠️  Created .env.production, please edit it with production values"
    exit 1
fi

# 7. 重新编译/构建（根据项目类型选择）

# Go 后端（如果需要重新编译）
go build -o new-api-bin main.go

# 或使用 Docker（如果有 Dockerfile）
docker build -t new-api:$TAG .

# 8. 启动服务
sudo systemctl start new-api
# 或 Docker:
# docker run -d --name new-api-container --env-file .env.production -p 3000:3000 new-api:$TAG

# 9. 验证服务启动
sleep 3
sudo systemctl status new-api
# 或 Docker:
# docker ps | grep new-api

# 10. 测试 API 可用性
curl http://localhost:3000/api/health
# Expected: 返回健康检查状态，通常是 {"status":"ok"} 或类似

# 11. 查看日志确认无错误
sudo journalctl -u new-api -f  # 最近 20 行
# 或 Docker:
# docker logs -f new-api-container
```

#### 第六步：验证部署成功（服务器上执行）

```bash
# 检查服务状态
sudo systemctl status new-api

# 检查应用日志（最近 50 行）
sudo journalctl -u new-api -n 50

# 测试核心 API 端点
curl http://localhost:3000/api/user/me \
  -H "Authorization: Bearer YOUR_VALID_TOKEN"

# 检查部署的版本
git describe --tags

# 确保 .env.production 文件不会被 git 追踪
git status | grep ".env"
# Expected: 无输出
```

---

## 回滚流程

如果部署后发现问题，快速回滚到上一个版本：

### 快速回滚（生产服务器上执行）

```bash
cd /opt/new-api

# 1. 停止当前服务
sudo systemctl stop new-api

# 2. 查看最近的几个 tag
git tag -l "v1.0.0-aiplantform-*" | sort -V | tail -5

# 3. 切换到前一个 tag
# 例如从 v1.0.0-aiplantform-20250112-1 回滚到 v1.0.0-aiplantform-20250111-1
git checkout v1.0.0-aiplantform-20250111-1

# 4. 重启服务
sudo systemctl start new-api

# 5. 验证回滚成功
curl http://localhost:3000/api/health

# 6. 查看现在的版本
git describe --tags
```

### 完整回滚日志

```bash
# 查看完整的部署历史
git log --oneline -20

# 查看特定 tag 的详细信息
git show v1.0.0-aiplantform-20250111-1

# 对比两个版本的差异
git log v1.0.0-aiplantform-20250111-1..v1.0.0-aiplantform-20250112-1 --oneline
```

---

## 配置管理

### 本地配置（.env.local）

- 不进 git，放在项目根目录
- 包含本地开发的 API key、数据库信息等

**创建方法：**

```bash
cp .env.example .env.local
# 编辑 .env.local，填入本地值
```

### 生产配置（.env.production）

- 不进 git，只存在于生产服务器
- 包含实际的数据库密码、API key、OAuth 秘密等

**创建方法（服务器上首次执行）：**

```bash
cd /opt/new-api
cp .env.example .env.production
# 编辑 .env.production，填入生产值
chmod 600 .env.production  # 只有所有者能读写
```

**安全建议：**

```bash
# 设置限制权限
chmod 600 .env.production

# 验证只有 new-api 进程和 root 能访问
ls -la .env.production
```

---

## 常用故障排查

### 问题 1：部署后应用无法启动

```bash
# 检查 .env.production 是否存在
[ -f .env.production ] && echo "Exists" || echo "Missing"

# 检查数据库连接
grep "SQL_DSN\|DATABASE" .env.production

# 测试数据库连接
PGPASSWORD=your_db_pass psql -h your_db_host -U your_db_user -d your_db_name -c "SELECT 1;"

# 查看应用日志
sudo journalctl -u new-api -n 100
```

### 问题 2：版本未更新（仍运行旧版本）

```bash
# 确认当前版本
git describe --tags

# 确认进程运行的是哪个二进制
ps aux | grep new-api

# 检查二进制文件的编译时间
stat /path/to/new-api-bin | grep Modify

# 确认进程确实已停止并重启
sudo systemctl stop new-api
sleep 2
ps aux | grep new-api  # 应该没有进程
sudo systemctl start new-api
sleep 2
ps aux | grep new-api  # 应该看到新进程
```

### 问题 3：敏感数据意外泄露

```bash
# 检查 .env.production 是否被 git 追踪
git ls-files | grep ".env"
# 应该只看到 .env.example

# 检查历史中是否有敏感数据
git log -p --all -- .env | grep "PASSWORD\|SECRET\|API_KEY" | head -5

# 如果发现泄露，立即更换所有密钥并从 git 历史中清除
# （这是严重问题，需要特殊处理）
```

---

## 自动化部署（可选）

如果后续想要自动化部署，可以：

1. 在本地创建 `scripts/deploy.sh` 脚本（已在 Task 4 中提供）
2. 在服务器配置 webhook 或 CI/CD
3. 使用 GitHub Actions 自动构建和测试

目前手动部署足够简单。

---

## 检查清单

部署前最后检查：

- [ ] `git status` 输出为干净（无未提交更改）
- [ ] `develop` 分支已推送到 GitHub
- [ ] 本地完整测试通过（包括 Docker PostgreSQL 验证）
- [ ] `.env.production` 在服务器上已准备
- [ ] 备份当前生产数据（可选但推荐）
- [ ] Tag 已创建并推送到 GitHub
- [ ] 服务器网络连接正常

部署后验证：

- [ ] `git describe --tags` 显示正确的版本
- [ ] `curl http://localhost:3000/api/health` 返回 200
- [ ] 应用日志无 ERROR
- [ ] 核心功能测试通过（登录、API 调用等）
- [ ] `.env.production` 不在 git 中

---
