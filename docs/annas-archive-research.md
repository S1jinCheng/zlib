# Anna’s Archive 上游下载接口调研

调研日期：2026-10-06。仅检查公开文档与源码；没有使用账号密钥、调用有效文件下载或下载图书。

## 范围

首期可接入会员快速下载 API。免费慢速下载需要等待和浏览器验证。FAQ 仅承诺会员下载接口，搜索与元数据需单独评估。[官方 FAQ](https://annas-archive.gl/faq)

本次 `curl` 成功读取 `.gl` 接口自文档和官方 GitLab `main` 源码；浏览工具无法打开该 JSON 端点。下述成功流程、额度和有效期来自公开源码，未经真实会员请求验证。`main` 是可变分支，线上部署可能不同。[下载接口源码](https://software.annas-archive.gl/AnnaArchivist/annas-archive/-/raw/main/allthethings/dyn/views.py)

## 会员快速下载

请求：`GET /dyn/api/fast_download.json`。查询参数 `md5` 为文件 MD5，`key` 为有会员资格的账号 secret key；`path_index`、`domain_index` 为可选非负整数，分别选择收藏来源和下载服务器，源码默认均为 `0`。认证不是文档中的 Bearer header；实现必须对 URL 中的 `key` 脱敏。无需把 Z-Library 登录状态用于此接口。[接口自文档](https://annas-archive.gl/dyn/api/fast_download.json)

无参数实测返回 JSON：`download_url: null`、`error: "Invalid md5"`。成功自文档列出 `download_url` 与 `account_fast_download_info`，并称 HTTP 状态可能为 200 或 204；当前源码成功路径默认 200。HTTP 204 通常无响应体，客户端不能假定 204 可解析出链接，也不能把空响应当作下载成功。[接口自文档](https://annas-archive.gl/dyn/api/fast_download.json)、[下载接口源码](https://software.annas-archive.gl/AnnaArchivist/annas-archive/-/raw/main/allthethings/dyn/views.py)

源码错误映射：

| HTTP | `error` | 客户端处理建议 |
| --- | --- | --- |
| 400 | `Invalid md5` / `Invalid domain_index or path_index` | 校验输入；不要自动重试 |
| 401 | `Invalid secret key` | 提示更新密钥 |
| 403 | `Not a member` | 提示会员权限不足 |
| 404 | `Record not found` | 标记文件不可用 |
| 429 | `No downloads left` | 停止批量解析，提示额度耗尽 |
| 500 | `Error during fetching` | 有界退避重试 |

这些错误来自当前源码；客户端仍需安全展示未知状态与错误。[下载接口源码](https://software.annas-archive.gl/AnnaArchivist/annas-archive/-/raw/main/allthethings/dyn/views.py)

## 额度与链接生命周期

额度返回四字段：`downloads_left`、`downloads_per_day`、`recently_downloaded_md5s`、`downloads_done_today`。源码在为新的 MD5 生成链接时记录额度，文件传输失败不会自动撤销；近期已解析的相同 MD5 可重复取链接。返回的额度对象在写入记录之前计算，可能暂时比实际余额多一次。批量操作应顺序按需解析，避免预取大量链接。[下载接口源码](https://software.annas-archive.gl/AnnaArchivist/annas-archive/-/raw/main/allthethings/dyn/views.py)

源码使用滚动窗口：每天额度少于 450 时为 18 小时，其余为 24 小时；签名下载 URL 的过期时间为生成后两小时。不要按本地零点重置额度，也不要长期缓存链接；恢复失败时重新解析。[额度及链接生成源码](https://software.annas-archive.gl/AnnaArchivist/annas-archive/-/raw/main/allthethings/utils.py)

## 免费下载、搜索与文件标识

免费入口有共享慢速服务器及等待队列，并包含浏览器验证；可提供打开详情页或导入用户已取得直链的人工流程。验证供应商与等待规则可能变化，不应硬编码自动绕过流程。[官方 FAQ](https://annas-archive.gl/faq)

快速下载输入是具体文件的 MD5，不是 ISBN、书名或 Z-Library 数字 ID。官网文件详情使用 `/md5/<md5>`；同一本书不同文件版本可能有不同 MD5。适合先支持已知 MD5 或详情 URL，再考虑 HTML 搜索适配。[官方 FAQ 的文件链接与去重说明](https://annas-archive.gl/faq)

源码另有 `/dyn/api/aarecord_elasticsearch/<aarecord_id>.json`，需要账号的 `special_apis` 权限标记；这不是普通会员必然可用的公开搜索 API。不能把网页 JSON 或第三方代理宣传当作官方稳定搜索协议。[元数据接口源码](https://software.annas-archive.gl/AnnaArchivist/annas-archive/-/raw/main/allthethings/dyn/views.py)

## 域名与尚未验证项

调研时官方 FAQ 列出的镜像为 `annas-archive.gl`、`.pk`、`.gd`。这是官方当前声明，不保证所有地区实时可达；仅 `.gl` 文档和无参数端点在本次环境得到验证。官方还提供历史域名声明链与紧急 PGP 身份验证信息。配置应允许显式修改站点，并防止把密钥发送到未经确认的镜像。[官方镜像与身份说明](https://annas-archive.gl/faq)

实施前需验证会员成功响应、文件重定向、`Range` 续传、链接过期、搜索挑战、限流与 204 行为。
