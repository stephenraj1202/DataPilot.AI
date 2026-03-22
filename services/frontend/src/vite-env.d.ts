/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_GATEWAY_URL: string
  readonly VITE_AUTH_SERVICE_URL: string
  readonly VITE_BILLING_SERVICE_URL: string
  readonly VITE_FINOPS_SERVICE_URL: string
  readonly VITE_AI_QUERY_ENGINE_URL: string
  readonly VITE_GOOGLE_CLIENT_ID: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
