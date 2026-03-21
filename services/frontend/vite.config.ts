import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiGateway = env.VITE_API_GATEWAY_URL || 'http://localhost:8080'
  const frontendPort = parseInt(env.VITE_FRONTEND_PORT || '3000', 10)

  return {
    plugins: [react()],
    server: {
      port: frontendPort,
      proxy: {
        '/api': {
          target: apiGateway,
          changeOrigin: true,
        },
      },
    },
  }
})
