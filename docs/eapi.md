# EAPI 与自动连接

```bash
zlib login
zlib search "dune"
zlib popular
zlib recommended --json
```

新登录优先 EAPI，HTML 用于兼容与兜底。无需先运行 `doctor` 或配置域名。
旧 HTML 会话继续可用，重新登录可切换为 EAPI 优先。

## 手动设置与诊断

```bash
zlib login --domain https://z-lib.gd # 固定 endpoint
zlib login --backend eapi           # 禁止 HTML 兜底
zlib login --backend html           # 使用 HTML
zlib login --cookies ./cookies.txt --domain https://z-lib.gd
zlib endpoint --json                # 只读本地选择与缓存
zlib doctor                        # 检查 HTML 和 EAPI
```

`login --eapi` 保持兼容。`doctor --eapi` 已弃用，仍可仅检查 EAPI。
Cookie 导入需匹配选定域名；会话与导出文件包含登录凭据，请妥善保存。

## Endpoint 选择

优先级：`--domain` → `ZLIB_DOMAIN`（含 `.env`）→ 会话域名 → last-good → 内置 seed → 官方发现。
显式域名进入手动模式，禁止自动换站。自动模式先请求当前 endpoint，失败后才探测候选。

每次业务请求最多换站一次、降级至 HTML 一次。候选固定排序，须通过 HTTPS、域名
白名单和实际 Probe；EAPI Probe 要求 JSON `success=1`。最多检查 16 个候选，
恢复扫描预算 20 秒；官方发现最多尝试三个受信源站，不携带账号凭据。

成功请求更新 `~/.config/zlib/state.json`，记录 endpoint、延迟与成功时间。
该文件可删除重建，不保存凭据，也不修改 `.env` 或 `session.json`。
损坏、不兼容版本或不可信缓存会被忽略。换站提示写入 stderr，JSON stdout 保持可解析。

## 错误与兜底

| 情况 | 处理 |
| --- | --- |
| DNS/TLS/连接失败、超时、502/503/504、513/517、DiamWall、重定向异常、不兼容响应 | 自动模式可换站；支持 HTML 的功能可降级 |
| 无效凭据、401、429、权限、额度、图书不存在、错误参数、业务 `success=0` | 直接返回，不换站、不降级 |
| 登录端点在 HTTP 200/400 下精确返回 `Authorization failed` | 可通过 `/rpc.php` 取得 remix Cookie，继续保持 EAPI |

业务拒绝即使使用 HTTP 503，也不重试。HTML 兜底只影响当前请求；全文搜索和旧
HTML ID 直接走兼容路径。文件 CDN 的传输重试不触发 endpoint 扫描。

## 接口与输出

| 功能 | EAPI 端点 |
| --- | --- |
| 登录 | `POST /eapi/user/login` |
| 搜索 | `POST /eapi/book/search` |
| 热门 | `GET /eapi/book/most-popular` |
| 推荐 | `GET /eapi/user/book/recommended` |
| 配额 | `GET /eapi/user/profile` |
| 历史 | `GET /eapi/user/book/downloaded` |
| 详情 / 下载链接 | `GET /eapi/book/{id}/{hash}` / `GET /eapi/book/{id}/{hash}/file` |
| 域名发现与诊断 | `GET /eapi/info/domains` |

`Client.Popular()`、`Client.Recommended()` 返回现有 `[]Book`。两者仅使用 EAPI，
恢复时只选择 EAPI 可用的 endpoint；旧 HTML 会话的 remix Cookie 可复用。
热门允许匿名请求，推荐要求 `remix_userid` 和 `remix_userkey`。CLI 不支持
`--backend html`、分页或筛选，支持 `--json`（书籍数组）及 `--full-title`。
缺失元数据显示为 `-`，空数组正常，缺失或不兼容的 `books` 字段视为响应错误。

图书 ID 与 hash 组成下载引用：`zlib download 115066162:e9f13b`。
2026-10-06 匿名核验：热门返回 100 项，使用 `url` 字段且缺少年份、格式、大小；
推荐返回 HTTP 400 与 `Please login`。这不保证未来响应或真实账号行为不变。

## 域名与参考

CLI 接受 `1lib.sk`、`article.sk`、`articles.sk`、`z-lib.fm`、`z-lib.gd`、`z-lib.gl`、
`z-lib.sk`、`z-library.ec`、`zlib.bz`、`zliba.ru` 及其子域名。
配置必须是 HTTPS 源站，不包含端口、路径、查询或凭据。自动发现会排除未知域名、
HTTP、onion 与 redirector；业务请求拒绝跨源重定向。

接口参考 [EAPI 文档](https://github.com/baroxyton/zlibrary-eapi-documentation)。
自动发现取舍见 [KOPlugin 笔记](koplugin-endpoint-notes.md)。上游字段与域名可能变化，
以本次 `zlib doctor` 为准。
