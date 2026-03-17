# Kestrel — Frontend (SPA)

## Stack

| Decisão       | Escolha            | Razão                                           |
|---------------|--------------------|-------------------------------------------------|
| Framework     | React 19 + Vite    | Ecossistema maduro, familiaridade, tooling       |
| Linguagem     | TypeScript         | Type safety no frontend                          |
| Styling       | Tailwind CSS 4     | Utility-first, sem CSS custom, build rápido      |
| Data Fetching | TanStack Query + fetch | Cache, dedup, mutations, refetch |
| Roteamento    | React Router 7     | SPA client-side routing                          |
| Componentes   | shadcn/ui          | Componentes acessíveis, copy-paste, sem vendor lock |
| Build         | Vite               | HMR rápido, build otimizado, output estático     |
| Validação     | zod                | Schemas de validação para formulários            |
| Deploy        | Embed no binário Go | `embed.FS` serve static files, binário único    |

## Separação de Camadas

Mesmo para CRUD simples, o frontend segue separação mínima de responsabilidades:

| Camada            | Diretório          | Responsabilidade                                                   |
|-------------------|--------------------|--------------------------------------------------------------------|
| Tipos             | `src/types/`       | Tipos TypeScript espelhando entidades do domínio Go (`Account`, `APIKey`, `RequestLog`) |
| API Adapter       | `src/api/`         | Único ponto de contato com o backend — fetch wrapper, interceptors |
| Hooks             | `src/hooks/`       | TanStack Query hooks — conectam API adapter aos componentes de rota   |
| Componentes       | `src/components/`  | Componentes de apresentação puros (recebem props, renderizam)      |
| Rotas/Containers  | `src/routes/`      | Containers/pages que orquestram hooks + componentes                |

Para v1 CRUD, arquitetura simplificada. Componentes de rota atuam como containers. Se complexidade crescer, extrair para hooks de orquestração.

## Estrutura

```
web/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── types/
│   │   ├── account.ts             # Account, AccountStatus
│   │   ├── apiKey.ts              # APIKey
│   │   ├── requestLog.ts          # RequestLog
│   │   └── health.ts              # HealthResponse
│   │
│   ├── routes/
│   │   ├── Dashboard.tsx          # Visão geral (health, status contas)
│   │   ├── Accounts.tsx           # Lista de contas Claude
│   │   ├── AccountForm.tsx        # Criar/editar conta
│   │   ├── ApiKeys.tsx            # Lista de API keys
│   │   ├── ApiKeyForm.tsx         # Criar API key
│   │   └── Logs.tsx               # Request logs com filtros
│   │
│   ├── components/
│   │   ├── Layout.tsx             # Shell (sidebar + content)
│   │   ├── Sidebar.tsx            # Navegação
│   │   ├── StatusBadge.tsx        # active/cooldown/disabled
│   │   ├── AccountCard.tsx        # Card de conta com status
│   │   ├── LogTable.tsx           # Tabela de logs paginada
│   │   ├── StatsCard.tsx          # Métricas numéricas
│   │   └── ConfirmDialog.tsx      # Dialog de confirmação
│   │
│   ├── api/
│   │   ├── client.ts              # Fetch wrapper com base URL + admin key + interceptors
│   │   ├── accounts.ts            # CRUD accounts
│   │   ├── keys.ts                # CRUD API keys
│   │   ├── logs.ts                # Query logs
│   │   └── health.ts              # Health check
│   │
│   ├── hooks/
│   │   ├── useAccounts.ts         # React Query hooks para accounts
│   │   ├── useApiKeys.ts          # React Query hooks para keys
│   │   └── useLogs.ts             # React Query hooks para logs
│   │
│   └── lib/
│       └── utils.ts               # Formatters, helpers
│
├── tests/
│   ├── mocks/
│   │   └── handlers.ts            # MSW handlers
│   ├── api/
│   │   └── client.test.ts         # Testa interceptors, error handling
│   ├── components/
│   │   ├── StatusBadge.test.tsx
│   │   └── AccountCard.test.tsx
│   └── routes/
│       └── Accounts.test.tsx      # Integration com mock API
│
├── index.html
├── vite.config.ts
├── vitest.config.ts
├── tsconfig.json
└── package.json
```

## Páginas

### Dashboard
```
┌─────────────────────────────────────────────────┐
│  Kestrel                                     │
├──────────┬──────────────────────────────────────┤
│          │                                      │
│ Dashboard│  ┌──────┐ ┌──────┐ ┌──────┐         │
│ Accounts │  │Active│ │Cooldown│ │Disabled│       │
│ API Keys │  │  3   │ │  1   │ │  1   │         │
│ Logs     │  └──────┘ └──────┘ └──────┘         │
│          │                                      │
│          │  ┌──────┐ ┌──────┐ ┌──────┐         │
│          │  │Reqs  │ │Tokens│ │Uptime│         │
│          │  │Today │ │Today │ │      │         │
│          │  │ 1.2k │ │ 450k │ │ 12h  │         │
│          │  └──────┘ └──────┘ └──────┘         │
│          │                                      │
│          │  Recent Requests                     │
│          │  ┌───────────────────────────────┐   │
│          │  │ req_abc | sonnet | 200 | 1.2s│   │
│          │  │ req_def | sonnet | 429 | 0.3s│   │
│          │  │ req_ghi | opus   | 200 | 3.1s│   │
│          │  └───────────────────────────────┘   │
└──────────┴──────────────────────────────────────┘
```

### Accounts
```
┌──────────────────────────────────────────────────┐
│  Accounts                        [+ Add Account] │
├──────────────────────────────────────────────────┤
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ claude-pro-1          ● Active    P:0      │  │
│  │ sk-ant-...a3f         Last used: 2min ago  │  │
│  │                       [Edit] [Reset] [Del] │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ claude-pro-2          ◐ Cooldown  P:1      │  │
│  │ sk-ant-...b7c         Cooldown: 45s left   │  │
│  │ Error: 429 rate limit [Edit] [Reset] [Del] │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ┌────────────────────────────────────────────┐  │
│  │ claude-backup         ○ Disabled  P:2      │  │
│  │ sk-ant-...d9e         Auth error           │  │
│  │                       [Edit] [Reset] [Del] │  │
│  └────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

### Logs
```
┌──────────────────────────────────────────────────┐
│  Request Logs                                    │
├──────────────────────────────────────────────────┤
│  Filters: [Model ▾] [Status ▾] [Account ▾]      │
│           [From: ___] [To: ___] [Search]         │
├──────────────────────────────────────────────────┤
│  ID      │ Model    │ Status │ Tokens │ Latency  │
│──────────┼──────────┼────────┼────────┼──────────│
│  req_abc │ sonnet   │  200   │ 4.3k   │ 1.2s     │
│  req_def │ sonnet   │  429   │   —    │ 0.3s     │
│  req_ghi │ opus     │  200   │ 12.1k  │ 3.1s     │
│  req_jkl │ sonnet   │  200   │ 2.8k   │ 0.9s     │
│──────────┴──────────┴────────┴────────┴──────────│
│  ◀ 1 2 3 ... 12 ▶              Showing 1-50/580  │
└──────────────────────────────────────────────────┘
```

## Integração com Go

### Build
```makefile
# Makefile
.PHONY: build-web build

build-web:
	cd web && npm run build

build: build-web
	go build -o kestrel ./cmd/kestrel
```

### Embed no binário
```go
// cmd/kestrel/embed.go
package main

import "embed"

//go:embed all:web/dist
var webFS embed.FS
```

> **Nota:** O arquivo `embed.go` fica em `cmd/kestrel/` e o build copia `web/dist` para `cmd/kestrel/web/dist` via Makefile antes de compilar.

```go
// Router setup
r.Handle("/app/*", http.StripPrefix("/app",
    http.FileServer(http.FS(webSubFS))))
```

### CORS para desenvolvimento

Em desenvolvimento, o Vite dev server roda em porta separada. O Go server inclui middleware CORS habilitado apenas em `LOG_FORMAT=text` (dev mode):

```go
if cfg.IsDev() {
    r.Use(corsMiddleware) // permite localhost:5173
}
```

### Roteamento
```
/v1/*          → Proxy API (Bearer token)
/health        → Health endpoint (público)
/admin/*       → Admin API (X-Admin-Key)
/app/*         → Frontend SPA (static files)
/app           → index.html (SPA entrypoint)
```

Rotas são lazy-loaded para minimizar bundle size:
```typescript
const Dashboard = lazy(() => import('./routes/Dashboard'));
const Accounts = lazy(() => import('./routes/Accounts'));
const Logs = lazy(() => import('./routes/Logs'));
```

## Tratamento de Auth Error

O `api/client.ts` implementa interceptor que detecta erros de autenticação:

```typescript
// api/client.ts
interface StoredAuth {
  version: number;
  adminKey: string;
}

const STORAGE_KEY = 'kestrel_auth';
const CURRENT_VERSION = 1;

function getAdminKey(): string | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    const stored: StoredAuth = JSON.parse(raw);
    if (stored.version !== CURRENT_VERSION) {
      localStorage.removeItem(STORAGE_KEY);
      return null;
    }
    return stored.adminKey;
  } catch {
    localStorage.removeItem(STORAGE_KEY);
    return null;
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const adminKey = getAdminKey();

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Key': adminKey ?? '',
      ...options?.headers,
    },
  });

  if (res.status === 401 || res.status === 403) {
    // Admin key inválida ou ausente — dispara evento para UI
    window.dispatchEvent(new CustomEvent('kestrel:auth_error'));
    throw new AuthError('Admin key inválida ou ausente');
  }

  if (!res.ok) {
    const body = await res.json();
    throw new ApiError(body.error?.message ?? 'Unknown error', res.status);
  }

  return res.json();
}
```

Fluxo de autenticação:
- Admin key armazenada em `localStorage` com schema versionado (`kestrel_auth`)
- Se não houver key configurada: tela de setup inicial pedindo a key
- Se key retornar 401/403: modal/banner pedindo reconfiguração
- O componente `App.tsx` escuta o evento `kestrel:auth_error` e exibe o prompt de configuração

> **Risco conhecido (v1):** Admin key em localStorage é acessível via XSS. Aceitável para painel admin local. Produção futura: migrar para httpOnly cookie com session-based auth.

## Error Boundaries

ErrorBoundary global no `App.tsx` captura erros de renderização:

```tsx
<ErrorBoundary fallback={<ErrorPage />}>
  <QueryClientProvider client={queryClient}>
    <RouterProvider router={router} />
  </QueryClientProvider>
</ErrorBoundary>
```

Cada rota tem seu próprio ErrorBoundary + Suspense para isolamento de falhas:

```tsx
// Cada rota tem seu próprio ErrorBoundary + Suspense
<Route path="/accounts" element={
  <ErrorBoundary fallback={<RouteError />}>
    <Suspense fallback={<Loading />}>
      <Accounts />
    </Suspense>
  </ErrorBoundary>
} />
```

**Nota:** ErrorBoundary por rota garante que um erro em uma página não derrube o app inteiro.

React Query configurado com defaults globais:
```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 10000),
      staleTime: 5000,
    },
  },
});
```

## Auto-refresh de status

```typescript
// useAccounts.ts
export function useAccounts() {
  return useQuery({
    queryKey: ['accounts'],
    queryFn: () => api.getAccounts(),
    refetchInterval: 5000,  // Poll a cada 5s para status atualizado
  });
}

// useHealth.ts
export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: () => api.getHealth(),
    refetchInterval: 10000,
  });
}
```

### Polling vs SSE

v1: polling via `refetchInterval` (5s contas, 10s health). Trade-off consciente — simplicidade sobre real-time. O backend já suporta SSE; migração para SSE push é melhoria futura que não requer mudança de arquitetura.

## Testes

Ferramentas: **vitest** + **@testing-library/react** + **msw** (mock service worker para API).

```
web/
├── tests/
│   ├── mocks/handlers.ts               # MSW handlers compartilhados
│   ├── api/client.test.ts              # Interceptors, error handling, auth flow
│   ├── components/StatusBadge.test.tsx  # Renderização por status (active/cooldown/disabled)
│   ├── components/AccountCard.test.tsx  # Props, ações (edit/reset/delete)
│   └── routes/Accounts.test.tsx        # Integration com mock API (msw)
├── vitest.config.ts
```

Escopo de testes para v1:
- **`client.test.ts`** — interceptor de auth (401/403 dispara evento), headers corretos, error parsing
- **`StatusBadge.test.tsx`** — renderiza badge correto para cada status, classes CSS esperadas
- **`AccountCard.test.tsx`** — exibe dados da conta, callbacks de ação chamados corretamente
- **`Accounts.test.tsx`** — integração: lista contas via mock API (msw), testa loading/error/success states

Formulários (AccountForm, ApiKeyForm) usam validação com **zod** para feedback client-side antes do submit. Schemas zod espelham as regras do backend.

Configuração msw para testes:
```typescript
// tests/mocks/handlers.ts
import { http, HttpResponse } from 'msw';

export const handlers = [
  http.get('/admin/accounts', () => {
    return HttpResponse.json({
      data: [
        { id: 'acc_001', name: 'claude-pro-1', status: 'active', priority: 0 },
      ],
    });
  }),
];
```

## Fase de implementação

Adiciona à **Fase 7** (antes era só deploy):

```
Fase 7 — Frontend + Deploy

Arquivos:
  web/   (toda a estrutura acima)
  cmd/kestrel/embed.go  (embed.FS)
  Dockerfile  (multi-stage: node build + go build)
  docker-compose.yml
```
