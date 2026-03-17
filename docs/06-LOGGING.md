# Kestrel — Logging e Observabilidade

## Stack

- `log/slog` (stdlib Go 1.22+)
- JSON para produção, pretty text para desenvolvimento
- Arquivo + stdout simultâneo

## Níveis

```
DEBUG  — detalhes internos (tradução de chunks, seleção de conta)
INFO   — eventos normais (request recebido, response enviado, conta selecionada)
WARN   — degradação (fallback de conta, cooldown aplicado, retry)
ERROR  — falha (todas contas esgotadas, erro de conexão, panic recovered)
```

## Campos estruturados padrão

Todo log entry inclui automaticamente:

```json
{
  "time": "2026-03-16T12:00:00.000Z",
  "level": "INFO",
  "msg": "request_completed",
  "request_id": "req_abc123",
  "module": "proxy"
}
```

## Eventos por fase do request

### 1. Request recebido
```json
{
  "msg": "request_start",
  "module": "handler",
  "request_id": "req_abc123",
  "method": "POST",
  "path": "/v1/chat/completions",
  "model": "claude-sonnet-4-5",
  "stream": true,
  "api_key_id": "key_xyz",
  "api_key_name": "claude-code-main",
  "message_count": 15,
  "has_tools": true,
  "tool_count": 42
}
```

### 2. Conta selecionada
```json
{
  "msg": "account_selected",
  "module": "selector",
  "request_id": "req_abc123",
  "account_id": "acc_001",
  "account_name": "claude-pro-1",
  "reason": "sticky_session",
  "priority": 0,
  "available_accounts": 3
}
```

### 3. Request enviado ao Claude
```json
{
  "msg": "upstream_request",
  "module": "claude",
  "request_id": "req_abc123",
  "account_id": "acc_001",
  "url": "https://api.anthropic.com/v1/messages",
  "model": "claude-sonnet-4-5",
  "max_tokens": 8192,
  "stream": true
}
```

> **Nota sobre emissores de log:** Logs com `module: "handler"` e `module: "fallback"` são emitidos
> pelo middleware logging (adapter). Logs com `module: "claude"` e `module: "selector"` são emitidos
> pelos respectivos adapters (claude/client.go, sqlite/account_repo.go). O `slog.Logger` é injetado
> nos adapters via construtor. Use cases NUNCA emitem logs diretamente.

### 4. Response recebido (sucesso)
```json
{
  "msg": "upstream_response",
  "module": "claude",
  "request_id": "req_abc123",
  "account_id": "acc_001",
  "status": 200,
  "latency_ms": 1250,
  "input_tokens": 3500,
  "output_tokens": 850,
  "stop_reason": "end_turn"
}
```

### 5. Fallback de conta

> **Nota:** Logs de fallback são emitidos pelo adapter (middleware de logging), que observa os erros retornados pelo use case. O use case não loga diretamente.

```json
{
  "msg": "account_fallback",
  "module": "fallback",
  "level": "WARN",
  "request_id": "req_abc123",
  "failed_account_id": "acc_001",
  "failed_account_name": "claude-pro-1",
  "error_reason": "rate_limit",
  "cooldown_seconds": 4,
  "backoff_level": 2,
  "next_account_id": "acc_002",
  "retry_index": 1
}
```

### 6. Request completado
```json
{
  "msg": "request_completed",
  "module": "handler",
  "request_id": "req_abc123",
  "status": 200,
  "total_latency_ms": 1300,
  "account_id": "acc_001",
  "model": "claude-sonnet-4-5",
  "input_tokens": 3500,
  "output_tokens": 850,
  "retries": 0
}
```

### 7. Request falhou (todas contas esgotadas)
```json
{
  "msg": "request_failed",
  "module": "handler",
  "level": "ERROR",
  "request_id": "req_abc123",
  "status": 503,
  "error": "all accounts exhausted",
  "total_latency_ms": 4500,
  "retries": 3,
  "model": "claude-sonnet-4-5"
}
```

## Request Log (persistido em SQLite)

Para auditoria e debugging, cada request é salvo.

> **Nota:** O request log é persistido pelo middleware logging (`adapter/middleware/logging.go`) via interface `RequestLogger` (definida no mesmo middleware — é concern de infra, não de negócio). O adapter SQLite (`request_log_repo.go`) implementa a interface. O middleware envolve o handler e registra o resultado após o handler retornar. Nem o handler nem o use case conhecem o request log.

```sql
CREATE TABLE request_log (
    id              TEXT PRIMARY KEY,     -- request_id
    api_key_id      TEXT NOT NULL,
    api_key_name    TEXT,
    account_id      TEXT,
    account_name    TEXT,
    model           TEXT NOT NULL,
    status          INTEGER NOT NULL,
    input_tokens    INTEGER,
    output_tokens   INTEGER,
    latency_ms      INTEGER,
    retries         INTEGER DEFAULT 0,
    error           TEXT,
    stream          BOOLEAN NOT NULL,
    created_at      TEXT NOT NULL,

    FOREIGN KEY (api_key_id) REFERENCES api_keys(id),
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

CREATE INDEX idx_request_log_created ON request_log(created_at);
CREATE INDEX idx_request_log_account ON request_log(account_id);
CREATE INDEX idx_request_log_apikey ON request_log(api_key_id);
CREATE INDEX idx_request_log_status ON request_log(status);
```

## Formato de Erro (Error Response)

O proxy retorna erros no formato OpenAI para compatibilidade com clientes existentes:

```json
{
  "error": {
    "message": "All accounts exhausted",
    "type": "server_error",
    "code": "service_unavailable"
  }
}
```

### Mapeamento de tipos de erro

| HTTP Status | `type`                    | Exemplo de `code`       |
|-------------|---------------------------|-------------------------|
| 400         | `invalid_request_error`   | `bad_request`           |
| 401         | `authentication_error`    | `invalid_api_key`       |
| 403         | `permission_error`        | `model_not_allowed`     |
| 413         | `invalid_request_error`   | `request_too_large`     |
| 500         | `server_error`            | `internal_error`        |
| 503         | `server_error`            | `service_unavailable`   |

O handler de erros centralizado traduz qualquer erro interno para este formato antes de enviar ao cliente, garantindo que ferramentas como Claude Code interpretem as respostas corretamente.

## Graceful Shutdown

O servidor captura sinais de encerramento e finaliza conexões ativas de forma ordenada:

```go
// Captura SIGINT/SIGTERM
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

// Shutdown com deadline
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```

Comportamento durante shutdown:
- Novas conexões são recusadas imediatamente
- Requests SSE em andamento recebem deadline de 30s para completar
- Após o deadline, conexões restantes são encerradas forçadamente
- O processo sai com código 0 se todas as conexões fecharam limpo

Sequência de cleanup:
1. `server.Shutdown(shutdownCtx)` — para de aceitar novos requests
2. Aguardar goroutines em andamento (WaitGroup)
3. `db.Close()` — fechar conexões SQLite
4. Flush do log file

## Configuração

```env
# Nível de log (debug, info, warn, error)
LOG_LEVEL=info

# Formato (json, pretty)
LOG_FORMAT=json
```

> **Nota:** `LOG_FORMAT=pretty` usa `slog.TextHandler` para saída legível em desenvolvimento.
> `LOG_FORMAT=json` (default) usa `slog.JSONHandler` para produção.

### Variáveis planejadas (não implementadas)

```env
# Arquivo de log (vazio = só stdout)
LOG_FILE=/var/log/kestrel/app.log

# Log de request detalhado (inclui bodies — CUIDADO com PII)
LOG_REQUEST_BODY=false

# Retenção de request_log no SQLite (dias)
LOG_RETENTION_DAYS=30

# TTL de sessão (duração até expirar por inatividade)
SESSION_TTL=30m
```

## Implementação

```go
// internal/infra/logger/slog.go

func New(level string, format string) *slog.Logger {
    lvl := parseLevel(level)
    opts := &slog.HandlerOptions{Level: lvl}

    var handler slog.Handler
    if strings.ToLower(format) == "pretty" {
        handler = slog.NewTextHandler(os.Stdout, opts)
    } else {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    }

    log := slog.New(handler)
    slog.SetDefault(log)
    return log
}
```
