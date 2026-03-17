# OmniRouter Go — Rotação de Contas

## Princípio

Simples: conta atual esgotou → pega a próxima com tokens disponíveis.
Sem estratégias complexas. Sem round-robin. Sem power-of-two-choices.

## Estados de uma conta

```
         ┌─────────┐
         │ ACTIVE   │ ← estado normal, aceita requests
         └────┬─────┘
              │ erro 429/529/5xx
              ▼
         ┌─────────┐
         │ COOLDOWN │ ← temporariamente indisponível, backoff exponencial
         └────┬─────┘
              │ cooldown expirou
              ▼
         ┌─────────┐
         │ ACTIVE   │ ← volta a aceitar requests
         └──────────┘

         ┌─────────┐
         │ ACTIVE   │
         └────┬─────┘
              │ erro 401/403 (auth inválida)
              ▼
         ┌──────────┐
         │ DISABLED  │ ← precisa intervenção manual (reconfigurar key)
         └──────────┘
```

## Algoritmo de seleção

```
SelectAccount(preferredID, excludeID, now):
  1. Buscar todas contas WHERE:
     - status = 'active'
     - (cooldown_until IS NULL OR cooldown_until < NOW())
     - id != excludeID (se fornecido)

  2. Se preferredID fornecido E está na lista:
     → retorna preferred (sticky routing para mesma sessão)

  3. Ordenar por:
     a. priority ASC        (admin configura qual conta usar primeiro)
     b. last_used_at ASC    (LRU — a menos usada recentemente primeiro)

  4. Se lista vazia:
     → retorna ErrAllAccountsExhausted

  5. Retorna accounts[0]
```

## Backoff exponencial

```
Nível 0:  1 segundo
Nível 1:  2 segundos
Nível 2:  4 segundos
Nível 3:  8 segundos
Nível 4:  16 segundos
Nível 5:  32 segundos
Nível 6:  64 segundos
Nível 7+: 120 segundos (cap)

Fórmula: min(2^level, 120) segundos
Reset: sucesso → backoffLevel = 0
```

## Classificação de erros

### Domínio: enum + behavior (domain/vo/error_classification.go)

O domínio define os tipos de erro e seus behaviors. **NÃO conhece HTTP.**

```go
// domain/vo/error_classification.go

type ErrorClassification string

const (
    ErrRateLimit      ErrorClassification = "rate_limit"
    ErrQuotaExhausted ErrorClassification = "quota_exhausted"
    ErrOverloaded     ErrorClassification = "overloaded"
    ErrServer         ErrorClassification = "server_error"
    ErrAuth           ErrorClassification = "authentication_error"
    ErrClient         ErrorClassification = "client_error"
    ErrUnknown        ErrorClassification = "unknown"
)

func (c ErrorClassification) ShouldFallback() bool {
    return c != ErrClient && c != ErrUnknown
}

func (c ErrorClassification) DefaultCooldownDuration() time.Duration {
    switch c {
    case ErrRateLimit:      return 0                    // usa backoff exponencial
    case ErrQuotaExhausted: return 5 * time.Minute      // 5 minutos
    case ErrOverloaded:     return 30 * time.Second
    case ErrServer:         return 60 * time.Second
    case ErrAuth:           return 0                    // auth error → Disable(), não usa cooldown
    case ErrClient:         return 0                    // client error → retorna direto, sem cooldown
    case ErrUnknown:        return 0                    // ApplyCooldown rejeita ErrUnknown, nunca usado
    default:                return 0
    }
}
```

### Adapter: classificação HTTP (adapter/claude/errors.go)

A conversão de HTTP status/body para `ErrorClassification` é responsabilidade
do adapter — é detalhe de infraestrutura, não regra de domínio.

```go
// adapter/claude/errors.go

// ProviderError é o tipo interno do adapter que implementa a interface
// ClassifiedError do use case. O use case extrai a classificação via
// errors.As(err, &classErr) sem importar este tipo.
type ProviderError struct {
    classification vo.ErrorClassification
    message        string
    originalStatus int  // interno ao adapter, não exposto
}

func (e *ProviderError) Error() string                         { return e.message }
func (e *ProviderError) Classification() vo.ErrorClassification { return e.classification }

// classifyHTTPError converte status HTTP + body em ErrorClassification.
// Lógica específica do provider Claude — pertence ao adapter.
func classifyHTTPError(status int, body string) vo.ErrorClassification {
    // Pattern matching no body primeiro (mais específico)
    switch {
    case contains(body, "rate limit"):
        return vo.ErrRateLimit
    case contains(body, "quota exceeded"):
        return vo.ErrQuotaExhausted
    case contains(body, "overloaded"), contains(body, "capacity"):
        return vo.ErrOverloaded
    case contains(body, "no credentials"):
        return vo.ErrAuth
    }

    // Fallback por status HTTP
    switch status {
    case 429:
        return vo.ErrRateLimit
    case 401, 403:
        return vo.ErrAuth
    case 529:
        return vo.ErrOverloaded
    case 500, 502, 503, 504:
        return vo.ErrServer
    case 400:
        return vo.ErrClient
    default:
        return vo.ErrUnknown
    }
}
```

### Unknown errors

Erros desconhecidos (`ErrUnknown`) **não** fazem fallback por padrão.
Justificativa: se o erro é desconhecido, trocar de conta pode não resolver e pode
esgotar todas as contas inutilmente. Abordagem conservadora — melhor retornar o erro
ao cliente do que queimar contas sem motivo.

## Retry loop no ProxyChatUseCase

O use case trabalha com tipos do domínio (`ChatRequest`, não structs da OpenAI).
O body já chega como struct bufferizado pelo handler — retry funciona naturalmente
sem precisar reler o body.

A entidade `Account` gerencia sua própria transição de estado via `ApplyCooldown()`.
Para erros de autenticação, o `HandleFallback` chama `account.Disable()` diretamente
em vez de `ApplyCooldown()`:

```go
func (a *Account) ApplyCooldown(classification ErrorClassification, now time.Time) error {
    if a.status == StatusDisabled {
        return ErrAccountDisabled
    }
    if classification == ErrAuth || classification == ErrClient || classification == ErrUnknown {
        return ErrInvalidClassification // ApplyCooldown aceita apenas erros transientes
    }

    baseDuration := classification.DefaultCooldownDuration()
    var cooldownDuration time.Duration

    // Nil-check: conta sem cooldown anterior inicia no nível 0
    // currentLevel = nível anterior + 1 (incremento progressivo)
    currentLevel := 0
    if a.cooldown != nil {
        currentLevel = a.cooldown.BackoffLevel() + 1
    }

    if baseDuration > 0 {
        // Duração fixa: quota_exhausted=5min, overloaded=30s, server_error=60s
        cooldownDuration = baseDuration
    } else {
        // Backoff exponencial: rate_limit → min(2^level, 120) segundos
        seconds := min(1<<currentLevel, 120)
        cooldownDuration = time.Duration(seconds) * time.Second
    }

    // BackoffLevel armazena o nível USADO no cálculo (não o próximo).
    // Na próxima chamada, currentLevel = BackoffLevel + 1 (incremento antes do cálculo).
    //
    // Se o tipo de erro mudou, reset do backoff level.
    // Backoff level só acumula para erros consecutivos do MESMO tipo.
    usedLevel := currentLevel
    if a.cooldown != nil && a.cooldown.Reason() != classification {
        usedLevel = 0 // reset: primeiro nível do novo tipo
        // Recalcular duração com nível resetado
        if baseDuration == 0 {
            seconds := min(1<<usedLevel, 120)
            cooldownDuration = time.Duration(seconds) * time.Second
        }
    }

    c := vo.NewCooldown(now.Add(cooldownDuration), usedLevel, classification)
    a.cooldown = &c
    a.status = StatusCooldown
    return nil
}
```

O adapter que implementa `ChatSender` retorna erros que implementam a interface
`ClassifiedError` (definida no use case) — o use case extrai a classificação via
`errors.As(err, &classErr)` sem importar tipos do adapter. O use case **nunca vê
HTTP status codes**.

> **Nota ErrAuth:** `ShouldFallback()` retorna `true` para `ErrAuth` — o retry loop
> tenta outra conta. Mas `ApplyCooldown()` rejeita `ErrAuth` — o `HandleFallback`
> chama `account.Disable()` diretamente. A conta é desabilitada permanentemente,
> mas o request pode ter sucesso com outra conta. Fallback SIM, cooldown NÃO.

```
excludeID = nil
var lastErr error
const maxRetries = 10 // safety cap — defesa em profundidade

// Loop até selectAccount retornar erro (sem contas disponíveis).
// Safety cap de 10 iterações previne loop infinito em caso de bug.
// O context do request também serve como safety net (timeout do server).
for i := 0; i < maxRetries; i++ {
    account, err = selectAccount.Execute(ctx, session.AccountID, excludeID, clock.Now())
    if err != nil {
        return errs.ErrAllAccountsExhausted
    }

    creds = account.Credentials()  // API key nunca exposta via getter
    chatResp, err = chatSender.SendChat(ctx, creds, chatReq)

    if err == nil {
        // Sucesso
        account.ClearError()
        account.RecordUsage(clock.Now())
        accountWriter.UpdateStatus(ctx, account)
        session.BindAccount(account.ID())
        session.RecordRequest(clock.Now())
        sessionWriter.Save(ctx, session)
        return chatResp, nil
    }

    // Extrair classificação do erro via interface (sem ver HTTP nem tipos do adapter)
    var classErr ClassifiedError
    if !errors.As(err, &classErr) {
        // Erro não é do provider (ex: timeout, connection refused)
        return err
    }

    // Delega tratamento ao sub-use-case HandleFallback
    result = handleFallback.Execute(ctx, account, classErr.Classification())

    if !result.ShouldFallback {
        // ClientError ou Unknown — retorna direto sem trocar conta
        return err
    }

    // Tenta próxima conta
    excludeID = account.ID()
    lastErr = err
}

return errs.ErrAllAccountsExhausted
```

**Logging do fallback:** o use case retorna o erro para o handler (adapter).
O handler/middleware de logging registra fallbacks, retries e erros finais.
O use case NÃO chama logger diretamente — observabilidade é responsabilidade
do adapter layer.

## Concorrência na seleção de conta

A seleção de conta usa `BEGIN IMMEDIATE` no SQLite (transação exclusiva de escrita).
Isso evita race condition onde duas goroutines selecionam a mesma conta simultaneamente.

```sql
BEGIN IMMEDIATE;

SELECT id, name, api_key, base_url, priority, backoff_level, last_used_at
FROM accounts
WHERE status = 'active'
  AND (cooldown_until IS NULL OR cooldown_until < datetime('now'))
  AND (id != :excludeID OR :excludeID IS NULL)
-- Source of truth para disponibilidade é Account.IsAvailable(now).
-- O SQL implementa a mesma lógica como otimização de query.
-- Testes unitários da entidade garantem equivalência.
ORDER BY
  priority ASC,
  last_used_at ASC;

-- A lógica de preferência (sticky routing via preferredID) fica no use case
-- SelectAccount, que verifica se a conta preferida está na lista retornada.

-- Marca a conta como usada dentro da mesma transação
UPDATE accounts SET last_used_at = datetime('now') WHERE id = :selectedID;

COMMIT;
```

`BEGIN IMMEDIATE` garante que apenas uma goroutine por vez executa o SELECT + UPDATE.
Outras goroutines que tentarem iniciar uma transação ficam bloqueadas até o COMMIT,
eliminando a possibilidade de duas requests escolherem a mesma conta.

## Sticky routing por sessão

```
Sessão = fingerprint baseado em: APIKeyID + Model

Primeira request:
  → session não existe
  → selectAccount escolhe por prioridade
  → session.BindAccount(accountID)

Requests seguintes (mesma sessão):
  → session.AccountID existe
  → selectAccount prefere essa conta (se disponível)
  → mantém na mesma conta (prompt cache, consistência)

Conta da sessão ficou indisponível:
  → selectAccount ignora preferredID
  → escolhe próxima disponível
  → session.BindAccount(novaContaID)
```

## Schema SQLite

```sql
CREATE TABLE accounts (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    api_key         TEXT NOT NULL,     -- encrypted
    base_url        TEXT NOT NULL DEFAULT 'https://api.anthropic.com',
    status          TEXT NOT NULL DEFAULT 'active',  -- active|cooldown|disabled
    priority        INTEGER NOT NULL DEFAULT 0,
    cooldown_until  TEXT,              -- ISO 8601
    backoff_level   INTEGER NOT NULL DEFAULT 0,
    last_used_at    TEXT,
    last_error              TEXT,
    error_classification    TEXT,          -- ErrorClassification do domínio. Usado para reconstruir Cooldown.Reason via RehydrateAccount.
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE INDEX idx_accounts_status ON accounts(status);
CREATE INDEX idx_accounts_priority ON accounts(priority);

CREATE TABLE api_keys (
    id              TEXT PRIMARY KEY,
    key_hash        TEXT NOT NULL,      -- bcrypt hash
    key_prefix      TEXT NOT NULL,      -- primeiros 8 chars para lookup
    name            TEXT NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT 1,
    allowed_models  TEXT,               -- JSON array, NULL = todos
    created_at      TEXT NOT NULL,
    last_used_at    TEXT
);

CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_active ON api_keys(is_active);
```
