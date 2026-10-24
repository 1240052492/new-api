/* ============================================================
   智算星球 · 门户站点公共脚本
   - 注入统一页眉 / 页脚（所有页面共用，改一处全站生效）
   - 登录 / 注册 / 控制台链接单点指向 site-config.js 里的 consoleBase
   - 页面在 iframe 中嵌入时自动以 target="_top" 跳转，避免被框住
   ============================================================ */
(function () {
  var C = window.SITE_CONFIG || {};
  var SITE = C.siteName || "智算星球";
  var CONSOLE = C.consoleBase || "";
  // 子目录页面（如 docs/）通过 body[data-root="../"] 指定站内链接前缀
  var ROOT = document.body.dataset.root || "";

  // 嵌入态（被 new-api「首页内容」以 iframe 方式加载）时隐藏门户自身页眉/页脚，
  // 由 new-api 顶栏与后台「页脚」字段承担导航与版权，避免双重导航。
  try {
    if (window.self !== window.top) document.documentElement.classList.add("embedded");
  } catch (e) {
    document.documentElement.classList.add("embedded");
  }

  function u(path) {
    // 控制台链接：拼 consoleBase；站内链接：保持相对路径
    if (path.indexOf("/") === 0 && CONSOLE) return CONSOLE + path;
    return path;
  }
  function r(path) {
    return ROOT + path;
  }
  function top() {
    // 处于 iframe（new-api 后台首页内容 URL 嵌入）中时，链接跳出 iframe
    try { return window.self !== window.top ? ' target="_top"' : ""; } catch (e) { return ' target="_top"'; }
  }

  /* ---------------- 页眉 ---------------- */
  var header = document.getElementById("site-header");
  if (header) {
    var page = (document.body.dataset.page || "").trim();
    var links = [
      { href: "index.html", text: "首页", key: "home" },
      { href: "models.html", text: "模型广场", key: "models" },
      { href: "pricing.html", text: "定价", key: "pricing" },
      { href: "docs/index.html", text: "文档中心", key: "docs" },
      { href: "about.html", text: "关于我们", key: "about" },
    ];
    var nav = links.map(function (l) {
      var cur = page === l.key ? ' aria-current="page" class="active"' : "";
      return '<a href="' + r(l.href) + '"' + cur + top() + ">" + l.text + "</a>";
    }).join("");
    header.innerHTML =
      '<div class="header-inner">' +
      '  <a class="brand" href="' + r("index.html") + '"' + top() + '>' +
      '    <span class="brand-mark" aria-hidden="true">' +
      '      <svg viewBox="0 0 32 32" width="26" height="26"><defs><linearGradient id="lg-brand" x1="0" y1="0" x2="1" y2="1">' +
      '        <stop offset="0" stop-color="#6d7cff"/><stop offset="1" stop-color="#a855f7"/></linearGradient></defs>' +
      '      <circle cx="16" cy="16" r="9" fill="url(#lg-brand)"/><ellipse cx="16" cy="16" rx="14" ry="4.6" fill="none" stroke="#8ea2ff" stroke-width="1.6" transform="rotate(-18 16 16)"/>' +
      '      <circle cx="24.5" cy="10.5" r="1.6" fill="#67e8f9"/></svg>' +
      "    </span>" +
      '    <span class="brand-name">' + SITE + "</span>" +
      "  </a>" +
      '  <nav class="main-nav" aria-label="主导航">' + nav + "</nav>" +
      '  <div class="header-actions">' +
      '    <a class="btn btn-ghost" href="' + u("/login") + '"' + top() + ">登录</a>" +
      '    <a class="btn btn-primary" href="' + u("/register") + '"' + top() + ">免费注册</a>" +
      '    <button class="nav-toggle" aria-label="打开菜单" aria-expanded="false"><span></span><span></span><span></span></button>' +
      "  </div>" +
      "</div>";
    var toggle = header.querySelector(".nav-toggle");
    toggle.addEventListener("click", function () {
      var open = header.classList.toggle("nav-open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    });
  }

  /* ---------------- 页脚 ---------------- */
  var footer = document.getElementById("site-footer");
  if (footer) {
    var icp = C.icp
      ? '<p class="icp"><a href="https://beian.miit.gov.cn" target="_blank" rel="noopener">' + C.icp + "</a></p>"
      : "";
    footer.innerHTML =
      '<div class="footer-inner">' +
      '  <div class="footer-brand">' +
      '    <a class="brand" href="' + r("index.html") + '"' + top() + '><span class="brand-name">' + SITE + "</span></a>" +
      '    <p>统一接入全球主流大模型，OpenAI 兼容协议，换一行 Base URL 即刻使用。</p>' +
      '    <p class="footer-contact">商务与合作：<a href="mailto:' + (C.contactEmail || "") + '">' + (C.contactEmail || "") + "</a></p>" +
      "  </div>" +
      '  <div class="footer-col"><h4>产品</h4>' +
      '    <a href="' + r("models.html") + '"' + top() + ">模型广场</a>" +
      '    <a href="' + r("pricing.html") + '"' + top() + ">定价说明</a>" +
      '    <a href="' + u("/console") + '"' + top() + ">进入控制台</a></div>" +
      '  <div class="footer-col"><h4>资源</h4>' +
      '    <a href="' + r("docs/index.html") + '"' + top() + ">文档中心</a>" +
      '    <a href="' + r("docs/quickstart.html") + '"' + top() + ">快速开始</a>" +
      '    <a href="' + r("docs/api-reference.html") + '"' + top() + ">接口文档</a>" +
      '    <a href="' + r("docs/faq.html") + '"' + top() + ">常见问题</a></div>" +
      '  <div class="footer-col"><h4>公司与法律</h4>' +
      '    <a href="' + r("about.html") + '"' + top() + ">关于我们</a>" +
      '    <a href="' + r("agreement.html") + '"' + top() + ">用户协议</a>" +
      '    <a href="' + r("privacy.html") + '"' + top() + ">隐私政策</a></div>" +
      "</div>" +
      '<div class="footer-bottom"><p>' + (C.copyright || "") + "</p>" + icp + "</div>";
  }

  /* ---------------- 模型广场筛选（仅 models.html 使用） ---------------- */
  var grid = document.getElementById("model-grid");
  if (grid) {
    var models = window.MODEL_DATA || [];
    var tabs = document.querySelectorAll(".model-tabs button");
    function render(cat) {
      var list = models.filter(function (m) { return cat === "all" || m.cat === cat; });
      grid.innerHTML = list.map(function (m) {
        return '<article class="model-card">' +
          '<div class="model-head"><span class="model-name">' + m.name + '</span>' +
          '<span class="tag tag-' + m.cat + '">' + m.catLabel + "</span></div>" +
          '<p class="model-vendor">' + m.vendor + "</p>" +
          '<p class="model-desc">' + m.desc + "</p>" +
          '<div class="model-price"><span>输入 ¥' + m.in + " / 百万 tokens</span><span>输出 ¥" + m.out + " / 百万 tokens</span></div>" +
          "</article>";
      }).join("");
    }
    tabs.forEach(function (b) {
      b.addEventListener("click", function () {
        tabs.forEach(function (x) { x.classList.remove("active"); });
        b.classList.add("active");
        render(b.dataset.cat);
      });
    });
    render("all");
  }

  /* ---------------- 控制台类链接（data-console）统一改写 ----------------
     页面中写死的 href="/register"、"/console" 等相对链接：
     - 嵌入 www iframe 时会解析到 home 子域（静态站）导致 404；
     - 直接整页访问 home 子域时同样 404。
     统一改写为 consoleBase + 路径，并始终以 _top 跳出 iframe。 */
  document.querySelectorAll("a[data-console]").forEach(function (a) {
    var p = a.getAttribute("href") || "";
    if (p.indexOf("/") === 0) {
      a.setAttribute("href", u(p));
      a.setAttribute("target", "_top");
    }
  });

  /* ---------------- 文档页代码复制 ---------------- */
  document.querySelectorAll(".copy-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var pre = btn.parentElement.querySelector("code");
      if (!pre) return;
      navigator.clipboard.writeText(pre.innerText).then(function () {
        btn.textContent = "已复制";
        setTimeout(function () { btn.textContent = "复制"; }, 1500);
      });
    });
  });
})();
