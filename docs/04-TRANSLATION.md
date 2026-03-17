# OmniRouter Go — Tradução OpenAI ↔ Claude

## Por que traduzir

Claude Code envia requests no formato OpenAI (`/v1/chat/completions`).
A API da Anthropic espera formato Claude (`/v1/messages`).
O proxy traduz na ida e na volta, transparente para o cliente.

## Request: OpenAI → Claude

### Mapeamento de campos

```
OpenAI Request                          Claude Request
──────────────                          ──────────────
POST /v1/chat/completions               POST /v1/messages
{                                       {
  "model": "claude-sonnet-4-5"     →      "model": "claude-sonnet-4-5",
  "messages": [                   →      "system": "You are...",
    {"role":"system","content":"You are..."},
    {"role":"user","content":"Hi"},→      "messages": [
    {"role":"assistant","content":"Hello"},  {"role":"user","content":"Hi"},
    {"role":"user","content":"..."}         {"role":"assistant","content":"Hello"},
  ],                                        {"role":"user","content":"..."}
  "max_tokens": 4096,             →      ],
  "temperature": 0.7,             →      "max_tokens": 4096,
  "stream": true,                 →      "temperature": 0.7,
  "tools": [...],                 →      "stream": true,
  "tool_choice": "auto"           →      "tools": [...],  (formato diferente)
}                                        "tool_choice": {"type":"auto"}
                                       }

Mapeamento completo tool_choice:
  OpenAI "auto"                                    → Claude {"type":"auto"}
  OpenAI "none"                                    → Omitir tool_choice no Claude
  OpenAI "required"                                → Claude {"type":"any"}
  OpenAI {"type":"function","function":{"name":"X"}} → Claude {"type":"tool","name":"X"}
```

### Regras de tradução

#### 1. System messages → campo `system`
```
OpenAI: messages[i].role == "system" → extrair para campo system separado
Claude: system = join de todas as system messages com "\n"

Se múltiplas system messages intercaladas:
  → concatenar todas, manter ordem
  → remover dos messages
```

#### 2. Messages — role mapping
```
OpenAI role    → Claude role
"system"       → extraído (ver acima)
"user"         → "user"
"assistant"    → "assistant"
"tool"         → "user" (com content block tipo tool_result)
```

#### 3. Content blocks
```
OpenAI: content pode ser string ou array de {type, text/image_url}
Claude: content é sempre array de blocks

String simples:
  OpenAI: {"role":"user", "content":"hello"}
  Claude: {"role":"user", "content":[{"type":"text","text":"hello"}]}

Com imagem:
  OpenAI: {"type":"image_url","image_url":{"url":"data:image/png;base64,..."}}
  Claude: {"type":"image","source":{"type":"base64","media_type":"image/png","data":"..."}}
```

#### 4. Tools
```
OpenAI:
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "Get weather",
      "parameters": {
        "type": "object",
        "properties": { "city": {"type":"string"} },
        "required": ["city"]
      }
    }
  }]

Claude:
  "tools": [{
    "name": "get_weather",
    "description": "Get weather",
    "input_schema": {
      "type": "object",
      "properties": { "city": {"type":"string"} },
      "required": ["city"]
    }
  }]
```

#### 5. Tool calls (assistant → client)
```
OpenAI (assistant message com tool_calls):
  {"role":"assistant","tool_calls":[{
    "id": "call_123",
    "type": "function",
    "function": {"name":"get_weather","arguments":"{\"city\":\"SP\"}"}
  }]}

Claude (assistant message com tool_use block):
  {"role":"assistant","content":[{
    "type": "tool_use",
    "id": "toolu_123",
    "name": "get_weather",
    "input": {"city": "SP"}
  }]}
```

#### 6. Tool results (client → assistant)
```
OpenAI:
  {"role":"tool","tool_call_id":"call_123","content":"25°C sunny"}

Claude:
  {"role":"user","content":[{
    "type": "tool_result",
    "tool_use_id": "toolu_123",
    "content": "25°C sunny"
  }]}

**Mapeamento de tool_call_id:** passthrough — o ID gerado pelo Claude (`toolu_123`)
é usado diretamente no formato OpenAI (`id: "toolu_123"`). O cliente envia de volta
o mesmo ID no tool_result (`tool_call_id: "toolu_123"`). O proxy repassa para Claude
sem conversão. Não há mapa bidirecional — IDs são transparentes.

IMPORTANTE: Claude exige tool_result em mensagem separada do user,
imediatamente após a mensagem assistant com tool_use.
```

#### 7. Thinking / Extended Thinking

**Request (tradução):**
```
Claude Code envia campo não-oficial no request OpenAI:
  {"thinking":{"type":"enabled","budget_tokens":10000}}

O proxy faz passthrough: se o campo "thinking" existir no request OpenAI,
traduz diretamente para o formato Claude (mesma estrutura).
Se não existir, não adiciona — thinking é opt-in pelo cliente.
O budget_tokens vem do cliente, não é hardcoded pelo proxy.
```

**Response (tradução):**
```
Claude response: content blocks com type "thinking"
  {"type":"thinking","thinking":"Let me analyze..."}

OpenAI response: campo reasoning_content
  {"role":"assistant","reasoning_content":"Let me analyze...","content":"Answer"}

Streaming: thinking_delta → delta.reasoning_content
```

#### 8. Max tokens
```
Se não especificado:
  → default 8192 (suficiente para maioria dos usos)

Se modelo suporta extended context ([1m] suffix):
  → injetar header "anthropic-beta: max-tokens-3-5-sonnet-2025-04-14"
```

## Response: Claude → OpenAI

### Non-streaming

```
Claude Response                         OpenAI Response
──────────────                         ───────────────
{                                      {
  "id": "msg_123",              →        "id": "chatcmpl-msg_123",
  "type": "message",                     "object": "chat.completion",
  "role": "assistant",                   "created": 1710000000,
  "content": [                           "model": "claude-sonnet-4-5",
    {"type":"thinking","thinking":"..."},   "choices": [{
    {"type":"text","text":"Hello"}          "index": 0,
  ],                              →        "message": {
  "model": "claude-sonnet-4-5",              "role": "assistant",
  "stop_reason": "end_turn",     →          "content": "Hello",
  "usage": {                     →          "reasoning_content": "...",
    "input_tokens": 100,                  },
    "output_tokens": 50                   "finish_reason": "stop"
  }                                     }],
}                                       "usage": {
                                          "prompt_tokens": 100,
                                          "completion_tokens": 50,
                                          "total_tokens": 150
                                        }
                                       }
```

### Streaming (SSE)

```
Claude SSE Event                        OpenAI SSE Chunk
────────────────                        ────────────────
event: message_start             →     data: {"id":"chatcmpl-...","choices":[{
data: {"type":"message_start",           "delta":{"role":"assistant"}}]}
  "message":{"id":"msg_123",...}}

event: content_block_start       →     (noop se text, emit se thinking)
data: {"type":"content_block_start",
  "content_block":{"type":"text"}}

event: content_block_delta       →     data: {"choices":[{
data: {"type":"content_block_delta",     "delta":{"content":"H"}}]}
  "delta":{"type":"text_delta",
    "text":"H"}}

event: content_block_delta       →     data: {"choices":[{
data: {"type":"content_block_delta",     "delta":{"reasoning_content":"..."}}]}
  "delta":{"type":"thinking_delta",
    "thinking":"..."}}

event: content_block_start       →     data: {"choices":[{"delta":{
data: {"content_block":{                  "tool_calls":[{"index":0,
  "type":"tool_use",                       "id":"call_123",
  "id":"toolu_123",                        "type":"function",
  "name":"get_weather"}}                   "function":{"name":"get_weather",
                                            "arguments":""}}]}}]}

event: content_block_delta       →     data: {"choices":[{"delta":{
data: {"delta":{                          "tool_calls":[{"index":0,
  "type":"input_json_delta",               "function":{"arguments":"{\"ci"}}]
  "partial_json":"{\"ci"}}                }}]}

event: content_block_stop        →     (noop — OpenAI não tem equivalente.
data: {"type":"content_block_stop"}     O proxy ignora este evento.)

event: message_delta             →     data: {"choices":[{
data: {"type":"message_delta",           "finish_reason":"stop"}],
  "delta":{"stop_reason":"end_turn"},    "usage":{"prompt_tokens":100,
  "usage":{"output_tokens":50}}           "completion_tokens":50}}

event: message_stop              →     data: [DONE]
```

### Mapeamento stop_reason → finish_reason

```
Claude              OpenAI
──────              ──────
"end_turn"      →   "stop"
"max_tokens"    →   "length"
"tool_use"      →   "tool_calls"
"stop_sequence" →   "stop"
```

## Headers necessários para a API Claude

```
POST /v1/messages HTTP/1.1
Host: api.anthropic.com
Content-Type: application/json
x-api-key: sk-ant-...
anthropic-version: 2023-06-01
anthropic-beta: prompt-caching-2024-07-31  (se usar caching)
```
