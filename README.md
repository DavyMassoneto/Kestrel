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
- **Multi-account rotation** — automatic fallback when an account is rate-limited or exhausted
- **SSE streaming** — transparent passthrough with format translation
- **Exponential backoff** — per-account cooldown with automatic recovery
- **SQLite persistence** — accounts, API keys, and request logs
- **Admin API** — CRUD for accounts and keys, request log queries
- **Web dashboard** — React SPA embedded in the binary (single binary deploy)
- **Structured logging** — full request tracing with slog
- **Encryption at rest** — AES-256-GCM for stored API keys

## Stack

| Component   | Choice                    |
|-------------|---------------------------|
| Language    | Go 1.22+                  |
| Architecture| Clean Architecture + SOLID|
| Router      | chi                       |
| Database    | SQLite (modernc.org/sqlite)|
| Logging     | slog (stdlib)             |
| Frontend    | React 19 + Vite + Tailwind|
| Deploy      | Single binary (embed.FS)  |

## Documentation

Full architecture and implementation docs in [`docs/`](./docs/00-INDEX.md).

## Status

Under development.

## License

MIT
