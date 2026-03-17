# Kestrel

High-performance Claude API proxy with multi-account rotation. Built in Go.

## What it does

Kestrel sits between Claude Code and the Anthropic API. It translates OpenAI-format requests to Claude format, manages multiple Claude accounts, and automatically rotates to the next available account when one hits rate limits or quota.

```
Claude Code ──> Kestrel (Go proxy) ──> Anthropic API
                    │
                    ├── Account 1 (active)
                    ├── Account 2 (cooldown)
                    └── Account 3 (active)
```

## Features

- **OpenAI-compatible proxy** — Claude Code connects via `/v1/chat/completions`
- **Multi-account rotation** — automatic fallback when an account is rate-limited or exhausted (planned)
- **SSE streaming** — transparent passthrough with format translation
- **Exponential backoff** — per-account cooldown with automatic recovery
- **SQLite persistence** — accounts and API keys with auto-migration
- **Admin API** — CRUD for accounts and keys
- **Structured logging** — slog with JSON or pretty text output
- **Encryption at rest** — AES-256-GCM for stored API keys

## Stack

| Component   | Choice                     |
|-------------|----------------------------|
| Language    | Go 1.25+                   |
| Architecture| Clean Architecture + SOLID |
| Router      | chi                        |
| Database    | SQLite (modernc.org/sqlite)|
| Logging     | slog (stdlib)              |
| Config      | caarlos0/env               |

## Quick Start

```bash
# Run the server
go run ./cmd/kestrel

# Or use the Makefile
make dev-api

# Run tests
make test
# or
go test ./... -coverprofile=coverage.out

# Build
make build
```

## Environment Variables

| Variable     | Default | Description                         |
|--------------|---------|-------------------------------------|
| `PORT`       | `8080`  | HTTP server port                    |
| `LOG_LEVEL`  | `info`  | Log level (debug, info, warn, error)|
| `LOG_FORMAT` | `json`  | Log format (json, pretty)           |

## Endpoints Implemented

| Method | Path                         | Description                |
|--------|------------------------------|----------------------------|
| GET    | `/health`                    | Health check (public)      |
| GET    | `/v1/models`                 | List supported models      |
| POST   | `/v1/chat/completions`       | Chat proxy (OpenAI format) |
| GET    | `/admin/accounts`            | List accounts              |
| POST   | `/admin/accounts`            | Create account             |
| PUT    | `/admin/accounts/{id}`       | Update account             |
| DELETE | `/admin/accounts/{id}`       | Delete account             |
| POST   | `/admin/accounts/{id}/reset` | Reset account cooldown     |
| GET    | `/admin/keys`                | List API keys              |
| POST   | `/admin/keys`                | Create API key             |
| DELETE | `/admin/keys/{id}`           | Revoke API key             |

Admin endpoints require `X-Admin-Key` header.

## Implementation Status

- **Phase 1** — Skeleton (server, config, logger, middlewares): Done
- **Phase 2** — Domain + Translation (entities, VOs, Claude adapter, chat handler, SSE): Done
- **Phase 3** — SQLite + Persistence (repos, migrations, crypto, admin CRUD): Done
- **Phase 4** — Authentication + Logging Middleware: Pending
- **Phase 5** — Multi-account + Fallback: Pending
- **Phase 6** — Request Log persistence: Pending
- **Phase 7** — Frontend + Deploy: Pending
- **Phase 8** — Integration + E2E Tests: Pending

## Documentation

Full architecture and implementation docs in [`docs/`](./docs/00-INDEX.md).

## License

MIT
