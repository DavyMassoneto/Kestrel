# OmniRouter Go — Visão do Projeto

## O que é

Um proxy reverso de alta performance escrito em Go que roteia requisições
no formato OpenAI para a API da Anthropic (Claude), gerenciando múltiplas
contas com rotação automática por esgotamento de cota.

## Por que reescrever

O OmniRoute atual roda em Next.js — um framework de aplicação web sendo
usado como API gateway. O overhead é inaceitável para um proxy:

| Métrica          | Next.js (atual) | Go (alvo)    |
|------------------|-----------------|--------------|
| Latência proxy   | ~5-15ms         | ~0.1-0.5ms   |
| Requests/s (SSE) | ~2-5k           | ~50-100k     |
| Memória idle     | ~300MB          | ~15MB        |
| Cold start       | ~3-5s           | ~50ms        |

## Escopo v1 — Apenas o necessário

### Inclui
- Proxy transparente OpenAI → Claude (passthrough SSE)
- Multi-conta com rotação por esgotamento de cota
- Autenticação de clientes via API key (compatível com Claude Code)
- Persistência SQLite (contas, keys, sessões)
- Logging estruturado com rastreamento completo de requests
- Health check endpoint
- API admin para CRUD de contas e keys
- Frontend SPA (React + Vite) embarcado no binário via `embed.FS`

### Não inclui (v1)
- Suporte a outros providers (OpenAI, Gemini, etc.)
- Pricing sync, cloud sync
- Codex service tier
- Combos multi-modelo
- Electron wrapper

## Decisões técnicas

| Decisão              | Escolha                           | Razão                                      |
|----------------------|-----------------------------------|--------------------------------------------|
| Linguagem            | Go 1.22+                         | Performance + simplicidade + ecossistema    |
| Arquitetura          | Clean Architecture + SOLID        | Testabilidade + extensibilidade             |
| HTTP Router          | `net/http` + `chi`               | Leve, idiomático, middleware composable     |
| SQLite               | `modernc.org/sqlite`             | Pure Go, sem CGO, cross-compile trivial     |
| Logging              | `log/slog` (stdlib)              | Estruturado, zero dependência, Go nativo    |
| Config               | Env vars + `caarlos0/env`        | Simples, 12-factor                          |
| Testes               | `testing` + `testify`            | Stdlib + assertions legíveis                |
| SSE                  | Manual (text/event-stream)       | Protocolo simples, zero overhead de lib     |
| JSON                 | `encoding/json` (stdlib)         | Suficiente; `sonic` se benchmark exigir     |

## Princípios

1. **Zero overhead desnecessário** — cada middleware, abstração e alocação precisa justificar sua existência
2. **Fail fast, recover smart** — erro de conta = próxima conta, não retry infinito
3. **Observabilidade total** — cada request rastreável do início ao fim
4. **Dependency rule** — domínio não conhece infra; infra implementa interfaces do domínio
5. **Testável em isolamento** — cada camada testável com mocks das interfaces
6. **TDD obrigatório** — red-green-refactor em cada feature, coverage 100%
7. **Concurrency-safe by design** — mutex, WAL, transações atômicas
