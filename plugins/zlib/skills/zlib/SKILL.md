---
name: zlib
description: Search, browse popular or recommended books, download by ID, view history, and check Z-Library quotas through the zlib CLI. Use for finding or downloading Z-Library books. Use the bundled kindle skill for local-file delivery.
license: MIT
---

# zlib

Use the installed `zlib` binary. Prefer non-interactive commands and JSON.

## Start

Check the binary with `command -v zlib` on POSIX or `Get-Command zlib` in
PowerShell. If missing, tell the user to install it with
`go install github.com/heartleo/zlib/cmd/zlib@latest` or
`brew install heartleo/tap/zlib`.

For authenticated commands, run `zlib profile --json` once. Skip this check for
popular-only browsing. If the session is missing or expired, tell the user:

> Open a separate terminal, run `zlib login`, and complete the prompts. Tell me when login succeeds.

Wait for the user to finish. Reuse this handoff for later session errors.
Do not run interactive login or pass credentials as flags; login belongs in the
user's terminal. Kindle SMTP credentials are handled by the `kindle` skill.

## Find books

```bash
zlib search "dune" --json --count 10
zlib search "dune" --json --count 10 --page 2 --ext epub
zlib popular --json
zlib recommended --json
```

Search returns `{books, page, total_pages}`. Popular and recommended return book
arrays, including `[]` when empty. Recommended requires login; popular allows
anonymous requests. The two lists have no `--count`, `--page`, or filter flags.

Use `--count 10` for search to limit response size. Parse the complete JSON
before selecting fields; never truncate raw JSON. Use the same rule for history
and profile. Relay every result in a Markdown table: #, ID, Title, Authors, Year,
Format, Size, Rating. Keep suspect entries and note the concern; missing values
appear as `-`. IDs are ready for download. Ask the user to choose when several
matches fit; otherwise select from the requested title, author, format, and size.

If an older binary lacks JSON support, use search with a query or history with
`--page` to get static tables, and suggest upgrading.

## Download

Check `ZLIB_DOWNLOAD_DIR` in the environment, working-directory `.env`, and
`~/.config/zlib/.env`, reading only that setting. If configured, omit `--dir` and
let the CLI resolve it. An explicit destination from the user takes precedence.
If neither exists, ask where to save, offering Downloads, `./books`, Desktop,
or a custom path. Show a platform-correct absolute path. Create a selected
missing directory and reuse the choice for this session.

If the user requests a permanent default, update `ZLIB_DOWNLOAD_DIR` in the
global `.env` without overwriting other settings. Precedence is shell → local
`.env` → global `.env`.

```bash
zlib download 115066162:e9f13b --dir ./books
```

Read and report the printed `Saved to:` path. Without a TTY, download prints
plain progress and exits; it does not require an interactive UI. `--dir` overrides
`ZLIB_DOWNLOAD_DIR`; otherwise the default is the current directory.
Add `--send-to-kindle` only when requested and Kindle is configured.

Older builds may ignore the default directory or wait on a TUI. If a command
hangs, stop it with the execution timeout and inspect the target file before
reporting success. Use an explicit `--dir` for builds that ignore the variable,
and suggest upgrading. Respect the daily quota; do not loop past it.

## History and quota

```bash
zlib history --json
zlib history --json --page 2 --format epub
zlib history --download 115066162:e9f13b --dir ./books
zlib profile --json
```

History returns `{items, page, total_pages}`. Profile returns `daily_amount`,
`daily_allowed`, `daily_remaining`, and `daily_reset`. Re-download uses the same
destination rules as download. Show parsed values, not raw JSON or ANSI output.

Bare `search`, bare `history`, and Kindle commands without a file can open a
prompt. Use the explicit forms above; local-file delivery belongs to `kindle`.

## Connection errors

```bash
zlib endpoint --json
zlib doctor --json
```

New logins prefer EAPI and can recover endpoints automatically. `--domain` or
`ZLIB_DOMAIN` pins an endpoint and disables switching; explain this when
suggesting an override. Doctor checks HTML and EAPI independently.

For a proxy, have the user edit `ZLIB_PROXY` in their env file. Preserve existing
settings and keep secrets out of chat and shell history. Shell values override
both `.env` files, and the working-directory file overrides the global one.
Upgrade old binaries if configured values are ignored.
