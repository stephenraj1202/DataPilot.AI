import axios from 'axios'

const AUTH_BASE = import.meta.env.VITE_AUTH_SERVICE_URL ?? 'http://localhost:8081'

const authAxios = axios.create({
  baseURL: AUTH_BASE,
  withCredentials: true,
})

export interface LoginResponse {
  access_token: string
  expires_in: number
}

export interface RegisterResponse {
  user_id: string
  verification_sent: boolean
}

export interface GoogleAuthResponse {
  access_token?: string
  refresh_token?: string
  expires_in?: number
  new_user?: boolean
  // returned when new_user=true and terms not yet accepted
  email?: string
  name?: string
  picture?: string
  error?: string
  message?: string
}

export const authService = {
  async login(email: string, password: string): Promise<LoginResponse> {
    const { data } = await authAxios.post('/auth/login', { email, password })
    return data
  },

  async register(email: string, password: string, accountName: string, termsAccepted: boolean): Promise<RegisterResponse> {
    const { data } = await authAxios.post('/auth/register', {
      email,
      password,
      account_name: accountName,
      terms_accepted: termsAccepted,
    })
    return data
  },

  async refresh(): Promise<LoginResponse> {
    const { data } = await authAxios.post('/auth/refresh', {})
    return data
  },

  async forgotPassword(email: string): Promise<void> {
    await authAxios.post('/auth/forgot-password', { email })
  },

  async resetPassword(token: string, password: string): Promise<void> {
    await authAxios.post('/auth/reset-password', { token, password })
  },

  async verifyEmail(token: string): Promise<void> {
    await authAxios.get(`/auth/verify-email?token=${token}`)
  },

  async logout(): Promise<void> {
    try {
      await authAxios.post('/auth/logout', {})
    } catch {
      // ignore
    }
  },

  async googleAuth(
    credential: string,
    opts?: { accountName?: string; termsAccepted?: boolean },
  ): Promise<GoogleAuthResponse> {
    try {
      const { data } = await authAxios.post('/auth/google', {
        credential,
        account_name: opts?.accountName ?? '',
        terms_accepted: opts?.termsAccepted ?? false,
      })
      return data
    } catch (err: unknown) {
      const resp = (err as { response?: { data?: GoogleAuthResponse } })?.response?.data
      if (resp) return resp
      throw err
    }
  },

  async sendOTP(email: string): Promise<void> {
    await authAxios.post('/auth/send-otp', { email })
  },

  async verifyOTP(email: string, otp: string): Promise<{ access_token: string; expires_in: number }> {
    const { data } = await authAxios.post('/auth/verify-otp', { email, otp })
    return data
  },
}
