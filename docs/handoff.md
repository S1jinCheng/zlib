# Handoff

更新：2026-10-07。此文件滚动记录当前工作，不保留重复的会话日志。

## 当前状态

- 分支：`feat/eapi-first`，基准提交 `6a299fd`。
- 改动已整理为本地提交，尚未推送。
- 已实现 EAPI 优先、自动 endpoint 恢复、`doctor` 双模式诊断、`endpoint`、
  `popular` 和 `recommended`。推荐支持 `--json`，需要 remix 登录凭据。
- Anna’s Archive 仅有接入分析，尚未实现。

## 实现边界

`endpoint.go` 管理候选验证与有界恢复。手动域名禁止换站；一次业务调用最多
换站一次、HTML 兜底一次。业务拒绝不触发恢复。成功状态写入独立 `state.json`，
不改用户 `.env`。完整规则见 [EAPI 文档](eapi.md)。

`eapi_lists.go` 提供两个列表 API，复用 `Book`；`internal/cli/books.go` 共用表格
与 JSON 输出。列表没有分页、过滤或独立 TUI，也不会为补齐元数据逐本查询详情。

README、中英文首页、Kindle 页及插件说明已按 no-ai-slop 精简。中文首页由
`node scripts/build-zh.mjs` 生成，不提交 `site/zh/index.html`。

搜索表格仅显示 EAPI 数字 ID，JSON 与下载仍使用 `id:hash`。深浅色 GIF
共用原版操作时序，每页 25 条真实书籍元数据；下载本地生成的 EPUB 样例，
路径显示为 `~/dune.epub`。录制脚本验证翻页、下载、文件名和播放时间轴。

## 验证

提交前已通过 `go test -race ./...`、`go vet ./...` 和 `go build ./...`。
热门接口已匿名核验；推荐成功路径使用模拟请求，尚未用真实账号验证。

文档阶段检查了生成脚本、页面 JavaScript、JSON-LD、浏览器渲染和 Lighthouse。
本地首页与 Kindle 页的无障碍、最佳实践、SEO 检查均通过；不代表线上已部署。

## 后续

合并前重新检查 diff。Pages 只在 `main` 的网站相关变更后
自动部署。若继续实现 Anna 下载，先读 [接入分析](annas-archive.md) 和
[上游调研](annas-archive-research.md)。

旧版本的 DiamWall 修复已在工作树中：先识别挑战再检查 HTTP 状态，限制挑战重试，
Cookie 读写加锁，限制详情并发；EAPI 业务错误保留服务端消息。不要仅凭 HTTP 200
或 `.env` 中的域名判断 endpoint 可用。
