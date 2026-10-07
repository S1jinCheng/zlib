<div align="center">

<img src="docs/logo.svg" alt="zlib" width="160"/>

# zlib

Z-Library 命令行客户端。

![Go version](https://img.shields.io/badge/go-1.25%2B-blue)
[![CI](https://img.shields.io/github/actions/workflow/status/heartleo/zlib/release.yml)](https://github.com/heartleo/zlib/actions)
[![Release](https://img.shields.io/github/v/release/heartleo/zlib)](https://github.com/heartleo/zlib/releases)
[![Downloads](https://img.shields.io/github/downloads/heartleo/zlib/total)](https://github.com/heartleo/zlib/releases)
![License](https://img.shields.io/badge/license-MIT-green)

<a href="https://trendshift.io/repositories/135168?utm_source=trendshift-badge&utm_medium=badge&utm_campaign=badge-trendshift-135168" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/135168/daily?language=Go" alt="heartleo%2Fzlib | Trendshift" width="250" height="55"/></a>

[English](README.md) | 中文 | [项目主页](https://zlib.heartleo.dev/zh/)

</div>

<picture>
  <source media="(prefers-color-scheme: light)" srcset="docs/demo-search-light.gif">
  <source media="(prefers-color-scheme: dark)" srcset="docs/demo-search.gif">
  <img alt="Search demo" src="docs/demo-search.gif">
</picture>

搜书、浏览热门与推荐、下载、发送到 Kindle。支持 JSON 输出和 Claude Code / Codex 插件。

## 安装

```bash
brew install heartleo/tap/zlib # macOS / Linux
```

Windows：`winget install heartleo.zlib`。

<details>
<summary>其他安装方式</summary>

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/heartleo/zlib/main/install.sh | sh

# Go 1.25+
go install github.com/heartleo/zlib/cmd/zlib@latest

# 从源码构建
git clone https://github.com/heartleo/zlib
cd zlib
go build -o zlib ./cmd/zlib
```

预编译二进制：[Releases](https://github.com/heartleo/zlib/releases)。

</details>

## 快速开始

```bash
zlib login
zlib search "dune"
zlib download 11735659:7659fb # 使用 --json 返回的 id:hash
```

EAPI 搜索表格只显示数字 ID。下载时使用 `zlib search "dune" --json` 返回的
`id:hash`，也可在交互搜索中直接选书下载。

自动选择 endpoint，无需配置域名。新登录优先 EAPI，HTML 用于兜底。
旧 HTML 会话仍可使用，重新登录即可切换为 EAPI 优先。

也可以直接浏览：

```bash
zlib popular
zlib recommended # 需要登录
```

## 命令

| 命令 | 用途 |
| --- | --- |
| `login` / `logout` | 保存或清除登录会话 |
| `search "dune"` | 展示结果；不带关键词进入交互模式 |
| `popular` | 浏览热门书籍，可不登录 |
| `recommended` | 查看当前账号的个性化推荐 |
| `download ID --dir ./books` | 下载到已存在的目录 |
| `history` | 浏览历史并重新下载 |
| `profile` | 查看每日下载配额 |
| `kindle` / `kindle send ./book.epub` | 配置投递或发送本地文件 |
| `endpoint` | 查看当前 endpoint 和缓存状态 |
| `doctor` | 检查 HTML 和 EAPI 连接 |
| `theme auto` / `theme nord` | 跟随终端背景或选择配色 |

用 `zlib COMMAND --help` 查看参数。交互式搜索和历史记录使用 `↑/↓` 选择，
`←/→` 翻页，`Enter` 下载。

```bash
zlib search "dune" --ext epub --page 2
zlib popular --full-title
zlib recommended --full-title
zlib download 11735659:7659fb --dir ./books --send-to-kindle
```

热门和推荐为静态列表，不提供分页或筛选。缺失字段显示为 `-`。
Kindle 配置：[发送到 Kindle](https://zlib.heartleo.dev/send-to-kindle/)。

### JSON

```bash
zlib search "dune" --json --count 10
zlib popular --json
zlib recommended --json
zlib history --json
```

搜索返回 `{books, page, total_pages}`，历史返回 `{items, page, total_pages}`。
热门和推荐返回书籍数组，空列表为 `[]`。书籍 ID 可直接下载，标题不截断。
`profile`、`endpoint`、`doctor` 也支持 `--json`。

## Agent 插件

Claude Code：

```text
/plugin marketplace add heartleo/zlib
/plugin install zlib@zlib-plugin-cc
```

用 `/zlib "dune"` 搜索和下载，`/zlib:kindle` 投递 Kindle。

Codex：

```bash
codex plugin marketplace add heartleo/zlib
codex plugin add zlib@zlib
```

先安装 CLI 并登录，再让 agent 找书。

## 配置

可选配置写入 `~/.config/zlib/.env`，每行一个 `KEY=value`。
优先级：shell 环境变量 → 工作目录 `.env` → 全局 `.env`。

| 变量 | 用途 |
| --- | --- |
| `ZLIB_DOMAIN` | 固定 endpoint，禁用自动换站 |
| `ZLIB_PROXY` | 图书请求的 HTTP / HTTPS / SOCKS 代理 |
| `ZLIB_DOWNLOAD_DIR` | 默认下载目录，支持展开 `~` |
| `ZLIB_DOWNLOAD_RETRIES` | 文件传输重试次数，默认 `3` |
| `ZLIB_SMTP_PWD` | Kindle 投递的 SMTP 密码 |
| `ZLIB_THEME` | 覆盖主题 |

`--domain`、`--dir` 优先于对应环境变量。密码避免写入 shell 历史。
`session.json` 保存登录 Cookie；`state.json` 是可重建的 endpoint 缓存。
登录不会修改 `.env`。

## 排障

```bash
zlib endpoint --json
zlib doctor
zlib doctor --proxy http://127.0.0.1:7890
```

检查 `Mode`、`Source` 是否存在旧域名配置；移除不需要的 `ZLIB_DOMAIN` 可恢复自动换站。
`doctor` 中优先查看 EAPI 的 `healthy` 记录，HTML 连接可能单独失败。
会话缺失或过期时重新运行 `zlib login`。认证、额度和限流错误不会换站重试。

[连接与兜底规则](docs/eapi.md) · [反馈问题](https://github.com/heartleo/zlib/issues)
