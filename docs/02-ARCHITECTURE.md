# Kestrel — Arquitetura

## Clean Architecture — Camadas

```
┌──────────────────────────────────────────────────────────┐
│                    cmd/kestrel/                        │
│              (composição raiz, wiring DI)                 │
└──────────────┬───────────────────────────┬───────────────┘
               │                           │
┌──────────────▼───────────┐ ┌─────────────▼───────────────┐
│   adapter/handler/       │ │   adapter/claude/            │
│   (HTTP entrada)         │ │   (cliente Claude)           │
│                          │ │                              │
│   adapter/middleware/     │ │   adapter/sqlite/            │
│   (auth, log, recovery)  │ │   (repositórios)             │
│                          │ │                              │
│   adapter/sse/           │ │   adapter/crypto/             │
│   (SSEWriter)            │ │   (encriptação AES-256-GCM)  │
└──────────────┬───────────┘ └─────────────┬───────────────┘
               │                           │
               │    ┌──────────────────┐   │
               └────▶   usecase/       ◀───┘
                    │  (orquestração)   │
                    │  (interfaces no   │
                    │   consumidor)     │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │    domain/        │
                    │  (entidades com   │
                    │   behavior, VOs,  │
                    │   erros)          │
                    └──────────────────┘
```

### Dependency Rule

- `domain/` → zero imports de framework (nem net/http, log/slog, database/sql). Tipos fundamentais da linguagem (time, errors, fmt, strings, math) são permitidos
- `usecase/` → importa apenas `domain/`; define suas próprias interfaces (ISP)
- `adapter/` → importa `domain/` e `usecase/`; implementa interfaces definidas nos use cases
- `cmd/` → importa tudo, faz wiring

### Interfaces Segregadas no Consumidor (Go Idiomático)

NÃO existe pacote `domain/port/` centralizado. Cada use case define apenas as interfaces
que precisa — Interface Segregation Principle aplicado ao Go:

```
Implementados:
usecase/ports.go            → define interfaces compartilhadas: AccountSelector, FallbackHandler, FallbackResult, Clock, ClassifiedError, RetryAttempt, ProxyChatResult, ProxyStreamResult
usecase/proxy_chat.go       → define ChatSender (1 método: SendChat)
usecase/proxy_stream.go     → define ChatStreamer (1 método: StreamChat)
usecase/admin_account.go    → define AccountStore (CRUD completo: FindByID, FindAll, Create, Save, Delete)
usecase/admin_apikey.go     → define APIKeyStore  (CRUD completo: FindByID, FindAll, Create, Delete)

usecase/authenticate.go     → define APIKeyFinder        (1 método: FindByPrefix)
adapter/middleware/logging.go → define RequestLogger (1 método: LogRequest) + RequestLogReader (1 método: FindAll) + RequestLogEntry + RequestLogFilters
adapter/middleware/auth.go  → define Authenticator       (1 método: Execute)

usecase/select_account.go   → define AccountFinder      (1 método: FindAvailable)
usecase/handle_fallback.go  → define AccountStatusWriter (2 métodos: UpdateStatus, RecordSuccess)
usecase/manage_session.go   → define SessionReader, SessionWriter
```

Isso garante que cada componente depende apenas do que consome, não de um contrato monolítico.

## Estrutura de Diretórios

```
kestrel/
├── cmd/
│   └── kestrel/
│       ├── main.go                     # Composição raiz
│       └── embed.go                    # embed.FS para frontend SPA (web/dist)
│
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── account.go              # Conta Claude (com behavior: ApplyCooldown, IsAvailable, etc.)
│   │   │   ├── apikey.go               # API Key do proxy (com behavior: Validate, IsModelAllowed)
│   │   │   └── session.go              # Sessão de roteamento (com behavior: BindAccount, IsExpired)
│   │   │
│   │   ├── vo/
│   │   │   ├── id.go                   # AccountID, APIKeyID, SessionID, RequestID
│   │   │   ├── credentials.go          # ProviderCredentials (APIKey, BaseURL)
│   │   │   ├── cooldown.go             # Cooldown com backoff exponencial
│   │   │   ├── model.go                # ModelName, parsing, validação
│   │   │   ├── error_classification.go # ErrorClassification (rate_limit, quota, auth, server, client)
│   │   │   └── chat.go                 # ChatRequest, ChatResponse, StreamEvent (tipos do domínio)
│   │   │
│   │   └── errs/
│   │       └── errors.go              # Erros tipados do domínio
│   │
│   ├── usecase/
│   │   ├── ports.go                   # Interfaces compartilhadas: AccountSelector, FallbackHandler, FallbackResult, Clock, ClassifiedError, RetryAttempt, ProxyChatResult, ProxyStreamResult
│   │   ├── proxy_chat.go              # ProxyChatUseCase (sync) + ChatSender interface + retry loop
│   │   ├── proxy_stream.go            # ProxyStreamUseCase (streaming) + ChatStreamer interface + retry loop
│   │   ├── select_account.go          # SelectAccountUseCase + AccountFinder interface
│   │   ├── handle_fallback.go         # HandleFallbackUseCase + AccountStatusWriter interface
│   │   ├── manage_session.go          # ManageSessionUseCase + SessionReader/SessionWriter interfaces
│   │   ├── admin_account.go           # AdminAccountUseCase (Create, Update, List, Delete, Reset) + AccountStore interface
│   │   ├── admin_apikey.go            # AdminAPIKeyUseCase (Create, List, Revoke) + APIKeyStore interface
│   │   └── authenticate.go            # AuthenticateUseCase + APIKeyFinder interface
│   │
│   ├── adapter/
│   │   ├── handler/
│   │   │   ├── chat.go                # POST /v1/chat/completions (traduz OpenAI ↔ domínio)
│   │   │   ├── translator.go          # OpenAI ↔ domínio (ChatRequest, ChatResponse, StreamEvent)
│   │   │   ├── models.go              # GET /v1/models
│   │   │   ├── health.go              # GET /health
│   │   │   ├── admin.go               # CRUD contas + keys (chama use cases administrativos)
│   │   │   └── oauth.go              # OAuth authorize + callback endpoints
│   │   │
│   │   ├── middleware/
│   │   │   ├── requestid.go           # Injeta X-Request-ID
│   │   │   ├── recovery.go            # Panic recovery
│   │   │   ├── auth.go                # Bearer token validation, APIKey no context
│   │   │   └── logging.go             # RequestLogger interface + slog request logging
│   │   │
│   │   ├── claude/
│   │   │   ├── client.go              # Implementa ChatSender, ChatStreamer (traduz domínio → Claude internamente)
│   │   │   ├── translator.go          # Funções internas: ChatRequest ↔ ClaudeRequest
│   │   │   ├── errors.go              # ClassifyHTTPError + ProviderError (HTTP → domínio)
│   │   │   └── sse.go                 # SSE stream reader (Claude API)
│   │   │
│   │   ├── sse/
│   │   │   └── writer.go             # SSEWriter: escreve StreamEvent → text/event-stream
│   │   │
│   │   ├── crypto/
│   │   │   └── aes.go                # AES-256-GCM encrypt/decrypt para API keys at rest
│   │   │
│   │   ├── oauth/
│   │   │   ├── claude.go              # OAuth Client: PKCE, AuthorizationURL, ExchangeCode, RefreshToken
│   │   │   └── claude_test.go         # Testes do client OAuth
│   │   │
│   │   ├── session/
│   │   │   └── memory.go             # MemorySessionStore (in-memory com RWMutex + cleanup goroutine)
│   │   │
│   │   └── sqlite/
│   │       ├── db.go                  # Conexão: 1 writer + N readers, WAL, busy_timeout
│   │       ├── migrations.go          # Auto-migration no startup
│   │       ├── account_repo.go        # Implementa AccountStore (FindByID, FindAll, Create, Save, Delete)
│   │       ├── apikey_repo.go         # Implementa APIKeyStore (FindByID, FindAll, Create, Delete)
│   │       └── request_log_repo.go   # Implementa RequestLogger (LogRequest) + RequestLogReader (FindAll)
│   │
│   └── infra/
│       ├── cfg/
│       │   └── config.go              # Env vars → struct
│       └── logger/
│           └── slog.go                # Setup do slog (JSON + pretty)
│
├── web/                               # Frontend SPA (React 19 + Vite + Tailwind + shadcn/ui)
│   ├── src/
│   │   ├── api/                       # API client com auth (X-Admin-Key)
│   │   ├── components/                # Layout, Sidebar, ErrorBoundary, UI components
│   │   ├── routes/                    # Dashboard, Accounts, ApiKeys, Logs pages
│   │   ├── hooks/                     # Custom React hooks
│   │   ├── types/                     # TypeScript types
│   │   ├── App.tsx                    # Router + QueryClient + auth setup
│   │   └── main.tsx                   # Entry point
│   └── dist/                          # Build output (embedded no binário Go)
│
├── migrations/
│   ├── embed.go                       # embed.FS para migrations SQL
│   ├── 001_accounts.sql
│   ├── 002_apikeys.sql
│   ├── 003_request_log.sql            # Tabela request_log + índices
│   └── 004_oauth_accounts.sql        # Campos OAuth na tabela accounts (auth_type, tokens, email)
│
├── Dockerfile                         # Multi-stage build (node → go → alpine)
├── go.mod
├── go.sum
└── Makefile
```

> **Nota:** `adapter/handler/` e `adapter/middleware/` são Interface Adapters (Clean Arch).
> `adapter/claude/`, `adapter/sqlite/`, `adapter/crypto/`, `adapter/session/` são Infrastructure.
> Ambos ficam em `adapter/` por pragmatismo Go (package naming curto),
> mas a dependency rule se mantém: Infrastructure implementa interfaces definidas nos use cases.

## Entidades do Domínio (com behavior)

Entidades NÃO são anêmicas — encapsulam regras de negócio e transições de estado.

**Diretriz de tempo:** Entidades e VOs NUNCA recebem `context.Context` nem chamam `time.Now()`. O tempo é sempre injetado como parâmetro. Use cases recebem Clock interface injetada — nunca chamam `time.Now()` diretamente. Adapters (handlers, middlewares) PODEM chamar `time.Now()` diretamente — a regra de Clock se aplica apenas a domain/ e usecase/.

### Account (Conta Claude)

```go
type Account struct {
    id            AccountID
    name          string
    apiKey        vo.SensitiveString  // encrypted at rest, fmt.Printf não vaza
    baseURL       string         // default: https://api.anthropic.com
    status        AccountStatus  // active | cooldown | disabled
    priority      int            // menor = preferido
    cooldown      *vo.Cooldown   // nil = sem cooldown ativo
    lastUsedAt    *time.Time
    lastError     *string
}

// NewAccount cria uma conta validada. Retorna erro se campos obrigatórios faltam.
func NewAccount(id AccountID, name string, apiKey vo.SensitiveString, baseURL string, priority int) (*Account, error)

// Getters (acesso somente leitura):
func (a *Account) ID() AccountID
func (a *Account) Name() string
func (a *Account) BaseURL() string
func (a *Account) Status() AccountStatus
func (a *Account) Priority() int
func (a *Account) CooldownUntil() *time.Time   // delega para cooldown VO, nil se sem cooldown
func (a *Account) BackoffLevel() int            // delega para cooldown VO, 0 se cooldown nil
func (a *Account) LastUsedAt() *time.Time
func (a *Account) LastError() *string
func (a *Account) ErrorClassification() *ErrorClassification // delega para cooldown.Reason, nil se cooldown nil

// Credentials retorna ProviderCredentials para uso pelo adapter.
// Encapsula o acesso à API key — nenhum getter expõe a key diretamente.
func (a *Account) Credentials() vo.ProviderCredentials

// Behavior (mutação controlada):

// ApplyCooldown aplica cooldown exponencial para erros transientes
// (rate_limit, quota_exhausted, server_error, overloaded).
// NÃO aceita ErrAuth, ErrClient nem ErrUnknown — para auth errors, usar Disable() diretamente.
// Retorna erro se classification é ErrAuth, ErrClient ou ErrUnknown.
func (a *Account) ApplyCooldown(classification ErrorClassification, now time.Time) error

// ClearError reseta backoff e status para active.
func (a *Account) ClearError()

// Disable marca a conta como desabilitada com motivo.
func (a *Account) Disable(reason string)

// IsAvailable retorna true se status != disabled E cooldown expirou.
// Verifica cooldown contra now.
func (a *Account) IsAvailable(now time.Time) bool

// RecordUsage atualiza LastUsedAt para now.
func (a *Account) RecordUsage(now time.Time)
```

> Campos privados (unexported) garantem que a mutação de estado acontece
> EXCLUSIVAMENTE via métodos de behavior. O adapter SQLite usa
> `RehydrateAccount(...)` para reconstruir instâncias a partir de dados persistidos.
> RehydrateAccount aplica as MESMAS validações de invariantes que NewAccount
> (campos não nulos, formato válido) — dados corrompidos no DB resultam em erro,
> não em objeto inválido. A diferença: RehydrateAccount não gera ID nem defaults.

### APIKey (Key do Proxy)

```go
type APIKey struct {
    id            APIKeyID
    keyHash       string
    keyPrefix     string
    name          string
    isActive      bool
    allowedModels []string
    lastUsedAt    *time.Time
}

// NewAPIKey cria uma API key validada. Retorna erro se campos obrigatórios faltam.
func NewAPIKey(id APIKeyID, name string, keyHash string, keyPrefix string) (*APIKey, error)

// Getters (acesso somente leitura):
func (k *APIKey) ID() APIKeyID
func (k *APIKey) KeyHash() string
func (k *APIKey) KeyPrefix() string
func (k *APIKey) Name() string
func (k *APIKey) IsActive() bool
func (k *APIKey) AllowedModels() []string
func (k *APIKey) LastUsedAt() *time.Time

// Behavior (mutação controlada):

// Validate verifica se o rawKey corresponde ao hash armazenado.
// Recebe uma função de comparação injetada (bcrypt.CompareHashAndPassword ou similar)
// para manter a entidade livre de imports de crypto.
func (k *APIKey) Validate(rawKey string, compareFn func(hash, raw string) bool) bool

// IsModelAllowed retorna true se o modelo está na lista de permitidos
// (ou se a lista está vazia = todos permitidos).
func (k *APIKey) IsModelAllowed(model string) bool

// RecordUsage atualiza LastUsedAt para now.
func (k *APIKey) RecordUsage(now time.Time)
```

### Session

```go
type Session struct {
    id           SessionID
    apiKeyID     APIKeyID
    accountID    *AccountID
    model        vo.ModelName
    requestCount int
    createdAt    time.Time
    lastActiveAt time.Time
    ttl          time.Duration
}

// NewSession cria uma sessão validada. Retorna erro se campos obrigatórios faltam.
func NewSession(id SessionID, apiKeyID APIKeyID, model vo.ModelName, ttl time.Duration, now time.Time) (*Session, error)

// Getters (acesso somente leitura):
func (s *Session) ID() SessionID
func (s *Session) APIKeyID() APIKeyID
func (s *Session) AccountID() *AccountID
func (s *Session) Model() vo.ModelName
func (s *Session) RequestCount() int
func (s *Session) CreatedAt() time.Time
func (s *Session) LastActiveAt() time.Time
func (s *Session) TTL() time.Duration

// Behavior (mutação controlada):

// BindAccount associa a sessão a uma conta (sticky routing).
func (s *Session) BindAccount(accountID AccountID)

// UnbindAccount remove a associação (conta entrou em cooldown).
func (s *Session) UnbindAccount()

// RecordRequest incrementa RequestCount e atualiza LastActiveAt.
func (s *Session) RecordRequest(now time.Time)

// IsExpired retorna true se LastActiveAt + TTL < now.
func (s *Session) IsExpired(now time.Time) bool
```

## Value Objects

### IDs

```
Formato: IDs são prefixados para legibilidade:
  AccountID: "acc_" + nanoid(21)
  APIKeyID:  "key_" + nanoid(21)
  SessionID: "ses_" + nanoid(21)
  RequestID: "req_" + nanoid(21)
```

### ProviderCredentials

```go
// ProviderCredentials encapsula as credenciais necessárias para comunicação com o provider.
// Criado internamente pela entidade Account via Credentials() — a API key nunca é exposta
// como getter público. O use case obtém ProviderCredentials chamando account.Credentials()
// e passa ao adapter sem manipular a key diretamente.
//
// Segurança: APIKey usa SensitiveString — fmt.Printf("%v", creds) imprime "[REDACTED]".
// O valor real é acessível via .Value() apenas no adapter claude/client.go.
type ProviderCredentials struct {
    APIKey  SensitiveString
    BaseURL string
}

// SensitiveString encapsula valores sensíveis. String() retorna "[REDACTED]".
type SensitiveString struct {
    value string
}

func NewSensitiveString(v string) SensitiveString { return SensitiveString{value: v} }
func (s SensitiveString) Value() string           { return s.value }
func (s SensitiveString) String() string          { return "[REDACTED]" }
func (s SensitiveString) GoString() string        { return "[REDACTED]" }
func (s SensitiveString) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
```

> SensitiveString tem campo unexported (`value`). Para persistência SQLite, o adapter
> usa `apiKey.Value()` para extrair o plaintext antes de encriptar, e
> `NewSensitiveString(decrypted)` ao reconstruir via RehydrateAccount.
> encoding/json e database/sql não acessam o campo diretamente — isso é intencional.

### ErrorClassification

```go
// ErrorClassification categoriza erros do provider para decisão de fallback.
// É um enum puro do domínio — NÃO conhece HTTP status codes.
// A classificação a partir de HTTP é feita pelo adapter (claude/client.go).
type ErrorClassification string

const (
    ErrRateLimit      ErrorClassification = "rate_limit"
    ErrQuotaExhausted ErrorClassification = "quota_exhausted"
    ErrAuth           ErrorClassification = "authentication_error"
    ErrServer         ErrorClassification = "server_error"
    ErrOverloaded     ErrorClassification = "overloaded"
    ErrClient         ErrorClassification = "client_error"  // não faz retry
    ErrUnknown        ErrorClassification = "unknown"
)

// ShouldFallback retorna true se a classificação justifica tentar outra conta.
func (c ErrorClassification) ShouldFallback() bool

// DefaultCooldownDuration retorna a duração base do cooldown para esta classificação.
func (c ErrorClassification) DefaultCooldownDuration() time.Duration
```

A factory que classifica a partir de HTTP status/body fica no adapter:

```go
// adapter/claude/errors.go
//
// classifyHTTPError converte status HTTP + body em ErrorClassification do domínio.
// Esta lógica é específica do provider Claude e pertence ao adapter.
func classifyHTTPError(status int, body string) vo.ErrorClassification

// ProviderError é o tipo interno do adapter que implementa a interface
// ClassifiedError do use case. O use case extrai a classificação via
// errors.As(err, &classErr) sem importar este tipo.
type ProviderError struct { ... }
func (e *ProviderError) Error() string
func (e *ProviderError) Classification() vo.ErrorClassification
```

### ChatRequest / ChatResponse / StreamEvent (Tipos do Domínio)

```go
// ChatRequest é a representação do domínio — NÃO é OpenAI nem Claude.
// O handler traduz OpenAI → ChatRequest. O adapter claude traduz ChatRequest → Claude.
type ChatRequest struct {
    Model       ModelName
    Messages    []Message
    MaxTokens   int
    Temperature *float64
    // ... demais campos neutros
}
// Stream NÃO faz parte do ChatRequest — o domínio não sabe se é streaming ou não.
// Isso é decisão de delivery: o handler chama Execute() ou ExecuteStream()
// baseado no campo stream do request OpenAI.

type ChatResponse struct {
    ID        string
    Content   string
    Model     string
    Usage     Usage
    StopReason string
}

type StreamEvent struct {
    Type    StreamEventType
    Content string
    Usage   *Usage  // presente no evento final
    Error   *string
}

type StreamEventType string
const (
    EventStart   StreamEventType = "start"
    EventDelta   StreamEventType = "delta"
    EventStop    StreamEventType = "stop"
    EventError   StreamEventType = "error"
)
// Tipos de evento são abstrações do domínio, não espelhos de nenhum provider.
// O adapter claude/sse.go traduz eventos Claude (message_start, content_block_delta,
// message_stop) para esses tipos neutros.

type Role string
const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

type Message struct {
    Role    Role
    Content []ContentBlock
}

type ContentBlock struct {
    Type       string  // "text", "image", "tool_use", "tool_result"
    Text       string  // para type "text"
    // demais campos conforme necessário
}

type Usage struct {
    InputTokens  int
    OutputTokens int
}
```

### Cooldown

```go
// Cooldown encapsula o estado de indisponibilidade temporária de uma conta.
// Value object imutável — value receiver em todos os métodos.
// Account armazena *Cooldown (pointer para nil-ability).
type Cooldown struct {
    until        time.Time
    backoffLevel int
    reason       ErrorClassification
}

func NewCooldown(until time.Time, backoffLevel int, reason ErrorClassification) Cooldown

func (c Cooldown) Until() time.Time
func (c Cooldown) BackoffLevel() int
func (c Cooldown) Reason() ErrorClassification
func (c Cooldown) IsExpired(now time.Time) bool
```

#### Interação Cooldown + DefaultCooldownDuration

`Account.ApplyCooldown()` combina os dois conceitos:

- **rate_limit** (`DefaultCooldownDuration = 0`): usa backoff exponencial puro — `min(2^level, 120)` segundos
- **quota_exhausted** (`DefaultCooldownDuration = 5min`): usa duração fixa de 5min, ignora backoff
- **overloaded** (`DefaultCooldownDuration = 30s`): usa duração fixa de 30s, ignora backoff
- **server_error** (`DefaultCooldownDuration = 60s`): usa duração fixa de 60s, ignora backoff

Regra: se `DefaultCooldownDuration > 0`, usa a duração fixa. Se `= 0`, usa backoff exponencial.

### ModelName

```go
// ModelName encapsula um nome de modelo com parsing e validação.
type ModelName struct {
    Raw      string   // o que o cliente enviou
    Resolved string   // nome canônico para Claude
}

// Parse cria um ModelName validado a partir de uma string.
func ParseModelName(raw string) (ModelName, error)

// IsValid retorna true se o modelo é suportado.
func (m ModelName) IsValid() bool
```

> **Convenção de campos em VOs:** VOs com invariantes complexas (Cooldown, SensitiveString)
> usam campos unexported + constructor + getters. VOs simples sem invariantes (ModelName,
> ProviderCredentials, ChatRequest, ChatResponse) usam campos exported — são DTOs de
> transporte entre camadas sem regras de validação interna.

## Interfaces Segregadas (detalhe)

### No use case — select_account.go

```go
// AccountFinder encontra contas disponíveis para roteamento.
type AccountFinder interface {
    FindAvailable(ctx context.Context, excludeID *AccountID) ([]*entity.Account, error)
}
```

### No use case — handle_fallback.go

```go
// AccountStatusWriter persiste mudanças de status de conta.
type AccountStatusWriter interface {
    UpdateStatus(ctx context.Context, account *entity.Account) error
    RecordSuccess(ctx context.Context, accountID vo.AccountID, now time.Time) error
}
// Nota: RecordSuccess faz UPDATE direto (ClearError + RecordUsage atomicamente)
// sem precisar da entidade — usado pela goroutine pós-streaming para evitar race condition.
```

### No use case — proxy_chat.go (sync) e proxy_stream.go (streaming)

ProxyChatUseCase e ProxyStreamUseCase são use cases separados (SRP).
Ambos dependem das mesmas interfaces de seleção e fallback via composição.

```go
// --- Interfaces compartilhadas (definidas em ports.go, usadas por ambos) ---

// AccountSelector abstrai a seleção de conta (implementado por SelectAccountUseCase).
type AccountSelector interface {
    Execute(ctx context.Context, preferredID *vo.AccountID, excludeID *vo.AccountID, now time.Time) (*entity.Account, error)
}

// FallbackHandler abstrai o tratamento de fallback (implementado por HandleFallbackUseCase).
type FallbackHandler interface {
    Execute(ctx context.Context, account *entity.Account, classification vo.ErrorClassification) (FallbackResult, error)
}

// FallbackResult encapsula o resultado de uma tentativa de fallback.
type FallbackResult struct {
    ShouldFallback bool                     // true = tentar outra conta
    Classification vo.ErrorClassification   // tipo do erro que causou o fallback
}

// Clock abstrai o relógio do sistema para testabilidade.
type Clock interface {
    Now() time.Time
}

// ClassifiedError é implementado por erros que carregam classificação do domínio.
// O use case extrai a classificação via errors.As sem importar tipos do adapter.
type ClassifiedError interface {
    error
    Classification() vo.ErrorClassification
}

// RetryAttempt registra uma tentativa de fallback para observabilidade.
type RetryAttempt struct {
    AccountID      vo.AccountID
    Classification vo.ErrorClassification
    RetryIndex     int
}

// --- proxy_chat.go (sync) ---

// ChatSender envia requests síncronos ao provider.
type ChatSender interface {
    SendChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (*vo.ChatResponse, error)
}

// ProxyChatResult encapsula resposta + metadata de retry para observabilidade.
// O middleware logging extrai Retries do resultado para logar fallbacks.
type ProxyChatResult struct {
    Response *vo.ChatResponse
    Retries  []RetryAttempt
}

// --- proxy_stream.go (streaming) ---

// ChatStreamer envia requests com streaming ao provider.
type ChatStreamer interface {
    StreamChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (<-chan vo.StreamEvent, error)
}

// ProxyStreamResult encapsula channel + metadata de retry.
type ProxyStreamResult struct {
    Events  <-chan vo.StreamEvent
    Retries []RetryAttempt
}
```

A verificação de modelo permitido (`apiKey.IsModelAllowed`) acontece no `ChatHandler`
(adapter), APÓS parsear o body (evita double-parse). O handler tem acesso à `APIKey`
(injetada pelo middleware auth) e retorna 403 se o modelo não é permitido.

O `ProviderError` (tipo interno do `adapter/claude/errors.go`) implementa a interface
`ClassifiedError` definida no use case. O use case extrai a classificação via
`errors.As(err, &classErr)` sem importar tipos do adapter.

Os sub-use-cases (SelectAccountUseCase, HandleFallbackUseCase) implementam as interfaces
`AccountSelector` e `FallbackHandler` definidas acima. A composição concreta acontece
no `cmd/kestrel/main.go` (composition root).

### No use case — authenticate.go

```go
// APIKeyFinder busca API key por prefixo para autenticação.
type APIKeyFinder interface {
    FindByPrefix(ctx context.Context, prefix string) (*entity.APIKey, error)
}
```

### No adapter/middleware — logging.go

```go
// RequestLogger registra o resultado de cada request processado.
// Definido no adapter (middleware) porque é o consumidor.
// Implementado pelo adapter SQLite (request_log_repo.go).
// NÃO é um use case — é cross-cutting concern de infraestrutura.
type RequestLogger interface {
    LogRequest(ctx context.Context, entry RequestLogEntry) error
}

type RequestLogEntry struct {
    RequestID    RequestID
    APIKeyID     APIKeyID
    APIKeyName   string
    AccountID    *AccountID
    AccountName  string
    Model        string
    Status       int
    InputTokens  int
    OutputTokens int
    LatencyMs    int64
    Retries      int
    Error        *string
    Stream       bool
}
```

### No use case — admin_account.go

```go
// AccountStore fornece CRUD completo para administração de contas.
// Usado por CreateAccountUseCase, UpdateAccountUseCase, DeleteAccountUseCase.
type AccountStore interface {
    FindByID(ctx context.Context, id AccountID) (*entity.Account, error)
    FindAll(ctx context.Context) ([]*entity.Account, error)
    Create(ctx context.Context, account *entity.Account) error
    Save(ctx context.Context, account *entity.Account) error
    Delete(ctx context.Context, id AccountID) error
}
```

### No use case — admin_apikey.go

```go
// APIKeyStore fornece CRUD completo para administração de API keys.
// Usado por CreateAPIKeyUseCase, RevokeAPIKeyUseCase.
type APIKeyStore interface {
    FindByID(ctx context.Context, id APIKeyID) (*entity.APIKey, error)
    FindAll(ctx context.Context) ([]*entity.APIKey, error)
    Create(ctx context.Context, key *entity.APIKey) error
    Delete(ctx context.Context, id APIKeyID) error
}
```

O handler admin (`adapter/handler/admin.go`) NÃO acessa repositórios diretamente — chama os use cases administrativos, que por sua vez definem e consomem as interfaces de repositório acima.

### Input DTOs para Use Cases Admin

Os use cases administrativos definem seus próprios tipos de input.
O handler traduz HTTP request → DTO. O use case cria a entidade via `NewAccount()`/`NewAPIKey()`.

```go
// usecase/admin_account.go

type CreateAccountInput struct {
    Name     string
    APIKey   string
    BaseURL  string
    Priority int
}

type UpdateAccountInput struct {
    Name     *string
    APIKey   *string
    BaseURL  *string
    Priority *int
}
```

```go
// usecase/admin_apikey.go

type CreateAPIKeyInput struct {
    Name          string
    AllowedModels []string
}
```

### No use case — manage_session.go

```go
// SessionReader obtém ou cria sessões para roteamento.
type SessionReader interface {
    GetOrCreate(ctx context.Context, apiKeyID vo.APIKeyID, model vo.ModelName) (*entity.Session, error)
}

// SessionWriter persiste mudanças em sessões.
type SessionWriter interface {
    Save(ctx context.Context, session *entity.Session) error
}
```

## Concorrência

### SessionStore (in-memory)

A implementação in-memory (`adapter/session/memory.go`) usa `sync.RWMutex` para segurança de concorrência:

- Leitura: `RLock()` / `RUnlock()`
- Escrita: `Lock()` / `Unlock()`
- Cleanup de sessões expiradas: goroutine com ticker

### SQLite

```
Modo:          WAL (Write-Ahead Logging)
busy_timeout:  5000ms
foreign_keys:  ON
journal_mode:  WAL
```

Conexões separadas por papel:
- **1 writer connection** — todas as escritas passam por ela (serialização natural)
- **N reader connections** — leituras concorrentes sem bloqueio

### Cancelamento de Contexto em Streaming

Quando o cliente desconecta (ctx cancelado), a cadeia inteira deve parar:

1. Handler detecta `ctx.Done()` → para de consumir o channel
2. Use case detecta channel não consumido / `ctx.Done()` → fecha output channel
3. Adapter claude detecta `ctx.Done()` → fecha conexão HTTP com Anthropic
4. Goroutine wrapper do use case retorna → sem goroutine leak

O adapter `claude/client.go` DEVE monitorar `ctx.Done()` via select no loop de leitura SSE:

```go
select {
case <-ctx.Done():
    close(events)
    return
case events <- event:
}
```

O SSEWriter no handler DEVE verificar ctx antes de cada flush:

```go
for event := range events {
    if ctx.Err() != nil {
        return
    }
    // write + flush
}
```

### Seleção de Conta (Transação Atômica)

A seleção de conta usa `BEGIN IMMEDIATE` para evitar race conditions
entre múltiplas requests simultâneas:

```sql
BEGIN IMMEDIATE;
  SELECT * FROM accounts
    WHERE status = 'active'
    AND (cooldown_until IS NULL OR cooldown_until < datetime('now'))
    AND (id != ? OR ? IS NULL)   -- excludeID
    ORDER BY priority ASC, last_used_at ASC;

  UPDATE accounts SET last_used_at = datetime('now') WHERE id = ?;
COMMIT;
```

`BEGIN IMMEDIATE` adquire o write lock imediatamente, garantindo que
duas requests não selecionem a mesma conta simultaneamente.

O SQL retorna contas disponíveis ordenadas por prioridade e uso. A lógica de
preferência (sticky routing via `preferredID`) fica no use case `SelectAccount`,
que verifica se a conta preferida está na lista retornada pelo repositório.

O SQL filtra por `status = 'active'` como otimização. A source of truth é `Account.IsAvailable(now)`, que verifica status E cooldown. Em testes, o behavior da entidade é o que é testado.

## Encriptação

### API Keys at Rest

As API keys das contas Claude são encriptadas antes de persistir no SQLite:

```
Algoritmo:     AES-256-GCM
Derivação:     HKDF (SHA-256) a partir de ENCRYPTION_KEY (env var)
Nonce:         12 bytes aleatórios (prepended ao ciphertext)
Boundary:      adapter/sqlite (encrypt no Save, decrypt no Find)
```

O domínio trabalha com a key em plaintext — a encriptação/decriptação
acontece exclusivamente no adapter SQLite, mantendo o domínio limpo.

O `adapter/sqlite` define a interface `Encryptor` no consumidor (ISP):

```go
// adapter/sqlite/account_repo.go
type Encryptor interface {
    Encrypt(plaintext string) (string, error)
    Decrypt(ciphertext string) (string, error)
}
```

`adapter/crypto/aes.go` implementa `Encryptor`. Injetado no construtor do repo.

```go
// adapter/crypto/aes.go
type AESEncryptor struct {
    key []byte // derivada via HKDF de ENCRYPTION_KEY
}

func (e *AESEncryptor) Encrypt(plaintext string) (string, error)
func (e *AESEncryptor) Decrypt(ciphertext string) (string, error)
```

## Formato de Erro (Error Response)

Todas as respostas de erro seguem o formato OpenAI para compatibilidade:

```json
{
    "error": {
        "message": "All accounts exhausted, try again later",
        "type": "server_error",
        "code": "service_unavailable"
    }
}
```

Mapeamento de erros internos → response:

| Erro interno                | HTTP Status | type                   | code                |
|-----------------------------|-------------|------------------------|---------------------|
| API key inválida            | 401         | authentication_error   | invalid_api_key     |
| Modelo não permitido        | 403         | permission_error       | model_not_allowed   |
| Request inválido            | 400         | invalid_request_error  | bad_request         |
| Todas as contas esgotadas   | 503         | server_error           | service_unavailable |
| Erro interno                | 500         | server_error           | internal_error      |
| Body acima do limite        | 413         | invalid_request_error  | request_too_large   |

## Limites de Request

O handler aplica `http.MaxBytesReader` antes de ler o body:

```go
r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB
```

Limite de 10MB cobre contextos grandes do Claude Code (muitas messages + tools).
Requests acima do limite retornam 413 Request Entity Too Large.
