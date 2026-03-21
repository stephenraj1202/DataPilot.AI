import { useState, useRef, useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { authService } from '../services/auth.service'
import { useAuth } from '../context/AuthContext'
import { setAccessToken } from '../services/api'
import toast from 'react-hot-toast'
import LoadingSpinner from '../components/common/LoadingSpinner'

interface LocationState {
  email?: string
  name?: string
  picture?: string
  credential?: string
}

export default function OtpPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const { googleAuth } = useAuth()
  const state = (location.state ?? {}) as LocationState

  const email = state.email ?? ''
  const [otp, setOtp] = useState(['', '', '', '', '', ''])
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const inputRefs = useRef<Array<HTMLInputElement | null>>([])

  // If no email in state, redirect to login
  useEffect(() => {
    if (!email) navigate('/login', { replace: true })
  }, [email, navigate])

  // Auto-send OTP on mount
  useEffect(() => {
    if (email) handleSendOTP()
  }, [])

  // Countdown timer for resend
  useEffect(() => {
    if (countdown <= 0) return
    const t = setTimeout(() => setCountdown(c => c - 1), 1000)
    return () => clearTimeout(t)
  }, [countdown])

  const handleSendOTP = async () => {
    setSending(true)
    try {
      await authService.sendOTP(email)
      setCountdown(60)
      toast.success('Verification code sent to your email')
    } catch {
      toast.error('Failed to send code. Please try again.')
    } finally {
      setSending(false)
    }
  }

  const handleInput = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return
    const next = [...otp]
    next[index] = value.slice(-1)
    setOtp(next)
    if (value && index < 5) inputRefs.current[index + 1]?.focus()
    if (next.every(d => d !== '') && next.join('').length === 6) {
      handleVerify(next.join(''))
    }
  }

  const handleKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !otp[index] && index > 0) {
      inputRefs.current[index - 1]?.focus()
    }
  }

  const handlePaste = (e: React.ClipboardEvent) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, 6)
    if (pasted.length === 6) {
      setOtp(pasted.split(''))
      handleVerify(pasted)
    }
  }

  const handleVerify = async (code: string) => {
    if (code.length !== 6) return
    setLoading(true)
    try {
      const data = await authService.verifyOTP(email, code)
      setAccessToken(data.access_token)
      toast.success('Email verified! Welcome to DataPilot.AI')
      navigate('/dashboard', { replace: true })
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Invalid code'
      toast.error(msg)
      setOtp(['', '', '', '', '', ''])
      inputRefs.current[0]?.focus()
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    handleVerify(otp.join(''))
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-indigo-50 via-white to-purple-50 px-4 dark:from-gray-900 dark:via-gray-900 dark:to-gray-800">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-indigo-600 text-3xl font-black text-white">D</div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Check your email</h1>
          <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
            We sent a 6-digit code to
          </p>
          <p className="mt-1 font-semibold text-gray-800 dark:text-white">{email}</p>
        </div>

        <div className="rounded-2xl border border-gray-200 bg-white p-8 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          {/* Avatar preview if from Google */}
          {state.picture && (
            <div className="mb-6 flex items-center gap-3 rounded-xl bg-indigo-50 p-3 dark:bg-indigo-900/20">
              <img src={state.picture} alt="avatar" className="h-10 w-10 rounded-full" />
              <div>
                <p className="text-sm font-semibold text-gray-800 dark:text-white">{state.name}</p>
                <p className="text-xs text-gray-500">{email}</p>
              </div>
            </div>
          )}

          <form onSubmit={handleSubmit}>
            <p className="mb-4 text-center text-sm text-gray-500 dark:text-gray-400">Enter the 6-digit verification code</p>

            {/* OTP input boxes */}
            <div className="mb-6 flex justify-center gap-2" onPaste={handlePaste}>
              {otp.map((digit, i) => (
                <input
                  key={i}
                  ref={el => { inputRefs.current[i] = el }}
                  type="text"
                  inputMode="numeric"
                  maxLength={1}
                  value={digit}
                  onChange={e => handleInput(i, e.target.value)}
                  onKeyDown={e => handleKeyDown(i, e)}
                  className="h-12 w-10 rounded-xl border-2 border-gray-200 bg-gray-50 text-center text-xl font-bold text-gray-900 transition-all focus:border-indigo-500 focus:bg-white focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:focus:border-indigo-400"
                  disabled={loading}
                />
              ))}
            </div>

            <button
              type="submit"
              disabled={loading || otp.join('').length !== 6}
              className="flex w-full items-center justify-center gap-2 rounded-xl bg-indigo-600 py-3 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors"
            >
              {loading ? <LoadingSpinner size="sm" /> : null}
              {loading ? 'Verifying...' : 'Verify & Continue'}
            </button>
          </form>

          <div className="mt-4 text-center">
            {countdown > 0 ? (
              <p className="text-sm text-gray-400">Resend code in {countdown}s</p>
            ) : (
              <button
                onClick={handleSendOTP}
                disabled={sending}
                className="text-sm font-medium text-indigo-600 hover:underline disabled:opacity-50 dark:text-indigo-400"
              >
                {sending ? 'Sending...' : 'Resend code'}
              </button>
            )}
          </div>
        </div>

        <p className="mt-4 text-center text-xs text-gray-400">
          Wrong account?{' '}
          <button onClick={() => navigate('/login')} className="text-indigo-600 hover:underline dark:text-indigo-400">
            Sign in with a different account
          </button>
        </p>
      </div>
    </div>
  )
}
