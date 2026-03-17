# Kestrel — OAuth

## Visao Geral

Kestrel suporta OAuth Authorization Code com PKCE para autenticacao de contas Anthropic.
O fluxo e opcional — habilitado quando `OAUTH_CLIENT_ID` e configurado.

## Fluxo

```
Browser                   Kestrel                    Anthropic
───────                   ──────────                  ─────────
   │                           │                          │
   │  GET /api/oauth/authorize │                          │
   │ ─────────────────────────▶│                          │
   │                           │                          │
   │                    gera PKCE (verifier + challenge)   │
   │                    gera state (random)                │
   │                    armazena state → verifier           │
   │                           │                          │
   │  302 Redirect             │                          │
   │ ◀─────────────────────────│                          │
   │                           │                          │
   │  GET /oauth/authorize?client_id=...&code_challenge=...│
   │ ─────────────────────────────────────────────────────▶│
   │                           │                          │
   │  [usuario autoriza]       │                          │
   │                           │                          │
   │  302 → /api/oauth/callback?code=...&state=...        │
   │ ◀────────────────────────────────────────────────────│
   │                           │                          │
   │  GET /api/oauth/callback  │                          │
   │ ─────────────────────────▶│                          │
   │                           │                          │
   │                    valida state                       │
   │                    recupera verifier                  │
   │                           │                          │
   │                           │  POST /oauth/token        │
   │                           │  code + code_verifier     │
   │                           │ ────────────────────────▶│
   │                           │                          │
   │                           │  { access_token,          │
   │                           │    refresh_token,         │
   │                           │    expires_in }           │
   │                           │ ◀────────────────────────│
   │                           │                          │
   │  { access_token, ...}     │                          │
   │ ◀─────────────────────────│                          │
```

## Endpoints

### GET /api/oauth/authorize

Inicia o fluxo OAuth. Gera PKCE e state, redireciona o browser para o provider.

**Parametros de query enviados ao provider:**
- `client_id` — ID do cliente OAuth
- `redirect_uri` — URL de callback configurada
- `response_type` — `code`
- `code_challenge` — SHA-256 do verifier (S256)
- `code_challenge_method` — `S256`
- `state` — token aleatorio anti-CSRF
- `scope` — scope configurado (se definido)

**Response:** `302 Found` com redirect para o authorization endpoint do provider.

### GET /api/oauth/callback

Recebe o callback do provider apos autorizacao.

**Query params recebidos:**
- `code` — authorization code
- `state` — token para validacao anti-CSRF

**Response (sucesso):**
```json
{
  "access_token": "ant-at-...",
  "refresh_token": "ant-rt-...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**Erros:**
- `400` — `code` ou `state` ausente, state invalido/expirado
- `400` — provider retornou erro (campo `error` presente no callback)
- `502` — falha na troca do code por tokens junto ao provider

## PKCE

O fluxo usa PKCE (Proof Key for Code Exchange) para seguranca:

1. `code_verifier` — 32 bytes random, base64url-encoded
2. `code_challenge` — SHA-256 do verifier, base64url-encoded
3. O verifier e armazenado em memoria (sync.Map), indexado pelo state
4. Na troca de code por token, o verifier e enviado ao provider para validacao
5. O state e consumido atomicamente (LoadAndDelete) — uso unico

## Configuracao

| Variavel            | Default                                          | Descricao                          |
|---------------------|--------------------------------------------------|------------------------------------|
| `OAUTH_CLIENT_ID`   | —                                                | Habilita OAuth quando configurado  |
| `OAUTH_REDIRECT_URI`| `http://localhost:8080/api/oauth/callback`       | URL de callback                    |
| `OAUTH_AUTH_URL`    | `https://console.anthropic.com/oauth/authorize`  | Endpoint de autorizacao do provider|
| `OAUTH_TOKEN_URL`   | `https://console.anthropic.com/oauth/token`      | Endpoint de token do provider      |

## Schema (migracao 004)

A migracao `004_oauth_accounts.sql` estende a tabela `accounts` com campos para OAuth:

```sql
ALTER TABLE accounts ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'api_key';
ALTER TABLE accounts ADD COLUMN access_token TEXT;
ALTER TABLE accounts ADD COLUMN refresh_token TEXT;
ALTER TABLE accounts ADD COLUMN token_expires_at TEXT;
ALTER TABLE accounts ADD COLUMN oauth_email TEXT;
ALTER TABLE accounts ADD COLUMN oauth_scope TEXT;
```

- `auth_type` — `api_key` (default, compativel com contas existentes) ou `oauth`
- `access_token` / `refresh_token` — tokens OAuth (encriptados at rest)
- `token_expires_at` — expiracao do access token (ISO 8601)
- `oauth_email` / `oauth_scope` — metadata da conta OAuth

## Token Refresh

O adapter `oauth.Client` implementa `RefreshToken()` para renovacao de tokens:

```go
func (c *Client) RefreshToken(ctx context.Context, cfg Config, refreshToken string) (TokenResponse, error)
```

Envia `grant_type=refresh_token` com o `refresh_token` e `client_id` para o token endpoint.

## Arquivos

```
internal/adapter/oauth/claude.go       — Client HTTP, PKCE, AuthorizationURL, ExchangeCode, RefreshToken
internal/adapter/oauth/claude_test.go  — Testes do client OAuth
internal/adapter/handler/oauth.go      — OAuthHandler (authorize + callback endpoints)
internal/adapter/handler/oauth_test.go — Testes dos endpoints OAuth
internal/infra/cfg/config.go           — Config com campos OAuth
migrations/004_oauth_accounts.sql      — Migracao para campos OAuth na tabela accounts
```
