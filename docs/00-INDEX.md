# OmniRouter Go — Índice de Documentação

Reconstrução do OmniRoute em Go, focado em performance extrema.
Escopo v1: apenas Claude, multi-conta com rotação por quota.

## Documentos

| # | Documento | Conteúdo |
|---|-----------|----------|
| 01 | [VISION](./01-VISION.md) | Escopo, decisões técnicas, princípios |
| 02 | [ARCHITECTURE](./02-ARCHITECTURE.md) | Clean Architecture, camadas, estrutura de diretórios, entidades, ports |
| 03 | [REQUEST-FLOW](./03-REQUEST-FLOW.md) | Fluxo completo de um request, diagramas, SSE streaming |
| 04 | [TRANSLATION](./04-TRANSLATION.md) | Tradução OpenAI ↔ Claude (request e response, streaming e non-streaming) |
| 05 | [ACCOUNT-ROTATION](./05-ACCOUNT-ROTATION.md) | Seleção de conta, backoff exponencial, classificação de erros, retry loop |
| 06 | [LOGGING](./06-LOGGING.md) | Eventos por fase, campos estruturados, request log SQLite, configuração |
| 07 | [API-ENDPOINTS](./07-API-ENDPOINTS.md) | Todos os endpoints (proxy, health, admin CRUD, logs) |
| 08 | [IMPLEMENTATION-PHASES](./08-IMPLEMENTATION-PHASES.md) | 8 fases de implementação com arquivos e entregáveis |
| 09 | [FRONTEND](./09-FRONTEND.md) | SPA React + Vite, páginas, integração com Go (embed), auto-refresh |

## Decisões rápidas

- **Linguagem:** Go 1.22+
- **Arquitetura:** Clean Architecture + SOLID
- **Router:** chi
- **DB:** SQLite (modernc.org/sqlite, pure Go)
- **Logger:** slog (stdlib)
- **Provider:** apenas Claude (v1)
- **Formato:** OpenAI ↔ Claude tradução bidirecional
- **Auth:** API keys próprias do proxy
- **Frontend:** React 19 + Vite + Tailwind + shadcn/ui + TanStack Query
- **Deploy frontend:** embed.FS no binário Go (single binary)

## Contexto

Este projeto substitui o OmniRoute (Next.js) para o caso de uso específico
de proxy Claude com rotação de contas. O OmniRoute original continuará existindo
para quem precisa de multi-provider e dashboard UI.
