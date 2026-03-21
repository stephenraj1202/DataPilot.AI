import React, { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { authService } from '../services/auth.service'
import { setAccessToken, getAccessToken, registerLogoutCallback } from '../services/api'

interface User {
  id: string
  email: string
  accountId: string
  role: string
  emailVerified: boolean
  name?: string
  avatar?: string
}

interface AuthContextValue {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
  register: (email: string, password: string, accountName: string, termsAccepted: boolean) => Promise<void>
  googleAuth: (credential: string, opts?: { accountName?: string; termsAccepted?: boolean }) => Promise<{ newUser?: boolean; email?: string; name?: string; picture?: string }>
}
const AuthContext = createContext<AuthContextValue | undefined>(undefined)

function decodeUser(token: string): User {
  const payload = JSON.parse(atob(token.split('.')[1]))
  return {
    id: payload.user_id,
    email: payload.email ?? '',
    accountId: payload.account_id,
    role: Array.isArray(payload.roles) ? (payload.roles[0] ?? 'user') : (payload.role ?? 'user'),
    emailVerified: payload.email_verified ?? true,
    name: payload.name ?? '',
    avatar: payload.avatar ?? '',
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const logout = useCallback(() => {
    setAccessToken(null)
    setUser(null)
  }, [])

  // Register logout callback so api.ts can trigger it on service-down / refresh failure
  useEffect(() => {
    registerLogoutCallback(logout)
  }, [logout])

  // On mount, try to refresh token to restore session
  useEffect(() => {
    const restore = async () => {
      try {
        const data = await authService.refresh()
        setAccessToken(data.access_token)
        setUser(decodeUser(data.access_token))
      } catch {
        // No valid session — that's fine
      } finally {
        setIsLoading(false)
      }
    }
    restore()
  }, [])

  const login = async (email: string, password: string) => {
    const data = await authService.login(email, password)
    setAccessToken(data.access_token)
    setUser(decodeUser(data.access_token))
  }

  const register = async (email: string, password: string, accountName: string, termsAccepted: boolean) => {
    await authService.register(email, password, accountName, termsAccepted)
  }

  const googleAuth = async (
    credential: string,
    opts?: { accountName?: string; termsAccepted?: boolean },
  ) => {
    const data = await authService.googleAuth(credential, opts)
    if (data.new_user && !opts?.termsAccepted) {
      // Backend says new user but terms not accepted — return info for frontend to show terms
      return { newUser: true, email: data.email, name: data.name, picture: data.picture }
    }
    setAccessToken(data.access_token)
    setUser(decodeUser(data.access_token))
    return {}
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!getAccessToken(),
        login,
        logout,
        register,
        googleAuth,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
