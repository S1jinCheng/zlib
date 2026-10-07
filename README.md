<div align="center">

<img src="docs/logo.svg" alt="zlib" width="160"/>

# zlib

A Z-Library client for your terminal.

![Go version](https://img.shields.io/badge/go-1.25%2B-blue)
[![CI](https://img.shields.io/github/actions/workflow/status/heartleo/zlib/release.yml)](https://github.com/heartleo/zlib/actions)
[![Release](https://img.shields.io/github/v/release/heartleo/zlib)](https://github.com/heartleo/zlib/releases)
[![Downloads](https://img.shields.io/github/downloads/heartleo/zlib/total)](https://github.com/heartleo/zlib/releases)
![License](https://img.shields.io/badge/license-MIT-green)

<a href="https://trendshift.io/repositories/135168?utm_source=trendshift-badge&utm_medium=badge&utm_campaign=badge-trendshift-135168" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/135168/daily?language=Go" alt="heartleo%2Fzlib | Trendshift" width="250" height="55"/></a>

English | [中文](README.zh.md) | [Website](https://zlib.heartleo.dev/)

</div>

<picture>
  <source media="(prefers-color-scheme: light)" srcset="docs/demo-search-light.gif">
  <source media="(prefers-color-scheme: dark)" srcset="docs/demo-search.gif">
  <img alt="Search demo" src="docs/demo-search.gif">
</picture>

Search, browse popular books and recommendations, download, and send files to Kindle.
Includes JSON output and Claude Code / Codex plugins.

## Install

```bash
brew install heartleo/tap/zlib # macOS / Linux
```

Windows: `winget install heartleo.zlib`.

<details>
<summary>Other installation options</summary>

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/heartleo/zlib/main/install.sh | sh

# Go 1.25+
go install github.com/heartleo/zlib/cmd/zlib@latest

# From source
git clone https://github.com/heartleo/zlib
cd zlib
go build -o zlib ./cmd/zlib
```

Prebuilt binaries: [Releases](https://github.com/heartleo/zlib/releases).

</details>

## Quick Start

```bash
zlib login
zlib search "dune"
zlib download 11735659:7659fb # use id:hash from --json
```

EAPI search tables show the numeric ID. For download, use `id:hash` from
`zlib search "dune" --json`, or select a book in interactive search.

Endpoints are selected automatically. New logins prefer EAPI with HTML fallback.
Existing HTML sessions still work; log in again to switch to EAPI first.

Browse without a query:

```bash
zlib popular
zlib recommended # requires login
```

## Commands

| Command | What it does |
| --- | --- |
| `login` / `logout` | Save or clear your session |
| `search "dune"` | Show results; omit the query for an interactive picker |
| `popular` | Browse popular books; a session is optional |
| `recommended` | Show recommendations for the logged-in account |
| `download ID --dir ./books` | Download to an existing directory |
| `history` | Browse and re-download past books |
| `profile` | Show daily download quota |
| `kindle` / `kindle send ./book.epub` | Configure delivery or send a local file |
| `endpoint` | Show the selected endpoint and cached health |
| `doctor` | Check HTML and EAPI connectivity |
| `theme auto` / `theme nord` | Follow the terminal background or choose a palette |

Run `zlib COMMAND --help` for flags. Interactive search and history use `↑/↓`
to select, `←/→` to change pages, and `Enter` to download.

```bash
zlib search "dune" --ext epub --page 2
zlib popular --full-title
zlib recommended --full-title
zlib download 11735659:7659fb --dir ./books --send-to-kindle
```

Popular and recommended are static lists with no pagination or filters.
Missing metadata appears as `-`. Kindle setup: [Send to Kindle](https://zlib.heartleo.dev/send-to-kindle/).

### JSON

```bash
zlib search "dune" --json --count 10
zlib popular --json
zlib recommended --json
zlib history --json
```

Search returns `{books, page, total_pages}`; history returns `{items, page,
total_pages}`. Popular and recommended return book arrays, or `[]` when empty.
Book IDs are ready for `download`, and titles stay complete.
`profile`, `endpoint`, and `doctor` also support `--json`.

## Agent Plugins

Claude Code:

```text
/plugin marketplace add heartleo/zlib
/plugin install zlib@zlib-plugin-cc
```

Use `/zlib "dune"` to search and download, or `/zlib:kindle` for Kindle delivery.

Codex:

```bash
codex plugin marketplace add heartleo/zlib
codex plugin add zlib@zlib
```

Install the CLI and log in, then ask the agent to find a book.

## Configuration

Optional settings go in `~/.config/zlib/.env`, one `KEY=value` per line.
Precedence: shell environment → working-directory `.env` → global `.env`.

| Variable | Purpose |
| --- | --- |
| `ZLIB_DOMAIN` | Pin an endpoint; disable automatic switching |
| `ZLIB_PROXY` | HTTP / HTTPS / SOCKS proxy for library requests |
| `ZLIB_DOWNLOAD_DIR` | Default download directory; expands `~` |
| `ZLIB_DOWNLOAD_RETRIES` | File-transfer retries; default `3` |
| `ZLIB_SMTP_PWD` | SMTP password for Kindle delivery |
| `ZLIB_THEME` | Theme override |

`--domain` and `--dir` override their environment settings. Keep secrets out of
shell history. Session cookies live in `session.json`; `state.json` is a
rebuildable endpoint cache. Login never edits your `.env`.

## Troubleshooting

```bash
zlib endpoint --json
zlib doctor
zlib doctor --proxy http://127.0.0.1:7890
```

Check `Mode` and `Source` for an old domain pin. Remove unwanted `ZLIB_DOMAIN`
settings to restore automatic switching. In `doctor`, look for `healthy` EAPI
rows; HTML access can fail independently. For missing or expired sessions, run
`zlib login`. Authentication, quota, and rate-limit errors are not retried on
another endpoint.

[Endpoint and fallback rules](docs/eapi.md) · [Report an issue](https://github.com/heartleo/zlib/issues)
