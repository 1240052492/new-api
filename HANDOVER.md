# New API 项目交接文档

> 最后更新：2026-10-06
>
> 本文是下一次会话、维护人员和发布操作的入口。先读取本文件，再读取 `AGENTS.md`、`STATE.md`，最后按任务需要读取 `docs/operations/` 与 `docs/superpowers/plans/` 下的记录。

## 一、项目目标

在 HNY 服务器上以官方方式运行 New API 生产实例，并在用户 fork 仓库 `1240052492/new-api` 上进行后续功能开发。首次生产基线必须保持官方上游版本，不得用 fork 的未验证 `main` 替换。

## 二、生产现状

- 服务器：HNY，使用已配置的 `ssh HNY` 连接。
- 系统：Ubuntu Server 24.04，8 vCPU，约 7.8 GiB RAM。
- 生产域名：`https://aiplantform.site`。
- 备用访问域名：`https://www.aiplantform.site`。
- 测试域名：`test.aiplantform.site`，仅预留，尚未部署测试环境。
- 服务器应用目录：`/opt/new-api`。
- 反向代理：宝塔 Nginx。
- 数据库：官方 Compose 内置 PostgreSQL 15，持久卷 `pg_data`。
- 缓存：官方 Compose 内置 Redis。
- New API：官方镜像 `calciumion/new-api:v1.0.0-rc.41`。
- New API 监听：仅 `127.0.0.1:3000`，公网通过 Nginx HTTPS 访问。
- 管理员：已由用户创建。
- 初始安全设置：用户已确认完成。
- 上游渠道：尚未在后台录入。

## 三、版本基线

首次部署使用官方上游 `QuantumNous/new-api` 的 `v1.0.0-rc.41`，对应源码提交以服务器 `/opt/new-api` 中的 Git 记录和容器镜像 OCI revision 为准。生产不得跟随 `latest`、移动分支或服务器现场编译。

当前生产容器应保持以下 Compose 服务：

- `new-api`
- `postgres`
- `redis`

生产发布必须使用固定 Git commit、不可复用的 release tag、固定镜像 tag/digest，并保留上一版本用于应用回滚。

## 四、已完成事项

1. 已完成 HNY 只读预检、Docker/Compose 检查和端口检查。
2. 已按官方 Compose 拓扑部署 New API、PostgreSQL、Redis。
3. 已替换默认数据库、Redis 和 Session 凭据；实际值不写入仓库、不发送到聊天。
4. 已配置 `aiplantform.site` 与 `www.aiplantform.site` 的 Nginx HTTPS 反代。
5. 已签发覆盖两个生产域名的 Let's Encrypt 证书，并配置自动续期。
6. 已将应用端口限制为 `127.0.0.1:3000`。
7. 已验证两个 HTTPS `/api/status` 返回 HTTP 200，版本为 `v1.0.0-rc.41`。
8. 已验证 New API 容器为 `healthy`，公网直连 3000 端口失败。
9. 已创建管理员并完成第一轮安全设置。

## 五、当前待办

### 5.1 首个上游渠道

用户计划接入 Sub2API 网关：

- Base URL：`https://code.shoestravel.xin`
- New API 渠道类型：`Sub2API`（内置类型 59）
- Base URL 不要追加 `/v1`。
- API Key 只能由用户在 New API 后台填写，禁止写入文档或聊天。

候选模型如下，必须先通过认证后的上游 `/v1/models` 清单和渠道测试确认：

- OpenAI：`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-image-2`、`gpt-image-2.5`
- Anthropic：`claude-opus-5`、`claude-opus5-5`
- Gemini：`gemini-3.1-pro`、`gemini-3.5-flash`
- Grok：`grok-4.6`、`grok-4.7`

未经过 `/v1/models` 和真实请求验证的模型，不得发布给用户。上游模型 ID 不一致时，使用显式模型映射，不要伪造模型名。

### 5.2 生产验证

渠道配置完成后，按以下顺序验证：

1. `/v1/models` 能返回已发布模型。
2. 一个非流式请求成功。
3. 一个流式请求成功。
4. 无效 API Key 返回预期认证错误。
5. 使用量日志、扣费和倍率结果符合配置。

凭据、响应中的密钥、完整 API Key 和恢复码只能保留在用户控制的安全位置。

### 5.3 测试环境

`test.aiplantform.site` 已保留给测试环境。部署前必须使用独立的：

- Compose project
- 应用容器
- PostgreSQL 数据卷
- Redis 实例或隔离命名空间
- 日志目录和非生产凭据

测试环境不得指向生产数据库、生产 Redis、生产域名或生产密钥。

## 六、开发、测试、生产代码管理

代码来源为用户 fork：`1240052492/new-api`。

### 开发

- 从 fork 的 `develop` 创建 `feature/<name>` 或 `fix/<name>`。
- 功能开发、单元测试和代码审查在 feature 分支完成。
- 不直接修改生产 checkout，不把 `develop` 部署到生产。

### 测试

- 从 `develop` 切出 `release/<version>` 或 `rc/<version>`。
- 用候选 commit 构建不可变镜像。
- 部署到隔离的 `test.aiplantform.site` 环境。
- 完成功能、认证、计费、迁移、回滚和安全验收。

### 生产

- 测试通过后，在精确批准的 commit 上创建不可移动 release tag。
- 生产使用该 tag 对应的镜像 digest。
- 生产不执行 `git pull`、不现场编译、不跟踪移动分支。
- 每次发布记录 commit、tag、镜像 digest、备份、迁移状态、健康检查和回滚结果。

详细方案见 `docs/superpowers/plans/2026-10-05-hny-new-api-production-and-development.md`。

## 七、回滚与备份

已存在的服务器备份：

- Nginx：`/opt/new-api-backups/nginx-20261005-154213.tgz`
- Compose：`/opt/new-api-backups/docker-compose-20261006-142554.yml`

应用回滚和数据库恢复必须分开处理：

- 应用异常：切回上一个固定镜像，执行 `docker compose up -d --no-deps new-api`，再验证健康和公网 API。
- 数据库恢复：只有确认数据损坏或不可逆迁移时才执行；必须先备份、安排维护窗口并获得明确批准。
- 不因普通应用容器异常自动恢复数据库备份。

## 八、关键路径

- 当前状态：`STATE.md`
- 生产部署记录：`docs/operations/hny-deployment-2026-10-05.md`
- 应用配置记录：`docs/operations/hny-application-configuration.md`
- 开发/发布方案：`docs/superpowers/plans/2026-10-05-hny-new-api-production-and-development.md`
- HNY Compose：服务器 `/opt/new-api/docker-compose.yml`
- Nginx vhost：服务器 `/www/server/panel/vhost/nginx/aiplantform.site.conf`
- 证书目录：服务器 `/www/server/panel/vhost/cert/aiplantform.site/`

## 九、接手操作顺序

1. 读取根目录 `AGENTS.md`、`STATE.md` 和本 `HANDOVER.md`。
2. 通过 `ssh HNY` 检查 `docker compose ps`、监听端口和公网 HTTPS 状态。
3. 不读取、打印或索要任何秘密值。
4. 先完成 Sub2API 渠道创建和模型发现，再做模型测试。
5. 更新 `docs/operations/hny-application-configuration.md` 和 `STATE.md`，记录结果但不记录凭据。
6. 测试环境和新功能开发必须遵守本文的三阶段分支与发布规则。

## 十、禁止事项

- 不得把密码、API Key、Session Secret、私钥、2FA 恢复码写入 Git、文档或聊天。
- 不得删除或覆盖生产 PostgreSQL 数据卷。
- 不得把 `test.aiplantform.site` 指向生产数据。
- 不得把未经测试的 fork `main`、`develop` 或 `latest` 镜像部署到生产。
- 不得移除 New API 与 QuantumNous 的原始归属、许可证和项目标识。
