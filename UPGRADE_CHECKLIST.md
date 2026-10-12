# 官方版本升级检查清单

## 什么时候升级？

- ✅ 官方发布新的 RC 版本或稳定版本
- ✅ 新版本包含你需要的功能或 bug fix
- ✅ 当前版本有安全问题
- ❌ 避免频繁升级导致不稳定

## 升级前的准备

- [ ] 记录当前的版本信息
  ```bash
  git describe --tags  # 显示当前版本 tag
  git log -1 --oneline  # 显示最新 commit
  ```

- [ ] 备份生产数据库（可选但推荐）
  ```bash
  # 在服务器上
  pg_dump -Fc new_api_dev > backup_$(date +%Y%m%d_%H%M%S).dump
  ```

- [ ] 确保本地 develop 分支干净
  ```bash
  git checkout develop
  git status
  # Expected: 工作目录干净
  ```

---

## 升级步骤

### 第一步：获取官方最新代码（本地执行）

```bash
# 1. 更新本地 upstream 信息
git fetch upstream main

# 2. 查看官方最新版本信息
git log upstream/main -5 --oneline

# 3. 查看 release notes（可选）
# 访问 https://github.com/QuantumNous/new-api/releases
```

- [ ] 官方代码已拉取到本地

### 第二步：评估更改（本地执行）

```bash
# 1. 比较当前版本和新版本的差异
git log main..upstream/main --oneline | head -20

# 2. 查看具体的 commit 变化
git diff main..upstream/main --stat

# 3. 检查是否有 breaking changes（通常在 GitHub release notes 中）
# 访问 https://github.com/QuantumNous/new-api/releases
```

检查清单：

- [ ] 查看了官方 release notes
- [ ] 确认没有数据库 schema 破坏性变更
- [ ] 确认没有 API 不兼容的改动
- [ ] 评估了新功能是否需要（必需？可选？）

### 第三步：本地集成（本地执行）

```bash
# 1. 切换到 develop 分支
git checkout develop
git pull origin develop

# 2. 将 upstream main 合并到 develop
git merge upstream/main --no-ff -m "chore: update from upstream to latest main"

# 3. 处理可能的冲突
# 通常不会有冲突，因为你不改源代码
# 如果有冲突，解决后：
git add .
git commit -m "chore: resolve merge conflicts with upstream"
```

- [ ] 本地 develop 分支已合并 upstream/main
- [ ] 合并冲突已解决（如果有）

### 第四步：本地测试（本地执行）

```bash
# 1. 启动本地开发环境
docker-compose up -d

# 2. 验证后端能够启动
go run main.go

# 3. 验证前端能够启动（新终端）
cd web && bun run dev

# 4. 运行基础功能测试
# - 访问 http://localhost:3000
# - 测试登录功能
# - 测试核心 API 调用
# - 检查是否有明显错误

# 5. 查看日志是否有 ERROR 或 WARN
# tail -f 后端日志、浏览器控制台等
```

测试清单：

- [ ] 后端能够成功启动（`go run main.go`）
- [ ] 前端能够成功启动（`bun run dev`）
- [ ] 应用主页能够加载
- [ ] 用户登录功能正常
- [ ] API 调用返回正确结果
- [ ] 后端日志无 ERROR（可能有 WARN）
- [ ] 前端控制台无明显错误

### 第五步：Staging 验证（可选）

如果在本地遇到问题或想要额外的验证，可以在 staging 环境测试：

```bash
# 在服务器上创建 staging 分支
git checkout staging  # 如果没有则 git checkout -b staging

# 或临时部署到一个不同端口的实例
docker run -p 3001:3000 --env-file .env.staging new-api:latest
```

### 第六步：部署到生产（本地 + 服务器执行）

如果本地测试通过，按照 `DEPLOYMENT.md` 中的步骤部署：

```bash
# 1. 本地准备（develop 已测试无误）
./scripts/deploy.sh
# 或手动执行：
# git checkout main
# git merge develop --no-ff
# git tag -a v1.0.0-aiplantform-YYYYMMDD-1 -m "Release"
# git push origin main && git push origin --tags

# 2. 服务器部署
# ssh 到服务器
cd /opt/new-api
git fetch origin main
git fetch origin --tags
git checkout v1.0.0-aiplantform-YYYYMMDD-1
sudo systemctl restart new-api

# 3. 验证
curl http://localhost:3000/api/health
```

- [ ] 代码已推送到 GitHub
- [ ] Tag 已创建并推送
- [ ] 服务器已检出新版本
- [ ] 服务已重启
- [ ] API 健康检查通过

### 第七步：生产验证（服务器执行）

```bash
# 1. 检查当前版本
git describe --tags

# 2. 检查服务状态
sudo systemctl status new-api

# 3. 查看应用日志（最近 100 行）
sudo journalctl -u new-api -n 100 | grep -i error

# 4. 测试关键功能
curl http://localhost:3000/api/health
# 访问 https://your-server.com 测试 Web 界面

# 5. 监控一段时间
# 观察日志是否有报错
# 观察系统资源使用情况
# 观察用户是否反馈问题
```

- [ ] 当前版本与预期相符
- [ ] 服务状态为 active (running)
- [ ] API 健康检查返回 200
- [ ] 应用日志无 ERROR
- [ ] 核心功能验证通过

---

## 升级失败时的回滚

如果升级后发现问题，快速回滚到上一个版本：

### 快速回滚（服务器执行）

```bash
cd /opt/new-api

# 1. 停止当前服务
sudo systemctl stop new-api

# 2. 查看最近的几个版本 tag
git tag -l "v1.0.0-aiplantform-*" | sort -V | tail -5

# 3. 切换回上一个版本
git checkout v1.0.0-aiplantform-20250111-1  # 替换为你的上一个版本

# 4. 重启服务
sudo systemctl start new-api

# 5. 验证
sleep 2
curl http://localhost:3000/api/health

# 6. 检查当前版本
git describe --tags
```

- [ ] 服务已重启
- [ ] API 恢复正常
- [ ] 版本已回滚到上一个

### 调查问题

```bash
# 查看升级后出现的 commit
git log v1.0.0-aiplantform-20250111-1..v1.0.0-aiplantform-20250112-1 --oneline

# 查看具体的代码改动
git show COMMIT_HASH

# 如果需要，可以在 develop 分支上进行修复
git checkout develop
git revert COMMIT_HASH  # 撤销有问题的 commit
# 或修复问题后重新测试
```

---

## 常见升级场景

### 场景 1：官方发布了 bug fix

```bash
# 快速升级流程：
git checkout develop
git pull origin develop
git merge upstream/main --no-ff
# 本地测试通过
./scripts/deploy.sh
```

### 场景 2：官方发布了新功能，但不是必需的

```bash
# 评估后决定暂不升级，继续使用当前版本
# 定期检查官方更新即可

git log upstream/main -1 --oneline  # 记录官方最新版本
# 后续升级时可以一次性应用多个版本的更新
```

### 场景 3：多个版本积压，需要一次性升级

```bash
# 1. 确保本地干净
git status

# 2. 一次性合并所有官方更新
git checkout develop
git merge upstream/main --no-ff -m "chore: catch up with upstream"

# 3. 充分测试（因为跨越多个版本，风险相对大一些）
docker-compose up -d
go run main.go
# 多花一些时间测试

# 4. 如果测试通过，按正常流程部署
./scripts/deploy.sh
```

---

## 升级检查表

升级前：
- [ ] 本地工作目录干净
- [ ] develop 分支最新（`git pull origin develop`）
- [ ] 了解官方更新的主要改动

升级中：
- [ ] 合并 upstream/main 到 develop
- [ ] 解决任何冲突
- [ ] 本地完整测试通过
- [ ] 推送 develop 到 GitHub

升级后（生产）：
- [ ] 代码已推送和 tag 已创建
- [ ] 服务器已检出新版本并重启
- [ ] API 健康检查通过
- [ ] 核心功能验证通过
- [ ] 监控日志是否有异常

---

## 获取帮助

如果升级过程中遇到问题：

1. **查看官方 release notes** — https://github.com/QuantumNous/new-api/releases
2. **查看官方文档** — https://docs.newapi.ai/
3. **检查项目 issue** — https://github.com/QuantumNous/new-api/issues
4. **查看应用日志**：`sudo journalctl -u new-api -f`

---

## 相关文档

- `DEPLOYMENT.md` — 完整的部署流程
- `LOCAL_SETUP.md` — 本地开发环境设置
- `WORKFLOW.md` — 工作流总结
