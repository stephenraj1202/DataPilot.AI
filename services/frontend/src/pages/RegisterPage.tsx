import { useEffect, useState } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import toast from 'react-hot-toast'
import LoadingSpinner from '../components/common/LoadingSpinner'

const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID ?? ''

interface LocationState {
  googleCredential?: string
  email?: string
  name?: string
  picture?: string
}

export default function RegisterPage() {
  const { register, googleAuth } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const state = (location.state ?? {}) as LocationState

  const [form, setForm] = useState({
    email: state.email ?? '',
    password: '',
    confirmPassword: '',
    accountName: state.name ? `${state.name}'s Organization` : '',
  })
  const [termsAccepted, setTermsAccepted] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const isGoogleFlow = !!state.googleCredential

  // Google Sign-In button (for non-Google flow)
  useEffect(() => {
    if (!GOOGLE_CLIENT_ID || isGoogleFlow) return
    const script = document.createElement('script')
    script.src = 'https://accounts.google.com/gsi/client'
    script.async = true
    script.onload = () => {
      window.google?.accounts.id.initialize({
        client_id: GOOGLE_CLIENT_ID,
        callback: handleGoogleCredential,
      })
      const btn = document.getElementById('google-register-btn')
      if (btn) {
        window.google?.accounts.id.renderButton(btn, {
          theme: 'outline',
          size: 'large',
          width: '100%',
          text: 'signup_with',
        })
      }
    }
    document.head.appendChild(script)
    return () => { document.head.removeChild(script) }
  }, [])

  const handleGoogleCredential = async (response: { credential: string }) => {
    if (!termsAccepted) {
      toast.error('Please accept the terms and conditions first')
      return
    }
    try {
      await googleAuth(response.credential, {
        accountName: form.accountName || undefined,
        termsAccepted: true,
      })
      navigate('/dashboard')
    } catch {
      toast.error('Google sign-up failed')
    }
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm(prev => ({ ...prev, [e.target.name]: e.target.value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!termsAccepted) {
      toast.error('Please accept the terms and conditions')
      return
    }

    // Google SSO flow — complete registration with terms
    if (isGoogleFlow && state.googleCredential) {
      setIsLoading(true)
      try {
        await googleAuth(state.googleCredential, {
          accountName: form.accountName,
          termsAccepted: true,
        })
        navigate('/dashboard')
      } catch {
        toast.error('Failed to complete sign-up')
      } finally {
        setIsLoading(false)
      }
      return
    }

    if (form.password !== form.confirmPassword) {
      toast.error('Passwords do not match')
      return
    }
    setIsLoading(true)
    try {
      await register(form.email, form.password, form.accountName, true)
      toast.success('Account created! Please check your email to verify.')
      navigate('/login')
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Registration failed'
      toast.error(msg)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-gray-900">
      <div className="w-full max-w-md">
        <div className="mb-8 text-center">
          <h1 className="text-3xl font-bold text-blue-600">DataPilot.AI</h1>
          <p className="mt-2 text-gray-600 dark:text-gray-400">
            {isGoogleFlow ? `Welcome, ${state.name ?? state.email}! Complete your account setup.` : 'Create your account'}
          </p>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-8 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          {/* Google avatar preview for SSO flow */}
          {isGoogleFlow && state.picture && (
            <div className="mb-4 flex items-center gap-3 rounded-xl bg-blue-50 p-3 dark:bg-blue-900/20">
              <img src={state.picture} alt="avatar" className="h-10 w-10 rounded-full" />
              <div>
                <p className="text-sm font-semibold text-gray-800 dark:text-white">{state.name}</p>
                <p className="text-xs text-gray-500">{state.email}</p>
              </div>
            </div>
          )}

          {/* Google Sign-Up button (non-Google flow) */}
          {GOOGLE_CLIENT_ID && !isGoogleFlow && (
            <>
              <div id="google-register-btn" className="mb-4 flex justify-center" />
              <div className="relative mb-4">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-200 dark:border-gray-600" />
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-white px-3 text-gray-400 dark:bg-gray-800">or sign up with email</span>
                </div>
              </div>
            </>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Organization Name</label>
              <input
                type="text"
                name="accountName"
                value={form.accountName}
                onChange={handleChange}
                required
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                placeholder="Acme Corp"
              />
            </div>

            {!isGoogleFlow && (
              <>
                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Email</label>
                  <input
                    type="email"
                    name="email"
                    value={form.email}
                    onChange={handleChange}
                    required
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    placeholder="you@example.com"
                  />
                </div>

                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Password</label>
                  <input
                    type="password"
                    name="password"
                    value={form.password}
                    onChange={handleChange}
                    required
                    minLength={12}
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    placeholder="Min 12 characters"
                  />
                </div>

                <div>
                  <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Confirm Password</label>
                  <input
                    type="password"
                    name="confirmPassword"
                    value={form.confirmPassword}
                    onChange={handleChange}
                    required
                    className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    placeholder="••••••••"
                  />
                </div>
              </>
            )}

            {/* Terms & Conditions */}
            <div className="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-600 dark:bg-gray-700/50">
              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={termsAccepted}
                  onChange={e => setTermsAccepted(e.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                <span className="text-sm text-gray-600 dark:text-gray-300">
                  I agree to the{' '}
                  <a href="/terms" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline dark:text-blue-400">
                    Terms of Service
                  </a>{' '}
                  and{' '}
                  <a href="/privacy" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline dark:text-blue-400">
                    Privacy Policy
                  </a>
                  . I understand that my data will be processed as described.
                </span>
              </label>
            </div>

            <button
              type="submit"
              disabled={isLoading || !termsAccepted}
              className="flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 py-2.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLoading ? <LoadingSpinner size="sm" /> : null}
              {isGoogleFlow ? 'Complete Sign-Up' : 'Create Account'}
            </button>
          </form>

          <p className="mt-4 text-center text-sm text-gray-600 dark:text-gray-400">
            Already have an account?{' '}
            <Link to="/login" className="text-blue-600 hover:underline dark:text-blue-400">
              Sign in
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
