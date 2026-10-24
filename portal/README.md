# 智算星球 · 门户站点（aiplantform.site）

全新的 new-api 前端门户，参考 heyroute.ai / toapis.cn / relaybases.com 的设计风格与信息架构，
纯静态实现（HTML + CSS + 原生 JS），**不改动 new-api 任何源码**，与主仓库开发互不冲突。

## 一、目录结构

```
portal/
├── index.html               首页（深空 Hero + 特性 + 三步接入 + FAQ + CTA）
├── models.html              模型广场（分类筛选，数据在页面底部 MODEL_DATA 数组中维护）
├── pricing.html             定价说明（计费模式 + 倍率规则 + 价格示例表）
├── docs/
│   ├── index.html           文档中心
│   ├── quickstart.html      快速开始（获取密钥→选模型→发请求→查任务）
│   ├── api-reference.html   接口文档（认证 / Chat / Models / Images / 错误码）
│   ├── integrations.html    客户端接入（Claude Code / Codex / Cursor / GUI 客户端 / SDK）
│   └── faq.html             常见问题
├── about.html               关于我们
├── agreement.html           用户协议（基于后台条款草稿扩写的正式全文）
├── privacy.html             隐私政策（同上）
├── assets/
│   ├── css/style.css        设计系统（改一处全站生效）
│   └── js/
│       ├── site-config.js   ★ 全局单点配置：控制台域名 / API 地址 / 邮箱 / ICP / 版权
│       └── main.js          页眉页脚注入、模型筛选、代码复制；iframe 内自动 target="_top"
└── admin-snippets/
    ├── homepage-standalone.html  ★ 零部署备用：整页粘贴到后台「首页内容」即可换首页
    ├── footer.html               粘贴到后台「页脚」字段
    └── about.html                粘贴到后台「关于」字段
```

## 二、部署方案评估（结论：不需要新增服务器）

三种方式按推荐顺序：

### 方案 A（推荐）：同机 nginx 挂静态站 + 后台填 URL，new-api 零迁移

RelayBases 正是 new-api 用这种方式实现的官方主页换肤，已被验证可靠。

1. 把 `portal/` 目录上传到服务器，例如 `/var/www/portal`；
2. nginx 增加一个静态 server（用你提供的子域名，如 `home.aiplantform.site`）：

```nginx
server {
    listen 443 ssl http2;
    server_name home.aiplantform.site;          # 换成你的子域名
    root /var/www/portal;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}
```

3. new-api 后台 → 系统设置：
   - **首页内容** 填 `https://home.aiplantform.site`（自动 iframe 嵌入；门户脚本已处理
     `target="_top"`，登录/注册/控制台按钮会跳出 iframe 直达 new-api）；
   - **文档链接** 填 `https://home.aiplantform.site/docs/index.html`；
   - **页脚 / 关于** 可粘贴 `admin-snippets/` 中对应片段，或留空由门户页自行呈现。

优点：不迁移 new-api、API Base URL 不变、老用户无感知；缺点：主页包一层 iframe。

### 方案 B（更干净的长期方案）：门户占 www，new-api 迁到子域名

- `www.aiplantform.site` → 静态门户（nginx root 指向 portal/）；
- `console.aiplantform.site` → 反代 new-api（现有进程端口）；
- 修改 `portal/assets/js/site-config.js`：`consoleBase` 与 `apiBase` 改为
  `https://console.aiplantform.site`，文档示例地址同步生效；
- 后台「首页内容」清空（门户即根路径），「文档链接」填 `/docs/index.html`。

优点：SEO 友好、无 iframe、结构最清晰；代价：老用户 API Base URL 需更换，建议公告过渡期。

### 方案 C（零部署过渡）：什么都不部署

把 `admin-snippets/homepage-standalone.html` 全文粘贴到后台「首页内容」，
配合 `admin-snippets/footer.html`、`about.html`，立即获得同风格首页。

## 三、本地预览

```bash
# 在 portal/ 目录起一个静态服务（Python）
cd portal && python -m http.server 8080
# 浏览器打开 http://localhost:8080
```

注意：登录/注册/控制台按钮指向 `site-config.js` 中的线上域名，本地预览时点击会跳到线上站点，属预期行为。

## 四、日常维护

| 需求 | 改哪里 |
| --- | --- |
| 控制台/API 域名 | `assets/js/site-config.js`（全站单点） |
| 新增/调整模型 | `models.html` 底部 `MODEL_DATA` 数组 |
| 页眉导航、页脚栏目 | `assets/js/main.js`（注入模板，改一处全站生效） |
| 配色与设计 | `assets/css/style.css` 顶部 CSS 变量 |
| 价格示例表 | `pricing.html` 表格 |
| 文档内容 | `docs/*.html` 对应页面 |
| ICP 备案号 | `site-config.js` 的 `icp` 字段 |

## 五、与 new-api 后台可自定义区域的关系

后台四个可配置字段与本门户的对应关系：

| 后台字段 | 推荐配置值 |
| --- | --- |
| 文档链接 | 门户文档中心 URL（方案 A/B 对应不同地址，见上文） |
| 页脚 | `admin-snippets/footer.html`（或在门户内展示则留空） |
| 关于 | `admin-snippets/about.html`（同上） |
| 首页内容 | 方案 A：门户 URL；方案 B：留空/清空；方案 C：自包含首页全文 |

**发布流程约束（重要）**：本目录为纯静态新增文件，未触碰 new-api 源码；后续上线按
"本地确认 → 打 tag → 部署"流程执行，避免与并行任务产生文件冲突。
