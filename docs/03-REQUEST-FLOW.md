# Kestrel — Fluxo de Request

## Visão Geral

```
Claude Code                Kestrel (Go)                    Anthropic API
─────────                  ───────────────                    ─────────────
    │                            │                                 │
    │  POST /v1/chat/completions │                                 │
    │  (formato OpenAI)          │                                 │
    │ ──────────────────────────▶│                                 │
    │                            │                                 │
    │                     ┌──────┴──────┐                          │
    │                     │ Middleware   │                          │
    │                     │ Chain       │                          │
    │                     │             │                          │
    │                     │ 1. Recovery │                          │
    │                     │ 2. RequestID│                          │
    │                     │ 3. Logging  │                          │
    │                     │ 4. Auth     │                          │
    │                     └──────┬──────┘                          │
    │                            │                                 │
    │                     ┌──────▼──────┐                          │
    │                     │ ChatHandler │                          │
    │                     │  (adapter)  │                          │
    │                     │             │                          │
    │                     │ 1. Buffer   │                          │
    │                     │    body     │                          │
    │                     │ 2. Parse    │                          │
    │                     │    OpenAI   │                          │
    │                     │ 3. Traduz   │                          │
    │                     │  → domínio  │                          │
    │                     └──────┬──────┘                          │
    │                            │ ChatRequest (domínio)           │
    │                     ┌──────▼──────┐                          │
    │                     │ ProxyChat   │                          │
    │                     │ UseCase     │                          │
    │                     │             │                          │
    │                     │ ┌─────────┐ │                          │
    │                     │ │Select   │ │                          │
    │                     │ │Account  │ │                          │
    │                     │ └────┬────┘ │                          │
    │                     │      │      │                          │
    │                     │ ┌────▼────┐ │  POST /v1/messages       │
    │                     │ │Provider │ │  (formato Claude)        │
    │                     │ │Gateway  │─┼─────────────────────────▶│
    │                     │ │(adapter)│ │  (traduz domínio→Claude) │
    │                     │ └─────────┘ │                          │
    │                     │             │  resposta Claude          │
    │                     │             │◀─────────────────────────│
    │                     │             │  (adapter traduz          │
    │                     │             │   Claude→domínio)         │
    │                     │      │      │                          │
    │                     │  OK? │      │                          │
    │                     │  ┌───┴───┐  │                          │
    │                     │  │  SIM  │──┼─▶ clearError, retorna    │
    │                     │  │  NÃO  │  │   ChatResponse (domínio) │
    │                     │  └───┬───┘  │                          │
    │                     │      │      │                          │
    │                     │ ┌────▼────┐ │                          │
    │                     │ │Fallback │ │                          │
    │                     │ │Handler  │ │                          │
    │                     │ │         │ │                          │
    │                     │ │shouldFb?│ │                          │
    │                     │ │ SIM: ───┼─┼─▶ loop com próxima conta│
    │                     │ │ NÃO: ───┼─┼─▶ retorna erro          │
    │                     │ └─────────┘ │                          │
    │                     └─────────────┘                          │
    │                            │                                 │
    │                     ┌──────▼──────┐                          │
    │                     │ ChatHandler │                          │
    │                     │  traduz     │                          │
    │                     │ domínio →   │                          │
    │                     │  OpenAI     │                          │
    │                     └──────┬──────┘                          │
    │                            │                                 │
    │  resposta (OpenAI fmt)     │                                 │
    │ ◀──────────────────────────│                                 │
    │                            │                                 │
```

### Fronteira de Tradução

A tradução de formatos acontece EXCLUSIVAMENTE nos adapters — o use case trabalha
apenas com tipos do domínio (`ChatRequest`, `ChatResponse`, `StreamEvent`):

```
OpenAI fmt ──▶ [handler traduz] ──▶ ChatRequest (domínio) ──▶ [use case]
                                                                   │
                                                          ChatSender/ChatStreamer
                                                          (adapter claude)
                                                                   │
                                                     [traduz domínio → Claude]
                                                                   │
                                                          Claude API call
                                                                   │
                                                     [traduz Claude → domínio]
                                                                   │
                                                   ChatResponse (domínio) ◀──┘
                                                                   │
[handler traduz] ◀── ChatResponse (domínio) ◀────── [use case retorna]
       │
OpenAI fmt ◀──┘
```

## Fluxo Detalhado

### 1. Middleware Chain

```
Recovery → defer/recover panic → 500 + log.Error (1. MAIS EXTERNO)
    ↓
RequestID → gera nanoid com prefixo req_ (2.)
    ↓
Logging → wraps handler — mede latência (3.)
    ↓
Auth → valida API key, injeta entidade no context (4.)
    ↓
Handler (5. MAIS INTERNO)
```

### 2. ChatHandler (adapter/handler/chat.go)

O handler é **thin** — apenas:
1. Bufferizar e parsear o body
2. Traduzir OpenAI → domínio
3. Delegar ao use case
4. Traduzir resposta domínio → OpenAI

**NÃO faz:** request logging (middleware logging).

```go
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Buffer + parse (body já limitado a 10MB pelo middleware)
    body, err := io.ReadAll(r.Body)
    if err != nil {
        writeError(w, 400, "invalid_request_error", "bad_request", "failed to read body")
        return
    }

    var openaiReq OpenAIChatRequest
    if err := json.Unmarshal(body, &openaiReq); err != nil {
        writeError(w, 400, "invalid_request_error", "bad_request", "invalid JSON")
        return
    }

    if openaiReq.Model == "" || len(openaiReq.Messages) == 0 {
        writeError(w, 400, "invalid_request_error", "bad_request", "model and messages required")
        return
    }

    // 2. Traduzir OpenAI → domínio
    chatReq, err := h.translator.OpenAIToDomain(openaiReq)
    if err != nil {
        writeError(w, 400, "invalid_request_error", "bad_request", err.Error())
        return
    }

    // 3. Verificar permissão de modelo (após parse, sem double-read do body)
    apiKey := r.Context().Value(apiKeyContextKey).(*entity.APIKey)
    if !apiKey.IsModelAllowed(chatReq.Model.Raw) {
        writeError(w, 403, "permission_error", "model_not_allowed", "model not allowed")
        return
    }

    // 4. Delegar ao use case
    // (Request log será registrado pelo middleware logging ao redor)
    apiKeyID := apiKey.ID()

    if openaiReq.Stream {
        result, err := h.proxyStream.Execute(r.Context(), apiKeyID, chatReq)
        if err != nil {
            writeError(w, errToStatus(err), errToType(err), errToCode(err), err.Error())
            return
        }
        h.onRequestComplete(result.Retries)
        h.sseWriter.Write(r.Context(), w, result.Events, h.translator.DomainEventToOpenAI)
    } else {
        result, err := h.proxyChat.Execute(r.Context(), apiKeyID, chatReq)
        if err != nil {
            writeError(w, errToStatus(err), errToType(err), errToCode(err), err.Error())
            return
        }
        h.onRequestComplete(result.Retries)
        openaiResp := h.translator.DomainToOpenAI(result.Response)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(openaiResp)
    }
}
```

### 3. ProxyChatUseCase (usecase/proxy_chat.go)

O use case trabalha EXCLUSIVAMENTE com tipos do domínio. Não conhece OpenAI,
Claude, HTTP, `http.ResponseWriter`, translators, nem SSE.

```
Execute(ctx, apiKeyID vo.APIKeyID, chatReq ChatRequest) (ProxyChatResult, error):
  1. model = chatReq.Model
  2. session = sessionReader.GetOrCreate(ctx, apiKeyID, model)
  3. var retries []RetryAttempt

  RETRY LOOP (safety cap de 10 iterações):
    4. account, err = accountSelector.Execute(ctx, session.AccountID, excludeID, clock.Now())
       se err → return ProxyChatResult{Retries: retries}, errs.ErrAllAccountsExhausted

    5. creds = account.Credentials()
       chatResp, err = chatSender.SendChat(ctx, creds, chatReq)

    6. SE sucesso:
         - account.ClearError()
         - account.RecordUsage(clock.Now())
         - accountWriter.UpdateStatus(ctx, account)
         - session.BindAccount(account.ID())
         - session.RecordRequest(clock.Now())
         - sessionWriter.Save(ctx, session)
         - RETURN ProxyChatResult{Response: chatResp, Retries: retries}, nil

    7. SE erro:
         - var classErr ClassifiedError
         - if !errors.As(err, &classErr) → RETURN ProxyChatResult{Retries: retries}, err
         - result = fallbackHandler.Execute(ctx, account, classErr.Classification())
         - retries = append(retries, RetryAttempt{account.ID(), classErr.Classification(), len(retries)})
         - SE result.ShouldFallback:
             excludeID = account.ID()
             CONTINUE loop
         - SENÃO:
             RETURN ProxyChatResult{Retries: retries}, err

Note: accountSelector e fallbackHandler são interfaces (AccountSelector, FallbackHandler)
definidas em ports.go. Os sub-use-cases (SelectAccountUseCase, HandleFallbackUseCase)
implementam essas interfaces. A composição concreta acontece no composition root.

Note: O use case retorna ProxyChatResult que inclui Retries (metadata de fallback).
O handler chama h.onRequestComplete(result.Retries) após obter o resultado do use case.
O middleware logging configura esse callback durante a composição (cmd/main.go).
Alternativa: o handler armazena retries no context via pointer mutável para o middleware consumir.
O use case NÃO loga diretamente — retorna dados, o adapter loga.

Note: O request log é registrado pelo middleware logging que envolve o handler.
O middleware mede latência, captura status e chama requestLogger.LogRequest().
Nem o handler nem o use case conhecem o request log.

Note: IsModelAllowed é verificado pelo ChatHandler APÓS parsear o body (evita
double-parse). O middleware auth valida a API key e injeta a entidade no context.

ExecuteStream (ProxyStreamUseCase — use case separado, SRP):

Execute(ctx, apiKeyID vo.APIKeyID, chatReq ChatRequest) (ProxyStreamResult, error):
  1. model = chatReq.Model
  2. session = sessionReader.GetOrCreate(ctx, apiKeyID, model)
  3. var retries []RetryAttempt

  RETRY LOOP (safety cap de 10 iterações):
    4. account, err = accountSelector.Execute(ctx, session.AccountID, excludeID, clock.Now())
       se err → return ProxyStreamResult{Retries: retries}, errs.ErrAllAccountsExhausted

    5. creds = account.Credentials()
       events, err = chatStreamer.StreamChat(ctx, creds, chatReq)

    6. SE err != nil (erro ANTES de qualquer evento — retry possível):
         - var classErr ClassifiedError
         - if !errors.As(err, &classErr) → return ProxyStreamResult{Retries: retries}, err
         - result = fallbackHandler.Execute(ctx, account, classErr.Classification())
         - retries = append(retries, RetryAttempt{account.ID(), classErr.Classification(), len(retries)})
         - SE result.ShouldFallback:
             excludeID = account.ID()
             CONTINUE loop
         - SENÃO:
             return ProxyStreamResult{Retries: retries}, err

    7. SE err == nil (channel aberto — streaming iniciou):
         - Cria goroutine wrapper que:
             - Consome events do provider channel
             - Repassa para output channel
             - Se EventError aparece: repassa para output (handler/SSEWriter lida)
               → Retry IMPOSSÍVEL neste ponto (headers 200 já enviados)
             - No final (channel fecha — operações best-effort com context.Background()):
               // NÃO muta a entidade original (evita race condition).
               // Faz UPDATE direto no repositório com valores calculados.
               accountWriter.RecordSuccess(context.Background(), account.ID(), clock.Now())
               // Session é safe para mutar na goroutine: a instância foi criada/obtida
               // dentro deste use case e nenhuma outra goroutine a acessa.
               // Session é per-request, não compartilhada.
               session.BindAccount(account.ID()), session.RecordRequest(clock.Now())
               if err := sessionWriter.Save(context.Background(), session); err != nil { log error }
               // Erros são logados, não propagados — cliente já recebeu resposta.
         - RETURN ProxyStreamResult{Events: outputChannel, Retries: retries}, nil
```

### 4. Account Selection (usecase/select_account.go)

```
Execute(ctx, preferredID, excludeID, now time.Time):
  1. accounts = accountFinder.FindAvailable(ctx, excludeID)
     // SQL já filtra por status='active' e cooldown expirado como otimização.
     // Defesa em profundidade: use case re-filtra com Account.IsAvailable(now)
     // para garantir single source of truth na entidade.
     accounts = filter(accounts, func(a) { return a.IsAvailable(now) })

  2. SE preferredID != nil E preferred está em accounts:
       → RETURN preferred (sticky routing)

  3. Ordenar por:
     a. Priority ASC (menor = preferido)
     b. LastUsedAt ASC (menos recente primeiro)

  4. RETURN accounts[0]
```

**Nota:** A seleção usa transação atômica (`BEGIN IMMEDIATE`) no adapter SQLite
para evitar race conditions — ver seção Concorrência em `02-ARCHITECTURE.md`.

### 5. Fallback Handler (usecase/handle_fallback.go)

```
Execute(ctx, account, classification ErrorClassification):
  1. SE classification == ErrAuth:
       account.Disable("auth error")
       accountWriter.UpdateStatus(ctx, account)
     SE classification == ErrClient || classification == ErrUnknown:
       // Nenhuma mutação — retorna direto
     SENÃO:
       account.ApplyCooldown(classification, clock.Now())
       accountWriter.UpdateStatus(ctx, account)

  2. RETURN FallbackResult{
       ShouldFallback: classification.ShouldFallback(),
       Classification: classification,
     }
```

Classificação de erros — o VO do domínio define behaviors, o adapter `claude/errors.go`
faz a conversão HTTP → classificação (ver `05-ACCOUNT-ROTATION.md`):

| Classificação     | Fallback? | Ação na conta    |
|-------------------|-----------|------------------|
| rate_limit        | sim       | cooldown exp.    |
| quota_exhausted   | sim       | cooldown exp.    |
| authentication_error | sim    | DISABLED         |
| overloaded        | sim       | cooldown 30s     |
| server_error      | sim       | cooldown 60s     |
| client_error      | não       | nenhuma          |
| unknown           | não       | nenhuma          |

## SSE Streaming Flow

```
Claude API                  Kestrel                    Claude Code
──────────                  ──────────                    ───────────
    │                            │                             │
    │ event: message_start       │                             │
    │ data: {"type":...}         │                             │
    │ ─────────────────────────▶ │                             │
    │                            │ adapter/claude traduz:       │
    │                            │ Claude event → StreamEvent   │
    │                            │ (tipo do domínio)            │
    │                            │                             │
    │                            │ use case repassa StreamEvent │
    │                            │ pelo channel de saída        │
    │                            │                             │
    │                            │ handler consome channel:     │
    │                            │ SSEWriter traduz:            │
    │                            │ StreamEvent → OpenAI chunk   │
    │                            │                             │
    │                            │ data: {"choices":[...]}     │
    │                            │ ───────────────────────────▶│
    │                            │                             │
    │ event: content_block_delta │                             │
    │ data: {"delta":{"text":"H"│                             │
    │ ─────────────────────────▶ │                             │
    │                            │ StreamEvent{Content:"H"}    │
    │                            │                             │
    │                            │ data: {"choices":[{         │
    │                            │   "delta":{"content":"H"}}]}│
    │                            │ ───────────────────────────▶│
    │                            │                             │
    │ ...chunks...               │ ...chunks...                │
    │                            │                             │
    │ event: message_stop        │                             │
    │ ─────────────────────────▶ │                             │
    │                            │ StreamEvent{Type:"stop"}    │
    │                            │                             │
    │                            │ data: [DONE]                │
    │                            │ ───────────────────────────▶│
    │                            │                             │
    │                            │ Log: usage, latency,        │
    │                            │      tokens, account        │
```

**Cancelamento:** quando o cliente desconecta, `ctx` é cancelado → adapter fecha conexão com Claude → use case fecha channel → SSEWriter retorna. Sem goroutine leak.

### SSEWriter (adapter/sse/writer.go)

O SSEWriter é um componente separado no adapter que encapsula a escrita SSE.
O handler delega a escrita para ele, mantendo o handler enxuto.

```go
// SSEWriter escreve StreamEvents no formato text/event-stream.
type SSEWriter struct{}

// Write consome o channel de StreamEvent e escreve no ResponseWriter.
// translateFn converte cada StreamEvent do domínio para o formato de saída (OpenAI).
func (s *SSEWriter) Write(
    ctx context.Context,
    w http.ResponseWriter,
    events <-chan vo.StreamEvent,
    translateFn func(vo.StreamEvent) []byte,
) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        writeError(w, 500, "server_error", "internal_error", "streaming not supported")
        return
    }
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(200)

    for event := range events {
        if ctx.Err() != nil {
            return
        }
        chunk := translateFn(event)
        fmt.Fprintf(w, "data: %s\n\n", chunk)
        flusher.Flush()
    }

    fmt.Fprintf(w, "data: [DONE]\n\n")
    flusher.Flush()
}
```

### Retry e Body Buffering

O body é bufferizado pelo handler (lido completo com `io.ReadAll`) e parseado
em `ChatRequest` antes de chamar o use case. Isso resolve o problema de retry
naturalmente: o use case recebe uma struct (`ChatRequest`), não um `io.Reader`,
então pode chamar `chatSender.SendChat()` múltiplas vezes no retry loop
sem precisar "rebobinar" o body.

```
Request body → [handler: io.ReadAll] → []byte → [json.Unmarshal] → OpenAIChatRequest
    → [translator] → ChatRequest (struct) → [use case retry loop]
                                                    │
                                              pode chamar SendChat(chatReq)
                                              N vezes — chatReq é uma struct,
                                              não um stream.
```
