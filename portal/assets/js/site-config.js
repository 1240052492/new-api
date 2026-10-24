/**
 * 智算星球 · 门户站点全局配置（单点修改）
 * ---------------------------------------------------------------
 * consoleBase : new-api 控制台所在域名（登录 / 注册 / 控制台入口都指向这里）
 *               当前 new-api 与门户同域部署在 www.aiplantform.site；
 *               若后续将 new-api 迁移到 console.aiplantform.site，
 *               只需把 consoleBase 改为 "https://console.aiplantform.site"。
 * apiBase     : 文档示例中的 API 调用地址（OpenAI 兼容入口）
 */
window.SITE_CONFIG = {
  siteName: "智算星球",
  consoleBase: "https://www.aiplantform.site",
  apiBase: "https://www.aiplantform.site",
  docsBase: "", // 文档中心与门户同域，留空即可
  contactEmail: "support@aiplantform.site",
  icp: "", // 如有 ICP 备案号，填写后自动显示在页脚，例如 "京ICP备XXXXXXXX号"
  copyright: "© 2025 智算星球 · 保留所有权利",
};
