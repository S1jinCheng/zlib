# zlib.heartleo.dev

静态 Pages 网站，无前端依赖。修改 `index.html`，生成中文页：

```bash
node scripts/build-zh.mjs
python3 -m http.server 8000 --directory site
```

预览 `/`、`/zh/` 和 `/send-to-kindle/`，检查桌面、手机和亮暗主题。

## 文件

- `index.html`：中英文首页与命令演示。
- `style.css`：共用样式，使用 Catppuccin Latte / Mocha。
- `send-to-kindle/index.html`：Kindle 配置指南。
- `zh/index.html`：生成的中文页，不提交。
- `og.png`：1200 × 630 社交分享图。

首页文案用相邻的 `data-i18n="en"` / `data-i18n="zh"` 元素配对。
生成脚本移除静态英文元素，并设置中文 canonical、hreflang 与分享信息。
演示中的书籍和连接状态是样例，不查询真实服务。

## 发布

`.github/workflows/pages.yml` 在 `main` 的网站、生成脚本或工作流变更后部署；
也可手动触发。工作流先生成中文页，再上传整个 `site/`。

域名在 GitHub Pages 设置中配置，不使用 `site/CNAME`。
DNS 将 `zlib.heartleo.dev` 指向 `heartleo.github.io`。
修改页面时保持现有 URL、canonical、hreflang 和 sitemap 一致。
