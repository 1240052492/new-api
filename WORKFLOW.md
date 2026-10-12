# 开发和部署工作流指南

快速参考：所有操作对照此指南

---

## 快速参考

### 初始化本地环境

```bash
# 第一次设置本地开发环境
git clone https://github.com/1240052492/new-api.git
cd new-api

# 同步到最新代码
git fetch upstream main
git checkout main
git reset --hard upstream/main

# 创建 develop 分支
git checkout -b develop
git push -u origin develop

# 复制本地配置（不会进 git）
cp .env.example .env.local
# 编辑 .env.local，填入本地 API key 等

# 启动 PostgreSQL 容器
docker-compose up -d

# 验证数据库连接
PGPASSWORD=localpass123 psql -h localhost -U postgres -c "SELECT 1;"
```

---

### 日常开发

```bash
# 1. 更新 develop 分支到最新
git checkout develop
git pull origin develop

# 2. 启动开发环境
docker-compose up -d

# 3. 启动后端（终端 1）
go run main.go

# 4. 启动前端（终端 2，新开）
cd web && bun run dev

# 5. 开发和测试（访问 http://localhost:3000）
# ... 你的开发工作 ...

# 6. 提交代码
git add .
git commit -m "feat: 你的改动说明

可选的详细说明"

git push origin develop

# 7. 停止开发环境
docker-compose down
```

---

### 部署到生产

```bash
# 1. 确保所有更改已推送到 develop
git checkout develop
git status  # 应该显示 "nothing to commit"

# 2. 自动化准备发布（推荐）
./scripts/deploy.sh

# 或手动执行：
# git checkout main
# git merge develop --no-ff -m "Release"
# git tag -a v1.0.0-aiplantform-YYYYMMDD-1 -m "Release"
# git push origin main && git push origin --tags

# 3. 在服务器上部署
ssh user@server
cd /opt/new-api
git fetch origin main --tags
git checkout v1.0.0-aiplantform-YYYYMMDD-1  # 使用实际的 tag
sudo systemctl restart new-api

# 4. 验证
curl http://localhost:3000/api/health
```

---

### 升级官方版本

```bash
# 1. 拉取官方最新代码
git fetch upstream main

# 2. 评估改动
git log main..upstream/main --oneline | head -10

# 3. 本地合并和测试
git checkout develop
git merge upstream/main --no-ff -m "chore: update from upstream"

# 启动开发环境测试
docker-compose up -d
go run main.go

# 完整测试通过后...

# 4. 部署到生产
./scripts/deploy.sh
```

---

### 出现问题时回滚

```bash
# 快速回滚到上一个版本
cd /opt/new-api
sudo systemctl stop new-api

git tag -l | sort -V | tail -3  # 查看最近 3 个版本
git checkout v1.0.0-aiplantform-20250111-1  # 切换到上一个版本

sudo systemctl start new-api
curl http://localhost:3000/api/health  # 验证
```

---

## 工作流架构图

```
┌─────────────────────────────────────────────────────────┐
│                    GitHub 官方                            │
│              upstream/main (最新代码)                    │
└──────────────────────┬──────────────────────────────────┘
                       │ git fetch upstream
                       │
┌─────────────────────┴──────────────────────────────────┐
│            你的 Fork (1240052492/new-api)              │
│                                                         │
│  ┌─ main (生产版本，每个 commit 对应一个 tag)          │
│  │  v1.0.0-aiplantform-20250112-1                      │
│  │  ↑ 来自 develop merge，对应服务器部署版本            │
│  │                                                     │
│  └─ develop (开发分支)                                  │
│     ↑ 日常开发工作在这里                               │
│     ↑ 充分测试后 merge 到 main                          │
│                                                         │
└─────────────────────┬──────────────────────────────────┘
                      │ git push
                      │
        ┌─────────────┴──────────────┐
        ↓                            ↓
┌──────────────┐         ┌──────────────────┐
│ 本地开发环境 │         │  生产服务器      │
│              │         │  /opt/new-api    │
│ .env.local   │         │  .env.production │
│ SQLite       │         │  PostgreSQL      │
│ Docker PG    │         │  main 分支       │
│ go run main  │         │  systemctl       │
└──────────────┘         └──────────────────┘
```

---

## 配置和安全

### 敏感数据管理

| 文件 | 位置 | 进 Git | 用途 |
|------|------|--------|------|
| `.env.example` | 项目根 | ✅ | 配置模板 |
| `.env.local` | 项目根 | ❌ | 本地开发配置 |
| `.env.production` | 服务器 | ❌ | 生产配置 |

**重要：敏感数据（API key、密码）永远不要进 git！**

```bash
# 验证敏感文件不在 git 中
git status | grep .env
# 应该无输出或只显示 .env.example
```

---

## 分支说明

| 分支 | 用途 | 部署对象 | 维护者 |
|------|------|----------|--------|
| `main` | 生产稳定版本 | 生产服务器 | 自动（develop merge） |
| `develop` | 开发分支 | 本地开发 | 你（日常工作） |
| `upstream/main` | 官方最新 | 参考 | 官方维护 |

---

## 常用命令速查

### Git 相关

```bash
# 查看当前分支和状态
git branch -a
git status

# 更新代码
git pull origin develop
git fetch upstream main

# 切换分支
git checkout main
git checkout develop

# 提交代码
git add .
git commit -m "feat: ..."
git push origin develop

# 查看版本历史
git log --oneline -10
git describe --tags

# 查看分支差异
git log main..develop --oneline
```

### Docker 相关

```bash
# 启动容器
docker-compose up -d

# 停止容器
docker-compose down

# 查看日志
docker-compose logs -f postgres

# 重置数据库
docker-compose down -v
docker-compose up -d
```

### 服务器相关

```bash
# SSH 连接
ssh user@server

# 查看服务状态
sudo systemctl status new-api

# 查看日志
sudo journalctl -u new-api -f

# 重启服务
sudo systemctl restart new-api

# 检查版本
git describe --tags
cd /opt/new-api && git log -1 --oneline
```

---

## 文档导航

- **本地开发** — 详见 `LOCAL_SETUP.md`
- **部署流程** — 详见 `DEPLOYMENT.md`
- **版本升级** — 详见 `UPGRADE_CHECKLIST.md`
- **故障排查** — 详见 `DEPLOYMENT.md` 中的"故障排查"章节

---

## 常见场景

### 我想修改代码

```bash
git checkout develop
# 修改代码...
git add .
git commit -m "feat: ..."
git push origin develop

# 本地测试通过后，部署到生产
./scripts/deploy.sh
```

### 我想应用官方的最新修复

```bash
git fetch upstream main
git checkout develop
git merge upstream/main --no-ff
# 本地测试...
./scripts/deploy.sh
```

### 我不小心提交了敏感数据

```bash
# 立即撤销提交
git reset HEAD~1

# 编辑 .gitignore，确保敏感文件被隐藏
# 删除或隐藏敏感文件

# 重新提交
git add .
git commit -m "fix: remove sensitive data"
```

### 生产环境出现问题需要回滚

```bash
# 在服务器上
cd /opt/new-api
git checkout v1.0.0-aiplantform-20250111-1  # 上一个版本

sudo systemctl restart new-api
curl http://localhost:3000/api/health
```

---

## 技术栈速览

| 组件 | 技术 | 端口 | 用途 |
|------|------|------|------|
| 后端 | Go 1.25 | 3000 | API 服务 |
| 前端 | React 19 + Bun | 5173 | Web 界面 |
| 数据库 | PostgreSQL 16 | 5432 | 主数据库 |
| 日志库 | PostgreSQL 16 | 5433 | 日志存储 |
| 容器 | Docker | — | 本地开发 |

---

## 获取帮助

- 官方文档：https://docs.newapi.ai/
- GitHub Issues：https://github.com/QuantumNous/new-api/issues
- 查看应用日志：`sudo journalctl -u new-api -f`

---

**最后更新：** 2025-01-12
**维护者：** @1240052492
