# Repository Guidelines

## Project Structure & Module Organization

The root `zlib` Go package implements the Z-Library client, HTML parsing, EAPI requests, downloads, and Kindle delivery. `cmd/zlib/main.go` is the executable entry point. `internal/cli/` contains Cobra commands and terminal interfaces; `internal/config/` manages saved settings and sessions; `internal/tableprinter/` handles table output. Tests live beside their source files.

`docs/` contains API notes, demo recordings, and artwork. `plugins/zlib/` contains agent skills. `site/` holds the static landing page and assets; `scripts/build-zh.mjs` generates its Chinese page. GitHub Actions workflows live in `.github/workflows/`.

## Build, Test, and Development Commands

Use the Go version declared in `go.mod` (currently 1.25.0).

- `make build`: build the local `zlib` executable.
- `go run ./cmd/zlib --help`: inspect CLI commands during development.
- `make fmt`: format all Go packages with Go's standard formatter.
- `make vet`: run static analysis.
- `make tidy`: reconcile module dependencies after dependency changes.
- `go test ./...`: run the complete test suite.
- `go build ./...`: compile all packages, matching CI.
- `node scripts/build-zh.mjs`: regenerate `site/zh/index.html`; the output is ignored by Git.
- `python3 -m http.server 8000 --directory site`: preview the landing page locally.

## Coding Style & Naming Conventions

Use `gofmt` formatting and tab indentation for Go. Follow existing exported `PascalCase` and unexported `camelCase` identifiers, with descriptive lowercase filenames such as `domain_policy.go`. Keep reusable client behavior in the root package and command presentation in `internal/cli/`. Use `go vet` for static checks; no separate lint configuration is present.

## Testing Guidelines

Use Go's standard `testing` package. Name files `*_test.go` and functions `TestBehavior` or `TestBehavior_Scenario`, following neighboring tests. Use `httptest` servers for HTTP behavior and isolated temporary directories for filesystem cases. Add regression tests for fixes, including relevant failure paths. Run focused tests with `go test ./internal/cli -run TestName`, then the full suite. CI enforces no numeric coverage threshold.

## Commit & Pull Request Guidelines

History uses prefixes such as `feat:`, `fix:`, `docs:`, and `chore:` followed by concise imperative summaries. Keep commits focused. PR descriptions should explain the behavior change, link relevant issues, and report validation. Include screenshots or terminal recordings when changing visible interfaces. Keep `README.md` and `README.zh.md` aligned when documenting user-facing changes.

## Configuration & Secrets

Use `.env.example` as a configuration reference. Environment precedence is shell variables, working-directory `.env`, then `~/.config/zlib/.env`. Keep credentials, session cookies, and SMTP passwords out of commits and test fixtures; CI runs Gitleaks.
