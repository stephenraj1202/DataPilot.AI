import { useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import toast from 'react-hot-toast'
import Logo from '../components/common/Logo'
import { Cloud, BarChart2, Shield, Zap, Activity } from 'lucide-react'

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (cfg: object) => void
          renderButton: (el: HTMLElement, cfg: object) => void
        }
      }
    }
  }
}

const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID ?? ''

const PERKS = [
  { icon: Cloud,     label: 'Multi-cloud unified view',       sub: 'AWS · Azure · GCP in one dashboard' },
  { icon: BarChart2, label: 'AI-powered cost analytics',      sub: 'Detect anomalies, cut waste by 40%' },
  { icon: Zap,       label: 'Natural language queries',       sub: 'Ask questions, get instant SQL insights' },
  { icon: Activity,  label: 'Usage-based billing engine',     sub: 'Meter streams · Stripe overages · dry-run previews' },
  { icon: Shield,    label: 'Enterprise-grade security',      sub: 'RBAC · JWT · AES-256 encryption' },
]

export default function LoginPage() {
  const { googleAuth } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (!GOOGLE_CLIENT_ID) return
    const script = document.createElement('script')
    script.src = 'https://accounts.google.com/gsi/client'
    script.async = true
    script.onload = () => {
      window.google?.accounts.id.initialize({
        client_id: GOOGLE_CLIENT_ID,
        callback: handleGoogleCredential,
      })
      const btn = document.getElementById('google-signin-btn')
      if (btn) {
        window.google?.accounts.id.renderButton(btn, {
          theme: 'outline',
          size: 'large',
          width: '320',
          text: 'continue_with',
          shape: 'pill',
        })
      }
    }
    document.head.appendChild(script)
    return () => { try { document.head.removeChild(script) } catch { /* ignore */ } }
  }, [])

  const handleGoogleCredential = async (response: { credential: string }) => {
    try {
      const result = await googleAuth(response.credential, { termsAccepted: true })
      if (result.newUser) {
        navigate('/verify-otp', { state: { email: result.email, name: result.name, picture: result.picture, credential: response.credential } })
        return
      }
      navigate('/dashboard')
    } catch {
      toast.error('Google sign-in failed. Please try again.')
    }
  }

  return (
    <div className="flex min-h-screen bg-[#07070d]">

      {/* ── Left panel — dark branded ── */}
      <div className="relative hidden lg:flex lg:w-[52%] flex-col overflow-hidden">
        {/* Background layers */}
        <div className="absolute inset-0 bg-gradient-to-br from-[#0d0d1a] via-[#0f0f20] to-[#0a0a14]" />
        {/* Orbs */}
        <div className="absolute -left-32 top-1/4 h-[500px] w-[500px] rounded-full bg-indigo-600/20 blur-[120px]" />
        <div className="absolute right-0 bottom-1/4 h-[400px] w-[400px] rounded-full bg-violet-600/15 blur-[100px]" />
        {/* Grid */}
        <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.025)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.025)_1px,transparent_1px)] bg-[size:48px_48px]" />

        {/* Content */}
        <div className="relative z-10 flex flex-1 flex-col px-14 py-12">
          {/* Logo */}
          <div className="flex items-center gap-3">
            <Logo size={36} variant="light" />
          </div>

          {/* Headline */}
          <div className="mt-auto mb-auto pt-16">
            <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-indigo-500/25 bg-indigo-500/10 px-3.5 py-1 text-xs font-medium text-indigo-300">
              <span className="h-1.5 w-1.5 rounded-full bg-indigo-400 animate-pulse" />
              Trusted by 500+ FinOps teams
            </div>

            <h1 className="text-4xl font-extrabold leading-tight tracking-tight text-white xl:text-5xl">
              Cloud cost intelligence
              <br />
              <span className="bg-gradient-to-r from-indigo-400 via-violet-400 to-cyan-400 bg-clip-text text-transparent">
                powered by AI
              </span>
            </h1>

            <p className="mt-5 max-w-md text-base leading-relaxed text-gray-400">
              Unify your multi-cloud spending, detect anomalies automatically, and query your data with natural language.
            </p>

            {/* Perks */}
            <div className="mt-10 space-y-4">
              {PERKS.map(({ icon: Icon, label, sub }) => (
                <div key={label} className="flex items-start gap-4">
                  <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-indigo-500/15 border border-indigo-500/20">
                    <Icon className="h-4 w-4 text-indigo-400" />
                  </div>
                  <div>
                    <p className="text-sm font-semibold text-white">{label}</p>
                    <p className="text-xs text-gray-500">{sub}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* UBB callout */}
          <div className="mt-10 rounded-2xl border border-violet-500/20 bg-violet-500/[0.07] p-5">
            <div className="flex items-start gap-3">
              <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-xl bg-violet-500/20 border border-violet-500/25">
                <Activity className="h-4 w-4 text-violet-400" />
              </div>
              <div>
                <p className="text-xs font-bold text-violet-300 mb-1">Usage-Based Billing — built in</p>
                <p className="text-[11px] leading-relaxed text-gray-500">
                  Create metered streams, post usage events via API, and let Stripe handle overage billing automatically. Includes dry-run invoice previews before any charge.
                </p>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {['Metered streams', 'Stripe overages', 'Dry-run preview', 'Pay-as-you-go'].map(tag => (
                    <span key={tag} className="rounded-full border border-violet-500/20 bg-violet-500/10 px-2 py-0.5 text-[10px] font-medium text-violet-400">
                      {tag}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Bottom quote */}
          <div className="mt-auto pt-10">            <div className="rounded-2xl border border-white/5 bg-white/[0.03] p-5">
              <p className="text-sm italic leading-relaxed text-gray-400">
                "DataPilot.AI cut our cloud bill by 38% in the first month. The AI query engine alone saves our team hours every week."
              </p>
              <div className="mt-3 flex items-center gap-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-violet-600 text-xs font-bold text-white">
                  SK
                </div>
                <div>
                  <p className="text-xs font-semibold text-white">Steffi S.</p>
                  <p className="text-[10px] text-gray-500">Head of FinOps, TechCorp</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ── Right panel — sign in ── */}
      <div className="flex flex-1 flex-col items-center justify-center px-6 py-12 bg-[#07070d]">
        {/* Mobile logo */}
        <div className="mb-10 flex items-center gap-2 lg:hidden">
          <Logo size={32} variant="light" />
        </div>

        <div className="w-full max-w-[360px]">
          {/* Heading */}
          <div className="mb-8">
            <h2 className="text-2xl font-extrabold text-white">Welcome back</h2>
            <p className="mt-1.5 text-sm text-gray-500">Sign in to your DataPilot.AI account</p>
          </div>

          {/* Sign-in card */}
          <div className="rounded-2xl border border-white/8 bg-white/[0.04] p-8 backdrop-blur">
            {GOOGLE_CLIENT_ID ? (
              <div className="flex flex-col items-center gap-5">
                {/* Decorative divider */}
                <div className="flex w-full items-center gap-3">
                  <div className="flex-1 h-px bg-white/10" />
                  <span className="text-[11px] font-medium text-gray-600 uppercase tracking-widest">Continue with</span>
                  <div className="flex-1 h-px bg-white/10" />
                </div>

                <div id="google-signin-btn" className="flex justify-center" />

                <p className="text-center text-[11px] leading-relaxed text-gray-600">
                  By signing in, you agree to our{' '}
                  <Link to="/terms" className="text-indigo-400 hover:text-indigo-300 hover:underline transition-colors">Terms of Service</Link>
                  {' '}and{' '}
                  <Link to="/privacy" className="text-indigo-400 hover:text-indigo-300 hover:underline transition-colors">Privacy Policy</Link>
                </p>
              </div>
            ) : (
              <div className="rounded-xl border border-amber-500/20 bg-amber-500/10 p-4 text-center text-sm text-amber-400">
                Google Sign-In is not configured.
                <br />
                <code className="mt-1 block text-xs font-mono text-amber-500">VITE_GOOGLE_CLIENT_ID</code>
              </div>
            )}
          </div>

          {/* New user note */}
          <p className="mt-6 text-center text-xs leading-relaxed text-gray-600">
            New to DataPilot.AI? Sign in with Google — your account is created automatically with a{' '}
            <span className="text-indigo-400 font-medium">30-day free trial</span>.
          </p>

          {/* Back to landing */}
          <div className="mt-8 text-center">
            <Link to="/" className="text-xs text-gray-600 hover:text-gray-400 transition-colors">
              ← Back to homepage
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}
