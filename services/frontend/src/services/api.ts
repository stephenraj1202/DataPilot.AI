import axios, { AxiosInstance, InternalAxiosRequestConfig } from 'axios'

// Access token stored in memory (not localStorage for security)
let accessToken: string | null = null
let _logoutCallback: (() => void) | null = null

export function setAccessToken(token: string | null) {
  accessToken = token
}

export function getAccessToken(): string | null {
  return accessToken
}

/** Register a logout callback so the interceptor can trigger it on auth failure or service down */
export function registerLogoutCallback(cb: () => void) {
  _logoutCallback = cb
}

function triggerLogout() {
  setAccessToken(null)
  if (_logoutCallback) {
    _logoutCallback()
  } else {
    window.location.href = '/login'
  }
}

const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_GATEWAY_URL ?? 'http://localhost:8080',
  withCredentials: true, // send httpOnly refresh_token cookie
})

// Attach access token to every request
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }
  return config
})

let isRefreshing = false
let refreshQueue: Array<(token: string) => void> = []

// Auto-refresh on 401; auto-logout on network error (service down) or refresh failure
api.interceptors.response.use(
  res => res,
  async error => {
    const original = error.config

    // Network error or service unavailable → logout immediately
    if (!error.response) {
      // Service is down — clear session and redirect to login
      triggerLogout()
      return Promise.reject(error)
    }

    if (error.response?.status === 401 && !original._retry) {
      original._retry = true

      if (isRefreshing) {
        return new Promise(resolve => {
          refreshQueue.push((token: string) => {
            original.headers.Authorization = `Bearer ${token}`
            resolve(api(original))
          })
        })
      }

      isRefreshing = true
      try {
        const { data } = await axios.post(
          `${import.meta.env.VITE_AUTH_SERVICE_URL ?? 'http://localhost:8081'}/auth/refresh`,
          {},
          { withCredentials: true },
        )
        const newToken = data.access_token
        setAccessToken(newToken)
        refreshQueue.forEach(cb => cb(newToken))
        refreshQueue = []
        original.headers.Authorization = `Bearer ${newToken}`
        return api(original)
      } catch {
        // Refresh failed — session is dead, logout
        triggerLogout()
        return Promise.reject(error)
      } finally {
        isRefreshing = false
      }
    }
    return Promise.reject(error)
  },
)

export default api
