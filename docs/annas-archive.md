# Anna’s Archive 下载接入分析

调研日期：2026-10-06。本文是设计建议，尚未实现功能，也未使用会员凭据下载文件。上游接口核验见 [调研记录](annas-archive-research.md)。

## 首版范围

建议先支持会员快速下载：输入 MD5 或 `/md5/<md5>` 链接，取得下载地址，复用文件传输和 Kindle 投递。不依赖搜索页面或 Z-Library 登录。

官方 [FAQ](https://annas-archive.gl/faq) 提供会员接口 `/dyn/api/fast_download.json`。免费慢速下载涉及等待和浏览器验证；搜索、免费路径与外部来源解析留待后续。

## 上游调用链

1. 校验并规范化输入：32 位十六进制 MD5；详情 URL 仅接受已核验的 Anna 域名及 `/md5/` 路径。
2. 请求 `GET /dyn/api/fast_download.json?md5=...&key=...`。`path_index`、`domain_index` 是可选下载路径选择参数，不应首版硬编码多个路径并行探测。
3. 检查 HTTP 状态、JSON 错误以及非空 `download_url`，随后下载文件。接口契约、可选参数和状态码详见 [接口自文档](https://annas-archive.gl/dyn/api/fast_download.json)。
4. 流式计算 MD5；与输入一致后才将临时文件提交为最终文件，再按需投递 Kindle。MD5 用于文件一致性校验，不作为来源真实性证明。

公开源码中，签名链接有效期为 2 小时，解析新 MD5 时记录额度；近期重复 MD5 不重复计数。返回额度为记录前快照。来源见调研记录，真实会员下载与配额边界尚未验证。

## 当前代码的耦合点

| 位置 | 当前行为 | 接入所需调整 |
| --- | --- | --- |
| `internal/cli/root.go:newClient` | 读取 Z-Library 会话，缺失时退出 | Anna 使用独立配置和客户端构造，不调用此入口 |
| `book.go:FetchBook` | 登录校验；解析 Z-Library HTML/EAPI | MD5 下载直接解析下载地址，不强制获取书名等元数据 |
| `download.go:DownloadWithContext` | 要求登录，可能先解析 EAPI URL | 将上游解析与通用文件传输分离 |
| `download.go:newDownloadRequest` | 添加 Z-Library Cookie 和 Referer | Anna/CDN 请求独立构造，凭据按来源隔离 |
| `internal/cli/download.go:downloadModel` | 持有 `*zlib.Client`，抓详情后下载 | 使用最小下载任务接口，兼容现有交互和非交互路径 |
| `internal/cli/search.go:bookCommandID` | 输出 `/book/` ID 或 `id:hash` | 后续增加来源明确的引用，避免把 MD5 当成 EAPI hash |
| `internal/cli/kindle.go` | 接受本地文件路径 | 直接复用下载后投递入口 |

`Book.Hash` 是现有 EAPI 引用的一部分；仓库没有证明其等于文件 MD5。后续搜索建议增加独立 `MD5` 字段与 `Source`，保留现有 JSON 字段含义。

## 建议的模块边界

- `internal/anna/`：独立客户端、MD5/URL 输入解析、快速下载 API 响应和错误映射。仅用于 CLI 时先放内部包；若需要公开 Go API，再明确公共包边界。
- `internal/filetransfer/`：提取可复用的流式写盘、进度通知、取消、有限重试和文件名清洗；由调用方提供请求策略，不认识登录状态或 EAPI。
- `internal/cli/anna.go`：新增独立命令，构造 Anna 下载任务。现有 `Client.Download*` 保留签名，通过内部传输模块维持兼容。

下载任务只需 `Run(ctx, destDir, progress) (DownloadResult, error)`，封装地址解析与传输。书名可暂用 MD5；文件名按 `Content-Disposition`、URL 路径、MD5 的顺序取值。

## 提议的 CLI 与配置

以下均为待实现设计：

```sh
zlib anna download <md5> --dir ~/Downloads
zlib anna download <官方详情URL> --send-to-kindle
zlib anna doctor
```

建议新增 `ANNA_DOMAIN`、`ANNA_API_KEY`，复用已有 `.env` 加载顺序、代理选项、下载目录和重试配置。会员 key 放环境或受保护的 `.env` 文件，不作为命令行必填参数，也不写入 Z-Library 的 `session.json`。`doctor` 默认只验证域名和无凭据接口契约，真实下载及配额操作由下载命令执行。

域名策略独立于 `domain_policy.go` 的 Z-Library 白名单。来源 API 只接受已核验 HTTPS 源站；下载链接可以指向其他 CDN，逐跳限制协议、重定向次数及目标地址策略，不能套用来源域名白名单导致合法 CDN 全部被拒。

## 下载正确性与凭据保护

- API key 只发送给 Anna API 源站；带 key 的请求默认拒绝跨源重定向。CDN 请求使用全新请求，不携带 key 或 Z-Library Cookie。
- API key 位于查询参数；`net/http` 错误可能包含完整 URL。错误展示、诊断和日志必须脱敏，签名下载 URL 也按临时凭据处理。
- 先写同目录临时文件，检查写入、关闭、长度（若已知）及 MD5，再提交；写盘失败、取消、校验失败均清理临时文件，明确同名文件策略。
- 检测伪装成 HTTP 200 的 HTML 登录/验证页面；同时支持没有 `Content-Length` 的真实文件响应。文件名解析使用 `mime.ParseMediaType`，兼容 `filename*`。
- 文件请求重试和重新调用解析 API 分开处理；401/403、额度不足和验证需求不盲目重试。是否支持链接刷新及断点续传，在上游行为核验后决定。
- 解析阶段和传输阶段共享可取消 context。现有非交互下载调用无 context 的路径也要纳入调整。

## 实施顺序与验收

1. 用脱敏固定响应和 `httptest` 完成 API 客户端，覆盖无 key、无效 MD5、上游错误、空链接、异常 JSON/HTML，以及 HTTP 204 无响应体的情况。
2. 提取文件传输模块，先通过现有 Z-Library 下载回归测试，再加入 MD5 校验、原子提交和跨源凭据隔离测试。
3. 增加独立 Anna 命令，验证无需 Z-Library 会话即可运行，目录选择、取消、进度和 Kindle 投递保持一致；同步中英文 README 与插件使用说明。
4. 如有测试会员 key，选择可用于测试的文件做一次端到端验证，确认文件 MD5、下载配额及重定向行为。自动测试不依赖真实 key 或在线书库。
5. 再评估 Anna 搜索。官方 FAQ 未提供面向普通会员的稳定搜索 API；源码另有受 `special_apis` 权限控制的元数据接口，不能假定普通会员可用。不能把网页抓取当成与快速下载 API 等价的可靠接口。免费路径先考虑浏览器手动完成验证；断点续传与多来源回退另行设计。

实施后运行 `go test ./...`、`go vet ./...` 和 `go build ./...`。
