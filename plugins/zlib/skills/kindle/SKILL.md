---
name: kindle
description: Send a local book to Kindle through zlib, configure SMTP and Amazon sender approval, or troubleshoot delivery. Use the zlib skill to find and download a book first.
license: MIT
---

# Kindle delivery

Send a local file through `zlib`; a Z-Library session is not required.
Check the binary with `command -v zlib` on POSIX or `Get-Command zlib` in
PowerShell. If missing, suggest `go install github.com/heartleo/zlib/cmd/zlib@latest`.

## Send

```bash
zlib kindle send ./books/dune.epub
```

Always pass a file path. Bare `kindle` and `kindle send` open interactive prompts.
Before sending, check `~/.config/zlib/kindle.json` and whether `ZLIB_SMTP_PWD` is
set, without displaying the secret. If config is missing, contains placeholders,
or lacks the password, complete setup below. Otherwise send and report the result.
`zlib download ID --send-to-kindle` also works after setup.

## Setup

Ask one question at a time and wait for each answer.

1. Ask for the Kindle address. The user can find it under Amazon → Manage Your
   Content and Devices → Preferences → Personal Document Settings.
2. Ask for the sender address. Remind the user to add it to Amazon's approved
   sender list on the same page.
3. Confirm the provider's SMTP host and port. For Gmail, use `smtp.gmail.com:587`.
   For other providers, use their documented settings or ask the user. Write
   `~/.config/zlib/kindle.json`, creating its directory if needed:

   ```json
   {"to":"kindle-address","from":"sender-address","smtp_host":"smtp.gmail.com","smtp_port":587}
   ```

4. Have the user edit `~/.config/zlib/.env` to set `ZLIB_SMTP_PWD` on its own line.
   Create the file only if missing; preserve existing settings and use `0600`
   permissions on POSIX. Give the exact path and the appropriate open command:

   - Windows: `notepad "$env:USERPROFILE\.config\zlib\.env"`
   - macOS: `open -t ~/.config/zlib/.env`
   - Linux: `${EDITOR:-nano} ~/.config/zlib/.env`

   For Gmail, use an [App Password](https://support.google.com/accounts/answer/185833)
   with 2-Step Verification enabled. Never request, paste, echo, or place the
   password on a command line. Wait until the user confirms the file is saved.

After setup, send the file and report SMTP success or the error. SMTP acceptance
alone does not prove the book reached the Kindle.

## Errors

| Error | Check |
| --- | --- |
| SMTP `535` / `454` | Sender, app password, and provider restrictions |
| Connection timeout | SMTP host, port, and network; `ZLIB_PROXY` does not cover SMTP |
| Sent but absent on Kindle | Kindle address and Amazon's approved sender list |
| Password missing | Shell or working-directory `.env` may override the global file |

Use a TCP probe appropriate to the shell if needed, such as PowerShell's
`Test-NetConnection HOST -Port PORT`. SMTP requires a reachable direct connection;
a system VPN/TUN may be needed. Older builds load only local `.env`; suggest
upgrading if global settings are ignored.
