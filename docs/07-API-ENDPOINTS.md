# Kestrel — API Endpoints

## Proxy (compatível com OpenAI)

### POST /v1/chat/completions
Endpoint principal. Claude Code envia requests aqui.

**Headers:**
```
Authorization: Bearer omni-key-abc123
Content-Type: application/json
```

**Request (OpenAI format):**
```json
{
  "model": "claude-sonnet-4-5",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant"},
    {"role": "user", "content": "Hello"}
  ],
  "max_tokens": 4096,
  "temperature": 0.7,
  "stream": true,
  "tools": []
}
```

**Response (streaming):**
```
Content-Type: text/event-stream

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}

data: {"id":"chatcmpl-...","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-...","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{...}}

data: [DONE]
```

**Response (non-streaming):**
```json
{
  "id": "chatcmpl-msg_123",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "claude-sonnet-4-5",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Hello! How can I help?"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 8,
    "total_tokens": 33
  }
}
```

### GET /v1/models
Lista modelos disponíveis.

```json
{
  "object": "list",
  "data": [
    {"id": "claude-sonnet-4-5", "object": "model", "owned_by": "anthropic"},
    {"id": "claude-opus-4-5", "object": "model", "owned_by": "anthropic"},
    {"id": "claude-haiku-3-5", "object": "model", "owned_by": "anthropic"}
  ]
}
```

### Formato de Erro

Todos os endpoints retornam erros no formato OpenAI:

```json
{
  "error": {
    "message": "Invalid model: gpt-4",
    "type": "invalid_request_error",
    "code": "bad_request"
  }
}
```

Exemplos por cenário:
- API key inválida → `401`, `type: "authentication_error"`, `code: "invalid_api_key"`
- Modelo não permitido → `403`, `type: "permission_error"`, `code: "model_not_allowed"`
- Todas contas esgotadas → `503`, `type: "server_error"`, `code: "service_unavailable"`
- Body acima do limite → `413`, `type: "invalid_request_error"`, `code: "request_too_large"`

---

## Health

### GET /health
```json
{
  "status": "ok",
  "version": "dev",
  "uptime_seconds": 3600.5
}
```

---

## Admin API

Protegida por header `X-Admin-Key` (configurado via env var).

> **Rate limiting (v1):** sem rate limiting na admin API. Risco aceito — acesso restrito por `X-Admin-Key`. Futuro: middleware de rate limit se exposto publicamente.

### Accounts

#### GET /admin/accounts
Lista todas as contas.

```json
{
  "data": [
    {
      "id": "acc_001",
      "name": "claude-pro-1",
      "base_url": "https://api.anthropic.com",
      "status": "active",
      "priority": 0,
      "cooldown_until": null,
      "backoff_level": 0,
      "last_used_at": "2026-03-16T12:00:00Z",
      "last_error": null,
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

> Nota: `api_key` NUNCA é retornado em listagens. Apenas mascarado (sk-ant-...xxx).

#### POST /admin/accounts
Cria nova conta.

```json
{
  "name": "claude-pro-2",
  "api_key": "sk-ant-api03-...",
  "base_url": "https://api.anthropic.com",
  "priority": 1
}
```

#### PUT /admin/accounts/:id
Atualiza conta.

```json
{
  "name": "claude-pro-2-renamed",
  "priority": 0
}
```

#### DELETE /admin/accounts/:id
Remove conta.

#### POST /admin/accounts/:id/reset
Reseta cooldown e erros de uma conta.

```json
{
  "status": "active",
  "cooldown_until": null,
  "backoff_level": 0,
  "last_error": null
}
```

### API Keys

#### GET /admin/keys
Lista API keys do proxy.

```json
{
  "data": [
    {
      "id": "key_001",
      "name": "claude-code-main",
      "prefix": "omni-key-abc",
      "is_active": true,
      "allowed_models": [],
      "created_at": "2026-03-01T00:00:00Z",
      "last_used_at": "2026-03-16T12:00:00Z"
    }
  ]
}
```

#### POST /admin/keys
Cria nova API key. Retorna o valor completo APENAS nesta resposta.

```json
// Request
{
  "name": "claude-code-secondary",
  "allowed_models": ["claude-sonnet-4-5"]
}

// Response (ÚNICA vez que mostra a key completa)
{
  "id": "key_002",
  "key": "omni-key-f8a3b2c1d9e0...",
  "name": "claude-code-secondary",
  "allowed_models": ["claude-sonnet-4-5"]
}
```

#### DELETE /admin/keys/:id
Revoga API key.

### Request Logs

#### GET /admin/logs
Lista requests recentes.

**Query params:**
- `limit` (default 50, max 500)
- `offset`
- `status` (filtro por HTTP status)
- `account_id`
- `api_key_id`
- `model`
- `from` (ISO 8601)
- `to` (ISO 8601)

```json
{
  "data": [
    {
      "id": "req_abc123",
      "api_key_name": "claude-code-main",
      "account_name": "claude-pro-1",
      "model": "claude-sonnet-4-5",
      "status": 200,
      "input_tokens": 3500,
      "output_tokens": 850,
      "latency_ms": 1250,
      "retries": 0,
      "stream": true,
      "created_at": "2026-03-16T12:00:00Z"
    }
  ],
  "total": 1523,
  "limit": 50,
  "offset": 0
}
```

---

## Roteamento

```
/v1/*                → Proxy (requer Bearer token)
/health              → Health (público)
/admin/*             → Admin (requer X-Admin-Key)
/app/*               → Frontend SPA (static files)
```
