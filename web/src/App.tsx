import { lazy, Suspense, useCallback, useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createBrowserRouter,
  Navigate,
  RouterProvider,
} from 'react-router-dom'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Layout } from '@/components/Layout'
import { getAdminKey, setAdminKey } from '@/api/client'

const Dashboard = lazy(() => import('@/routes/Dashboard'))
const Accounts = lazy(() => import('@/routes/Accounts'))
const ApiKeys = lazy(() => import('@/routes/ApiKeys'))
const Logs = lazy(() => import('@/routes/Logs'))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 10000),
      staleTime: 5000,
    },
  },
})

function RouteError() {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-20">
      <h2 className="text-lg font-semibold">Something went wrong</h2>
      <p className="text-sm text-muted-foreground">
        An error occurred while rendering this page.
      </p>
    </div>
  )
}

function Loading() {
  return (
    <div className="flex items-center justify-center py-20">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
    </div>
  )
}

function RouteWrapper({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary fallback={<RouteError />}>
      <Suspense fallback={<Loading />}>{children}</Suspense>
    </ErrorBoundary>
  )
}

const router = createBrowserRouter(
  [
    {
      path: '/',
      element: <Layout />,
      children: [
        { index: true, element: <Navigate to="/dashboard" replace /> },
        {
          path: 'dashboard',
          element: (
            <RouteWrapper>
              <Dashboard />
            </RouteWrapper>
          ),
        },
        {
          path: 'accounts',
          element: (
            <RouteWrapper>
              <Accounts />
            </RouteWrapper>
          ),
        },
        {
          path: 'keys',
          element: (
            <RouteWrapper>
              <ApiKeys />
            </RouteWrapper>
          ),
        },
        {
          path: 'logs',
          element: (
            <RouteWrapper>
              <Logs />
            </RouteWrapper>
          ),
        },
      ],
    },
  ],
  { basename: '/app' },
)

function AuthSetup({ onSubmit }: { onSubmit: (key: string) => void }) {
  const [key, setKey] = useState('')
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-4 rounded-lg border p-6">
        <h1 className="text-xl font-semibold">Kestrel Setup</h1>
        <p className="text-sm text-muted-foreground">
          Enter your admin key to continue.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (key.trim()) onSubmit(key.trim())
          }}
          className="space-y-3"
        >
          <input
            type="password"
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="Admin key"
            className="w-full rounded-md border bg-background px-3 py-2 text-sm"
            autoFocus
          />
          <button
            type="submit"
            className="w-full rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
          >
            Connect
          </button>
        </form>
      </div>
    </div>
  )
}

function ErrorPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="text-center">
        <h1 className="text-2xl font-bold">Something went wrong</h1>
        <p className="mt-2 text-muted-foreground">
          An unexpected error occurred.
        </p>
      </div>
    </div>
  )
}

export default function App() {
  const [hasKey, setHasKey] = useState(() => getAdminKey() !== null)

  const handleAuthError = useCallback(() => {
    setHasKey(false)
  }, [])

  useEffect(() => {
    window.addEventListener('kestrel:auth_error', handleAuthError)
    return () => {
      window.removeEventListener('kestrel:auth_error', handleAuthError)
    }
  }, [handleAuthError])

  if (!hasKey) {
    return (
      <AuthSetup
        onSubmit={(key) => {
          setAdminKey(key)
          setHasKey(true)
        }}
      />
    )
  }

  return (
    <ErrorBoundary fallback={<ErrorPage />}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ErrorBoundary>
  )
}
