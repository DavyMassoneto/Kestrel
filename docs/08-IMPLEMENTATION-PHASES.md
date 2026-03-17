# Kestrel — Fases de Implementação

## Metodologia: TDD (Red-Green-Refactor)

Toda feature segue o ciclo:
1. **RED** — Escrever teste que falha
2. **GREEN** — Implementar o mínimo para passar
3. **REFACTOR** — Limpar sem quebrar testes

Coverage target: 100% em `domain/` e `usecase/`. Mínimo 90% em `adapter/`.
Nenhum merge sem `go test ./... -coverprofile` passando.

---

## Fase 1 — Skeleton (infra + server + middlewares) [DONE]

**Objetivo:** HTTP server funcional com health check e middlewares. Sem chat handler — o chat entra na Fase 2 junto com o domínio.

```
Arquivos de produção:
  cmd/kestrel/main.go
  internal/infra/cfg/config.go
  internal/infra/logger/slog.go
  internal/adapter/handler/health.go
  internal/adapter/middleware/requestid.go
  internal/adapter/middleware/recovery.go
  go.mod
  Makefile

Arquivos de teste:
  internal/infra/cfg/config_test.go
  internal/adapter/handler/health_test.go
  internal/adapter/middleware/requestid_test.go
  internal/adapter/middleware/recovery_test.go
```

**Entregável:** `curl localhost:8080/health` → resposta OK, testes passam, server inicia e para limpo.

**Critério de aceite:** Testes verdes, health funcional, middlewares cobertos.

**Nota:** O chat handler NÃO entra nesta fase — entra na Fase 2 junto com o domínio. Isso evita escrever testes descartáveis para scaffolding temporário. Fase 1 valida infraestrutura (server, config, logger, middlewares).

---

## Fase 2 — Domínio + Tradução [DONE]

**Objetivo:** Claude Code se conecta e funciona transparentemente. Domínio modelado com entidades, VOs e erros.

```
Arquivos de produção:
  internal/domain/entity/account.go
  internal/domain/entity/apikey.go
  internal/domain/entity/session.go
  internal/domain/vo/id.go
  internal/domain/vo/cooldown.go
  internal/domain/vo/model.go
  internal/domain/vo/error_classification.go
  internal/domain/vo/chat.go
  internal/domain/vo/credentials.go
  internal/domain/errs/errors.go
  internal/adapter/claude/client.go
  internal/adapter/claude/translator.go
  internal/adapter/claude/errors.go
  internal/adapter/claude/sse.go
  internal/usecase/proxy_chat.go           # versão simplificada (single account, sem fallback, sem session)
  internal/usecase/proxy_stream.go         # versão simplificada (single account, sem fallback, sem session)
  internal/adapter/handler/chat.go
  internal/adapter/handler/translator.go
  internal/adapter/handler/models.go
  internal/adapter/sse/writer.go

Arquivos de teste:
  internal/domain/entity/account_test.go
  internal/domain/entity/apikey_test.go
  internal/domain/entity/session_test.go
  internal/domain/vo/id_test.go
  internal/domain/vo/cooldown_test.go
  internal/domain/vo/model_test.go
  internal/domain/vo/credentials_test.go
  internal/domain/vo/error_classification_test.go
  internal/domain/vo/chat_test.go
  internal/adapter/claude/client_test.go
  internal/adapter/claude/translator_test.go
  internal/adapter/claude/errors_test.go
  internal/usecase/proxy_chat_test.go      # versão simplificada
  internal/usecase/proxy_stream_test.go    # versão simplificada
  internal/adapter/claude/sse_test.go
  internal/adapter/handler/chat_test.go
  internal/adapter/handler/translator_test.go
```

**Entregável:** Claude Code aponta para o proxy e funciona, domínio 100% coberto.

**Critério de aceite:** Coverage 100% em `domain/`, tradução OpenAI ↔ Claude funcional.

**Nota:** Na Fase 2, ProxyChatUseCase/ProxyStreamUseCase são implementados em versão simplificada (single account, sem fallback, sem session). Fallback e session entram na Fase 5.

---

## Fase 3 — SQLite + Persistência [DONE]

**Objetivo:** Contas e keys persistidas. CRUD admin funcional.

```
Arquivos de produção:
  internal/adapter/sqlite/db.go
  internal/adapter/sqlite/migrations.go
  internal/adapter/sqlite/account_repo.go
  internal/adapter/sqlite/apikey_repo.go
  internal/adapter/crypto/aes.go
  internal/adapter/handler/admin.go
  internal/usecase/admin_account.go
  internal/usecase/admin_apikey.go
  internal/usecase/ports.go
  migrations/embed.go
  migrations/001_accounts.sql
  migrations/002_apikeys.sql

Arquivos de teste:
  internal/adapter/sqlite/db_test.go (implícito via integration tests)
  internal/adapter/sqlite/account_repo_test.go
  internal/adapter/sqlite/apikey_repo_test.go
  internal/adapter/sqlite/migrations_test.go
  internal/adapter/crypto/aes_test.go
  internal/usecase/admin_account_test.go
  internal/usecase/admin_apikey_test.go
  internal/adapter/handler/admin_test.go
```

**Entregável:** Admin API funcional, contas salvas no SQLite, testes de integração passam.

**Critério de aceite:** CRUD completo, testes de integração com SQLite in-memory verdes.

---

## Fase 4 — Autenticação + Logging Middleware [DONE]

**Objetivo:** Requests precisam de API key válida. Request logging funcional.

```
Arquivos de produção:
  internal/usecase/authenticate.go            # AuthenticateUseCase + APIKeyFinder interface
  internal/adapter/middleware/auth.go          # Bearer token validation, APIKey no context
  internal/adapter/middleware/logging.go       # RequestLogger interface + slog request logging

Arquivos de teste:
  internal/usecase/authenticate_test.go
  internal/adapter/middleware/auth_test.go
  internal/adapter/middleware/logging_test.go
```

**Entregável:** Requests sem key → 401. Key inválida → 401. Bearer auth no /v1/* routes.

**Critério de aceite:** Testes cobrindo todos os cenários de auth (sem key, key inválida, key válida, Bearer parsing).

---

## Fase 5 — Multi-conta + Fallback [DONE]

**Objetivo:** Rotação automática quando conta esgota.

```
Arquivos de produção:
  internal/usecase/select_account.go
  internal/usecase/handle_fallback.go
  internal/usecase/proxy_chat.go              # versão completa (multi-conta, fallback, session)
  internal/usecase/proxy_stream.go            # versão completa (multi-conta, fallback, session)
  internal/usecase/manage_session.go
  internal/adapter/session/memory.go

Arquivos de teste:
  internal/usecase/select_account_test.go
  internal/usecase/handle_fallback_test.go
  internal/usecase/proxy_chat_test.go
  internal/usecase/proxy_stream_test.go
  internal/usecase/manage_session_test.go
  internal/adapter/session/memory_test.go
```

**Entregável:** Conta 1 dá 429 → proxy tenta conta 2 → request sucede, fallback funciona.

**Critério de aceite:** Coverage 100% nos use cases, cenários de fallback cobertos (rate limit, quota, overloaded, auth error, unknown).

---

## Fase 6 — Logging + Request Log [DONE]

**Objetivo:** Rastreamento completo de cada request.

```
Arquivos de produção:
  migrations/003_request_log.sql
  internal/adapter/sqlite/request_log_repo.go
  internal/adapter/handler/admin.go (+ GET /admin/logs)

Arquivos de teste:
  internal/adapter/sqlite/request_log_repo_test.go
  internal/adapter/handler/admin_test.go
```

**Entregável:** Cada request logado com tokens, latência, conta usada, retries. Endpoint consultável.

**Critério de aceite:** Logs persistidos no SQLite, endpoint de consulta funcional, testes passando.

---

## Fase 7 — Frontend + Deploy

**Objetivo:** Interface web e deploy em produção.

```
Arquivos de produção:
  web/                    (React SPA)
  Dockerfile
  docker-compose.yml
  cmd/kestrel/embed.go (embed.FS setup)
  .env.example

Arquivos de teste:
  web/ — vitest + testing-library para componentes críticos
```

**Entregável:** Binário único com frontend embedado funciona em produção.

**Critério de aceite:** `docker run kestrel` funcional, frontend acessível, testes frontend verdes.

---

## Fase 8 — Integration + E2E Tests

**Objetivo:** Suite completa de testes de integração e E2E.

```
Arquivos de produção:
  nenhum novo

Arquivos de teste:
  tests/integration/   (proxy flow completo com SQLite real)
  tests/e2e/           (curl-based smoke tests)
```

**Entregável:** Suite completa verde, coverage report.

**Critério de aceite:** Fluxo completo testado (request → auth → seleção de conta → proxy → fallback → resposta), coverage report gerado.

---

## Ordem de dependência

```
Fase 1 (skeleton)
  └─▶ Fase 2 (domínio + tradução)
       └─▶ Fase 3 (SQLite)
            ├─▶ Fase 4 (auth)
            └─▶ Fase 5 (multi-conta)
                 └─▶ Fase 6 (logging DB)
                      └─▶ Fase 7 (frontend + deploy)
                           └─▶ Fase 8 (integration + E2E)
```

Fases 4 e 5 podem ser paralelas após fase 3.
