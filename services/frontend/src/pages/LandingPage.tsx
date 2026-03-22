import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Cloud, BarChart2, MessageSquare, Shield, ArrowRight,
  CheckCircle, ChevronDown, Zap, TrendingDown, Globe,
  Activity, DollarSign, Code2, CreditCard, Play, Pause,
  BarChart3, Database, TrendingUp, CreditCard as BillingIcon,
} from 'lucide-react'
import Logo from '../components/common/Logo'

// ── Intersection-observer hook ──────────────────────────────────────────────
function useInView(threshold = 0.15) {
  const ref = useRef<HTMLDivElement>(null)
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const obs = new IntersectionObserver(([e]) => { if (e.isIntersecting) setVisible(true) }, { threshold })
    obs.observe(el)
    return () => obs.disconnect()
  }, [threshold])
  return { ref, visible }
}

// ── Animated counter ─────────────────────────────────────────────────────────
function Counter({ to, suffix = '' }: { to: number; suffix?: string }) {
  const [val, setVal] = useState(0)
  const { ref, visible } = useInView()
  useEffect(() => {
    if (!visible) return
    let start = 0
    const step = Math.ceil(to / 60)
    const t = setInterval(() => {
      start += step
      if (start >= to) { setVal(to); clearInterval(t) } else setVal(start)
    }, 16)
    return () => clearInterval(t)
  }, [visible, to])
  return <span ref={ref}>{val.toLocaleString()}{suffix}</span>
}

const FEATURES = [
  {
    icon: Cloud,
    color: 'from-blue-500 to-cyan-400',
    title: 'Multi-Cloud Unified View',
    desc: 'Connect AWS, Azure, and GCP in minutes. One dashboard for all your cloud spend.',
    stat: '3', statLabel: 'clouds',
  },
  {
    icon: BarChart2,
    color: 'from-violet-500 to-purple-400',
    title: 'FinOps Analytics',
    desc: 'Visualise cost trends, detect anomalies, and get AI-powered optimisation recommendations.',
    stat: '40', statLabel: '% avg savings',
  },
  {
    icon: MessageSquare,
    color: 'from-emerald-500 to-teal-400',
    title: 'Natural Language Queries',
    desc: 'Ask questions in plain English. Get instant SQL-powered insights from your databases.',
    stat: '10x', statLabel: 'faster insights',
  },
  {
    icon: Shield,
    color: 'from-orange-500 to-amber-400',
    title: 'Enterprise Security',
    desc: 'JWT auth, RBAC, rate limiting, and AES-256 credential encryption — built in.',
    stat: '99.9', statLabel: '% uptime SLA',
  },
]

const PLANS = [
  {
    name: 'Free',
    price: '$0',
    period: '/month',
    badge: null,
    color: 'border-gray-700',
    btn: 'bg-white/10 hover:bg-white/20 text-white',
    features: ['1 Cloud Account', '2 Database Connections', '100 req/min', 'Community support'],
    cta: 'Get Started',
  },
  {
    name: 'Base',
    price: '$10',
    period: '/month',
    badge: null,
    color: 'border-gray-700',
    btn: 'bg-white/10 hover:bg-white/20 text-white',
    features: ['3 Cloud Accounts', '5 Database Connections', '500 req/min', 'Email support'],
    cta: 'Buy Now',
  },
  {
    name: 'Pro',
    price: '$20',
    period: '/month',
    badge: 'Most Popular',
    color: 'border-indigo-500 ring-2 ring-indigo-500/40',
    btn: 'bg-indigo-600 hover:bg-indigo-500 text-white',
    features: ['10 Cloud Accounts', 'Unlimited Databases', '2000 req/min', 'Priority support'],
    cta: 'Buy Now',
  },
  {
    name: 'Enterprise',
    price: '$50',
    period: '/month',
    badge: null,
    color: 'border-gray-700',
    btn: 'bg-white/10 hover:bg-white/20 text-white',
    features: ['Unlimited Cloud Accounts', 'Unlimited Databases', '10000 req/min', 'Dedicated support'],
    cta: 'Buy Now',
  },
]

const STATS = [
  { icon: TrendingDown, value: 40, suffix: '%', label: 'Average cost reduction' },
  { icon: Globe, value: 500, suffix: '+', label: 'Teams worldwide' },
  { icon: Zap, value: 99, suffix: '.9%', label: 'Platform uptime' },
  { icon: Cloud, value: 3, suffix: '', label: 'Cloud providers' },
]

// ── Feature card with stagger animation ──────────────────────────────────────
function FeatureCard({ f, i }: { f: typeof FEATURES[0]; i: number }) {
  const { ref, visible } = useInView()
  return (
    <div
      ref={ref}
      style={{ transitionDelay: `${i * 100}ms` }}
      className={`group relative overflow-hidden rounded-2xl border border-white/10 bg-white/5 p-6 backdrop-blur transition-all duration-700 hover:-translate-y-1 hover:border-white/20 hover:bg-white/10 ${visible ? 'translate-y-0 opacity-100' : 'translate-y-8 opacity-0'}`}
    >
      {/* Glow blob */}
      <div className={`absolute -right-8 -top-8 h-32 w-32 rounded-full bg-gradient-to-br ${f.color} opacity-0 blur-2xl transition-opacity duration-500 group-hover:opacity-20`} />

      <div className={`mb-4 inline-flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br ${f.color} shadow-lg`}>
        <f.icon className="h-6 w-6 text-white" />
      </div>

      <h3 className="mb-2 text-lg font-bold text-white">{f.title}</h3>
      <p className="text-sm leading-relaxed text-gray-400">{f.desc}</p>

      <div className="mt-4 flex items-baseline gap-1">
        <span className={`bg-gradient-to-r ${f.color} bg-clip-text text-2xl font-extrabold text-transparent`}>{f.stat}</span>
        <span className="text-xs text-gray-500">{f.statLabel}</span>
      </div>
    </div>
  )
}

// ── Stat card ─────────────────────────────────────────────────────────────────
function StatCard({ s, i }: { s: typeof STATS[0]; i: number }) {
  const { ref, visible } = useInView()
  return (
    <div
      ref={ref}
      style={{ transitionDelay: `${i * 80}ms` }}
      className={`flex flex-col items-center gap-2 transition-all duration-700 ${visible ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0'}`}
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-indigo-500/20">
        <s.icon className="h-5 w-5 text-indigo-400" />
      </div>
      <p className="text-3xl font-extrabold text-white">
        <Counter to={typeof s.value === 'number' ? s.value : 0} suffix={s.suffix} />
      </p>
      <p className="text-sm text-gray-400">{s.label}</p>
    </div>
  )
}

// ── UBB Section ──────────────────────────────────────────────────────────────
const UBB_STEPS = [
  {
    icon: Activity,
    color: 'from-violet-500 to-purple-400',
    step: '01',
    title: 'Create a metered stream',
    desc: 'Define a named stream with a per-unit price. Each stream gets a unique API key for direct SDK integration.',
    code: `POST /billing/ubb/streams
{
  "stream_name": "API Requests",
  "overage_price_cents": 4
}`,
  },
  {
    icon: Code2,
    color: 'from-cyan-500 to-blue-400',
    step: '02',
    title: 'Post usage from your app',
    desc: 'Instrument your SaaS with a single API call. Every unit posted is recorded and priced — Stripe meters every event.',
    code: `POST /billing/ubb/streams/:id/usage
{
  "quantity": 250,
  "action": "increment"
}`,
  },
  {
    icon: DollarSign,
    color: 'from-emerald-500 to-teal-400',
    step: '03',
    title: 'Preview next invoice',
    desc: 'Run a live preview anytime. See exactly what will be charged: plan flat fee + total units × per-unit rate.',
    code: `GET /billing/ubb/invoice/dryrun
→ flat_fee_usd:  20.00
→ usage_usd:      4.62
→ total_usd:     24.62`,
  },
  {
    icon: CreditCard,
    color: 'from-orange-500 to-amber-400',
    step: '04',
    title: 'Automatic Stripe invoicing',
    desc: 'Stripe invoices total units × rate at period end. Auto-charged to saved card or via hosted invoice link.',
    code: `POST /billing/ubb/invoice/pay
→ paid: true
→ total_usd: 24.62
→ invoice_url: stripe.com/...`,
  },
]

function UBBSection() {
  const { ref, visible } = useInView()
  return (
    <section id="ubb" className="py-24 px-5 border-t border-white/5">
      <div className="mx-auto max-w-6xl">
        <div
          ref={ref}
          className={`mb-16 text-center transition-all duration-700 ${visible ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0'}`}
        >
          <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-violet-400">Usage-Based Billing</p>
          <h2 className="text-4xl font-extrabold tracking-tight">
            Meter your SaaS.{' '}
            <span className="bg-gradient-to-r from-violet-400 to-cyan-400 bg-clip-text text-transparent">Bill what you use.</span>
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-gray-400">
            Built-in UBB engine lets you create metered streams, post usage events, and invoice customers automatically via Stripe — every unit posted goes straight to the next bill.
          </p>
        </div>

        {/* Steps grid */}
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {UBB_STEPS.map((s, i) => {
            const { ref: sRef, visible: sVis } = useInView()
            return (
              <div
                key={s.step}
                ref={sRef}
                style={{ transitionDelay: `${i * 100}ms` }}
                className={`group relative flex flex-col overflow-hidden rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur transition-all duration-700 hover:-translate-y-1 hover:border-white/20 hover:bg-white/8 ${sVis ? 'translate-y-0 opacity-100' : 'translate-y-8 opacity-0'}`}
              >
                {/* Glow */}
                <div className={`absolute -right-8 -top-8 h-28 w-28 rounded-full bg-gradient-to-br ${s.color} opacity-0 blur-2xl transition-opacity duration-500 group-hover:opacity-20`} />

                <div className="mb-3 flex items-center gap-3">
                  <div className={`flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br ${s.color} shadow-md`}>
                    <s.icon className="h-4 w-4 text-white" />
                  </div>
                  <span className="text-xs font-bold text-gray-600">{s.step}</span>
                </div>

                <h3 className="mb-2 text-sm font-bold text-white">{s.title}</h3>
                <p className="text-xs leading-relaxed text-gray-400 flex-1">{s.desc}</p>

                {/* Code snippet */}
                <div className="mt-4 rounded-lg bg-black/40 border border-white/5 p-3 font-mono text-[10px] leading-relaxed text-gray-400 whitespace-pre">
                  {s.code}
                </div>
              </div>
            )
          })}
        </div>

        {/* Pricing example callout */}
        <div className="mt-12 rounded-2xl border border-violet-500/20 bg-violet-500/5 p-8">
          <div className="grid grid-cols-1 gap-8 sm:grid-cols-3 items-center">
            <div className="sm:col-span-2">
              <p className="text-xs font-bold uppercase tracking-widest text-violet-400 mb-2">Example — Pro plan + 2 streams</p>
              <h3 className="text-xl font-extrabold text-white mb-3">Flat plan fee + pay per unit posted</h3>
              <p className="text-sm text-gray-400 leading-relaxed">
                Your Pro plan covers the flat monthly fee. Every unit you post to a stream is billed at your configured per-unit rate. No free tiers, no surprise overages — just transparent pay-as-you-go.
              </p>
            </div>
            <div className="rounded-xl border border-white/10 bg-white/5 p-5 space-y-2.5 text-sm font-mono">
              {[
                { label: 'Pro plan flat fee', value: '$20.00', muted: false },
                { label: 'Stream A — 8,420 units', value: '$3.37', muted: false },
                { label: '(8,420 × $0.0004)', value: '', muted: true, indent: true },
                { label: 'Stream B — 1,250 units', value: '$1.25', muted: false },
                { label: '(1,250 × $0.0010)', value: '', muted: true, indent: true },
              ].map((row, i) => (
                <div key={i} className={`flex justify-between ${row.indent ? 'pl-3' : ''}`}>
                  <span className={row.muted ? 'text-gray-600' : 'text-gray-300'}>{row.label}</span>
                  {row.value && <span className="text-white font-bold">{row.value}</span>}
                </div>
              ))}
              <div className="border-t border-white/10 pt-2.5 flex justify-between font-extrabold text-white">
                <span>Total</span><span>$24.62</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

// ── Demo Section ─────────────────────────────────────────────────────────────

const DEMO_SLIDES = [
  {
    id: 'finops',
    label: 'FinOps Dashboard',
    icon: BarChart3,
    color: 'from-blue-500 to-cyan-400',
    bg: 'bg-blue-500/10',
    border: 'border-blue-500/30',
    title: 'Unified Cloud Cost Intelligence',
    desc: 'See all your AWS, Azure, and GCP spend in one place. AI detects anomalies and surfaces savings opportunities automatically.',
    screen: (
      <div className="h-full w-full bg-[#0f1117] rounded-xl p-4 font-mono text-xs overflow-hidden">
        <div className="flex items-center gap-2 mb-4">
          <div className="h-3 w-3 rounded-full bg-red-500/70" />
          <div className="h-3 w-3 rounded-full bg-yellow-500/70" />
          <div className="h-3 w-3 rounded-full bg-green-500/70" />
          <span className="ml-2 text-gray-500 text-[10px]">FinOps Dashboard</span>
        </div>
        {/* Mock cost chart */}
        <div className="grid grid-cols-3 gap-2 mb-4">
          {[
            { label: 'Total Spend', val: '$12,480', delta: '-8%', up: false },
            { label: 'Anomalies', val: '3', delta: 'detected', up: true },
            { label: 'Savings', val: '$2,100', delta: 'available', up: false },
          ].map(c => (
            <div key={c.label} className="rounded-lg bg-white/5 border border-white/10 p-2.5">
              <p className="text-[9px] text-gray-500 mb-1">{c.label}</p>
              <p className="text-sm font-black text-white">{c.val}</p>
              <p className={`text-[9px] font-semibold ${c.up ? 'text-red-400' : 'text-emerald-400'}`}>{c.delta}</p>
            </div>
          ))}
        </div>
        {/* Mock bar chart */}
        <div className="rounded-lg bg-white/5 border border-white/10 p-3 mb-3">
          <p className="text-[9px] text-gray-500 mb-2">Monthly Spend by Service</p>
          <div className="flex items-end gap-1.5 h-16">
            {[60, 80, 45, 90, 70, 55, 85, 40, 75, 95, 65, 50].map((h, i) => (
              <div key={i} className="flex-1 rounded-t"
                style={{ height: `${h}%`, background: `hsl(${220 + i * 5}, 70%, ${50 + i * 2}%)`, opacity: 0.8 }} />
            ))}
          </div>
        </div>
        {/* Anomaly alert */}
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-3 py-2 flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-red-400 animate-pulse" />
          <span className="text-[10px] text-red-300">Anomaly: EC2 spend +340% vs last week</span>
        </div>
      </div>
    ),
  },
  {
    id: 'query',
    label: 'AI Query',
    icon: MessageSquare,
    color: 'from-emerald-500 to-teal-400',
    bg: 'bg-emerald-500/10',
    border: 'border-emerald-500/30',
    title: 'Natural Language → Instant Charts',
    desc: 'Connect any database and ask questions in plain English. The AI generates SQL, executes it, and renders the perfect chart automatically.',
    screen: (
      <div className="h-full w-full bg-[#0f1117] rounded-xl p-4 font-mono text-xs overflow-hidden flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <div className="h-3 w-3 rounded-full bg-red-500/70" />
          <div className="h-3 w-3 rounded-full bg-yellow-500/70" />
          <div className="h-3 w-3 rounded-full bg-green-500/70" />
          <span className="ml-2 text-gray-500 text-[10px]">AI Query Chat</span>
        </div>
        {/* Chat messages */}
        <div className="flex justify-end">
          <div className="rounded-2xl rounded-tr-sm bg-gradient-to-br from-indigo-500 to-violet-600 px-3 py-2 text-[10px] text-white max-w-[80%]">
            Show me top 5 customers by revenue this month
          </div>
        </div>
        <div className="flex gap-2">
          <div className="h-6 w-6 rounded-full bg-gradient-to-br from-indigo-500 to-violet-600 flex items-center justify-center flex-shrink-0">
            <Zap className="h-3 w-3 text-white" />
          </div>
          <div className="rounded-2xl rounded-tl-sm bg-white/5 border border-white/10 px-3 py-2 flex-1">
            <p className="text-[9px] text-emerald-400 mb-1.5">✓ 5 results · 42ms</p>
            {/* Mini bar chart */}
            <div className="flex items-end gap-1 h-10 mb-1.5">
              {[95, 78, 65, 52, 41].map((h, i) => (
                <div key={i} className="flex-1 rounded-t bg-emerald-500/60" style={{ height: `${h}%` }} />
              ))}
            </div>
            <div className="flex justify-between text-[8px] text-gray-500">
              {['Acme', 'TechCo', 'StartX', 'DevInc', 'CloudY'].map(n => <span key={n}>{n}</span>)}
            </div>
          </div>
        </div>
        {/* SQL badge */}
        <div className="rounded-lg bg-gray-900 border border-white/5 px-3 py-2">
          <p className="text-[9px] text-gray-500 mb-1">▸ SQL</p>
          <p className="text-[9px] text-green-400">SELECT customer, SUM(revenue)</p>
          <p className="text-[9px] text-green-400">FROM orders WHERE month = NOW()</p>
          <p className="text-[9px] text-green-400">GROUP BY customer LIMIT 5</p>
        </div>
      </div>
    ),
  },
  {
    id: 'billing',
    label: 'Billing & UBB',
    icon: BillingIcon,
    color: 'from-violet-500 to-purple-400',
    bg: 'bg-violet-500/10',
    border: 'border-violet-500/30',
    title: 'Usage-Based Billing, Built In',
    desc: 'Create metered streams, post usage events via API, and let Stripe handle invoicing automatically. No third-party metering service needed.',
    screen: (
      <div className="h-full w-full bg-[#0f1117] rounded-xl p-4 font-mono text-xs overflow-hidden flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <div className="h-3 w-3 rounded-full bg-red-500/70" />
          <div className="h-3 w-3 rounded-full bg-yellow-500/70" />
          <div className="h-3 w-3 rounded-full bg-green-500/70" />
          <span className="ml-2 text-gray-500 text-[10px]">Usage-Based Billing</span>
        </div>
        {/* Stream cards */}
        {[
          { name: 'API Requests', units: 8420, rate: '$0.0004', billed: '$3.37' },
          { name: 'Data Exports', units: 1250, rate: '$0.0010', billed: '$1.25' },
        ].map(s => (
          <div key={s.name} className="rounded-lg bg-white/5 border border-white/10 p-2.5">
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-[10px] font-bold text-white">{s.name}</span>
              <span className="text-[9px] rounded-full bg-emerald-500/20 text-emerald-400 px-1.5 py-0.5">active</span>
            </div>
            <div className="flex justify-between text-[9px] text-gray-400 mb-1.5">
              <span>{s.units.toLocaleString()} units · {s.rate}/unit</span>
              <span className="text-violet-400 font-bold">{s.billed}</span>
            </div>
            <div className="h-1.5 w-full rounded-full bg-white/10">
              <div className="h-full rounded-full bg-gradient-to-r from-violet-500 to-purple-500" style={{ width: '65%' }} />
            </div>
          </div>
        ))}
        {/* Next invoice */}
        <div className="rounded-lg bg-violet-500/10 border border-violet-500/30 p-2.5">
          <div className="flex justify-between items-center">
            <span className="text-[9px] text-violet-300">Next invoice (preview)</span>
            <span className="text-sm font-black text-violet-300">$24.62</span>
          </div>
          <p className="text-[9px] text-gray-500 mt-0.5">Plan $20.00 + usage $4.62 · auto-charged Mar 31</p>
        </div>
      </div>
    ),
  },
  {
    id: 'security',
    label: 'Security & RBAC',
    icon: Shield,
    color: 'from-orange-500 to-amber-400',
    bg: 'bg-orange-500/10',
    border: 'border-orange-500/30',
    title: 'Enterprise-Grade Security',
    desc: 'JWT auth, multi-tenant RBAC, rate limiting, AES-256 credential encryption, audit logs, and 2FA — all built in from day one.',
    screen: (
      <div className="h-full w-full bg-[#0f1117] rounded-xl p-4 font-mono text-xs overflow-hidden flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <div className="h-3 w-3 rounded-full bg-red-500/70" />
          <div className="h-3 w-3 rounded-full bg-yellow-500/70" />
          <div className="h-3 w-3 rounded-full bg-green-500/70" />
          <span className="ml-2 text-gray-500 text-[10px]">Security & Audit</span>
        </div>
        {/* RBAC roles */}
        <div className="rounded-lg bg-white/5 border border-white/10 p-2.5">
          <p className="text-[9px] text-gray-500 mb-2">Role-Based Access Control</p>
          {[
            { role: 'Super Admin', perms: 'Full access', color: 'text-red-400' },
            { role: 'Admin', perms: 'Manage team', color: 'text-orange-400' },
            { role: 'Analyst', perms: 'Read + query', color: 'text-blue-400' },
            { role: 'Viewer', perms: 'Read only', color: 'text-gray-400' },
          ].map(r => (
            <div key={r.role} className="flex justify-between py-1 border-b border-white/5 last:border-0">
              <span className={`text-[10px] font-semibold ${r.color}`}>{r.role}</span>
              <span className="text-[9px] text-gray-500">{r.perms}</span>
            </div>
          ))}
        </div>
        {/* Audit log */}
        <div className="rounded-lg bg-white/5 border border-white/10 p-2.5 flex-1">
          <p className="text-[9px] text-gray-500 mb-2">Recent Audit Events</p>
          {[
            { action: 'login', user: 'alice@co.com', time: '2m ago' },
            { action: 'query_exec', user: 'bob@co.com', time: '5m ago' },
            { action: 'stream_create', user: 'alice@co.com', time: '12m ago' },
          ].map((e, i) => (
            <div key={i} className="flex items-center gap-2 py-1 border-b border-white/5 last:border-0">
              <div className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
              <span className="text-[9px] text-emerald-400 font-mono">{e.action}</span>
              <span className="text-[9px] text-gray-500 flex-1 truncate">{e.user}</span>
              <span className="text-[9px] text-gray-600">{e.time}</span>
            </div>
          ))}
        </div>
        <div className="flex gap-2">
          <div className="flex-1 rounded-lg bg-emerald-500/10 border border-emerald-500/30 px-2 py-1.5 text-center">
            <p className="text-[9px] text-emerald-400 font-bold">AES-256</p>
            <p className="text-[8px] text-gray-500">Encrypted</p>
          </div>
          <div className="flex-1 rounded-lg bg-blue-500/10 border border-blue-500/30 px-2 py-1.5 text-center">
            <p className="text-[9px] text-blue-400 font-bold">JWT + 2FA</p>
            <p className="text-[8px] text-gray-500">Auth</p>
          </div>
          <div className="flex-1 rounded-lg bg-orange-500/10 border border-orange-500/30 px-2 py-1.5 text-center">
            <p className="text-[9px] text-orange-400 font-bold">RBAC</p>
            <p className="text-[8px] text-gray-500">Roles</p>
          </div>
        </div>
      </div>
    ),
  },
]

function DemoSection() {
  const [active, setActive] = useState(0)
  const [playing, setPlaying] = useState(true)
  const { ref, visible } = useInView()

  // Auto-advance slides
  useEffect(() => {
    if (!playing) return
    const t = setInterval(() => setActive(a => (a + 1) % DEMO_SLIDES.length), 4000)
    return () => clearInterval(t)
  }, [playing])

  const slide = DEMO_SLIDES[active]
  const Icon = slide.icon

  return (
    <section id="demo" className="py-24 px-5 border-t border-white/5">
      <div className="mx-auto max-w-6xl">
        <div
          ref={ref}
          className={`mb-12 text-center transition-all duration-700 ${visible ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0'}`}
        >
          <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-indigo-400">Product Tour</p>
          <h2 className="text-4xl font-extrabold tracking-tight">
            See it in{' '}
            <span className="bg-gradient-to-r from-indigo-400 via-violet-400 to-cyan-400 bg-clip-text text-transparent">action</span>
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-gray-400">
            Everything you need to monitor, query, and bill — in one platform.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-8 lg:grid-cols-5 items-start">
          {/* Feature tabs — left */}
          <div className="lg:col-span-2 flex flex-col gap-2">
            {DEMO_SLIDES.map((s, i) => {
              const TabIcon = s.icon
              const isActive = active === i
              return (
                <button
                  key={s.id}
                  onClick={() => { setActive(i); setPlaying(false) }}
                  className={`group flex items-start gap-3 rounded-2xl border p-4 text-left transition-all duration-300 ${
                    isActive
                      ? `${s.border} ${s.bg} shadow-lg`
                      : 'border-white/5 bg-white/[0.02] hover:bg-white/[0.05] hover:border-white/10'
                  }`}
                >
                  <div className={`flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-br ${s.color} shadow-md transition-transform ${isActive ? 'scale-110' : 'group-hover:scale-105'}`}>
                    <TabIcon className="h-4 w-4 text-white" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className={`text-sm font-bold transition-colors ${isActive ? 'text-white' : 'text-gray-400 group-hover:text-gray-200'}`}>
                      {s.label}
                    </p>
                    {isActive && (
                      <p className="mt-1 text-xs text-gray-400 leading-relaxed">{s.desc}</p>
                    )}
                  </div>
                  {isActive && (
                    <div className={`h-2 w-2 rounded-full bg-gradient-to-br ${s.color} flex-shrink-0 mt-1 animate-pulse`} />
                  )}
                </button>
              )
            })}

            {/* Play/pause */}
            <button
              onClick={() => setPlaying(p => !p)}
              className="mt-2 flex items-center gap-2 self-start rounded-full border border-white/10 bg-white/5 px-4 py-2 text-xs text-gray-400 hover:text-white hover:border-white/20 transition-all"
            >
              {playing
                ? <><Pause className="h-3 w-3" /> Pause tour</>
                : <><Play className="h-3 w-3" /> Auto-play</>
              }
            </button>
          </div>

          {/* Screen preview — right */}
          <div className="lg:col-span-3">
            <div className="relative">
              {/* Glow */}
              <div className={`absolute -inset-4 rounded-3xl bg-gradient-to-br ${slide.color} opacity-10 blur-2xl transition-all duration-700`} />

              {/* Browser chrome */}
              <div className="relative rounded-2xl border border-white/10 bg-[#1a1a2e] shadow-2xl overflow-hidden">
                {/* Browser bar */}
                <div className="flex items-center gap-3 border-b border-white/5 bg-[#0f0f1a] px-4 py-3">
                  <div className="flex gap-1.5">
                    <div className="h-3 w-3 rounded-full bg-red-500/60" />
                    <div className="h-3 w-3 rounded-full bg-yellow-500/60" />
                    <div className="h-3 w-3 rounded-full bg-green-500/60" />
                  </div>
                  <div className="flex-1 rounded-md bg-white/5 border border-white/5 px-3 py-1 text-[10px] text-gray-500">
                    app.datapilot.ai/{slide.id}
                  </div>
                  <div className={`flex h-5 w-5 items-center justify-center rounded-md bg-gradient-to-br ${slide.color}`}>
                    <Icon className="h-3 w-3 text-white" />
                  </div>
                </div>

                {/* Screen content */}
                <div className="h-80 p-3 transition-all duration-500">
                  {slide.screen}
                </div>

                {/* Progress bar */}
                <div className="h-0.5 bg-white/5">
                  <div
                    className={`h-full bg-gradient-to-r ${slide.color} transition-all duration-300`}
                    style={{ width: playing ? '100%' : `${((active + 1) / DEMO_SLIDES.length) * 100}%`, transition: playing ? 'width 4s linear' : 'width 0.3s ease' }}
                  />
                </div>
              </div>

              {/* Slide dots */}
              <div className="mt-4 flex justify-center gap-2">
                {DEMO_SLIDES.map((_, i) => (
                  <button
                    key={i}
                    onClick={() => { setActive(i); setPlaying(false) }}
                    className={`rounded-full transition-all duration-300 ${
                      active === i ? 'w-6 h-2 bg-indigo-500' : 'w-2 h-2 bg-white/20 hover:bg-white/40'
                    }`}
                  />
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

export default function LandingPage() {
  const heroRef = useRef<HTMLDivElement>(null)

  // Parallax on hero text
  useEffect(() => {
    const handler = () => {
      if (heroRef.current) {
        heroRef.current.style.transform = `translateY(${window.scrollY * 0.25}px)`
      }
    }
    window.addEventListener('scroll', handler, { passive: true })
    return () => window.removeEventListener('scroll', handler)
  }, [])

  const featuresSection = useInView()
  const pricingSection = useInView()

  return (
    <div className="min-h-screen bg-[#0a0a0f] text-white">

      {/* ── Navbar ── */}
      <header className="fixed top-0 z-50 w-full border-b border-white/5 bg-[#0a0a0f]/80 backdrop-blur-xl">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
          <div className="flex items-center gap-2">
            <Logo size={30} variant="light" />
          </div>
          <nav className="hidden items-center gap-8 text-sm text-gray-400 md:flex">
            <a href="#demo" className="hover:text-white transition-colors">Demo</a>
            <a href="#features" className="hover:text-white transition-colors">Features</a>
            <a href="#ubb" className="hover:text-white transition-colors">Usage Billing</a>
            <a href="#pricing" className="hover:text-white transition-colors">Pricing</a>
            <a href="#contact" className="hover:text-white transition-colors">Contact</a>
          </nav>
          <div className="flex items-center gap-3">
            <Link to="/login" className="text-sm text-gray-400 hover:text-white transition-colors">Sign In</Link>
            <Link to="/login" className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors">
              Get Started
            </Link>
          </div>
        </div>
      </header>

      {/* ── Hero ── */}
      <section className="relative flex min-h-screen items-center justify-center overflow-hidden px-5 pt-20">
        {/* Animated gradient orbs */}
        <div className="pointer-events-none absolute inset-0">
          <div className="absolute left-1/4 top-1/4 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-indigo-600/20 blur-3xl animate-pulse" />
          <div className="absolute right-1/4 top-1/2 h-80 w-80 rounded-full bg-violet-600/15 blur-3xl animate-pulse" style={{ animationDelay: '1s' }} />
          <div className="absolute bottom-1/4 left-1/2 h-64 w-64 -translate-x-1/2 rounded-full bg-cyan-600/10 blur-3xl animate-pulse" style={{ animationDelay: '2s' }} />
          {/* Grid overlay */}
          <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.02)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.02)_1px,transparent_1px)] bg-[size:64px_64px]" />
        </div>

        <div ref={heroRef} className="relative z-10 mx-auto max-w-4xl text-center">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-indigo-500/30 bg-indigo-500/10 px-4 py-1.5 text-sm text-indigo-300">
            <span className="h-1.5 w-1.5 rounded-full bg-indigo-400 animate-pulse" />
            Now with AI-powered cost recommendations
          </div>

          <h1 className="mb-6 text-5xl font-extrabold leading-tight tracking-tight sm:text-6xl lg:text-7xl">
            Cloud Cost Intelligence
            <br />
            <span className="bg-gradient-to-r from-indigo-400 via-violet-400 to-cyan-400 bg-clip-text text-transparent">
              Powered by AI
            </span>
          </h1>

          <p className="mx-auto mb-10 max-w-2xl text-lg leading-relaxed text-gray-400">
            Unify your multi-cloud spending, detect anomalies automatically, and query your data with natural language. Built for FinOps teams who move fast.
          </p>

          <div className="flex flex-col items-center gap-4 sm:flex-row sm:justify-center">
            <Link
              to="/login"
              className="group flex items-center gap-2 rounded-xl bg-indigo-600 px-7 py-3.5 text-base font-semibold text-white shadow-lg shadow-indigo-600/30 hover:bg-indigo-500 transition-all hover:shadow-indigo-500/40 hover:-translate-y-0.5"
            >
              Get Started Free
              <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-1" />
            </Link>
            <Link
              to="/login"
              className="rounded-xl border border-white/10 px-7 py-3.5 text-base font-medium text-gray-300 hover:border-white/20 hover:text-white transition-all"
            >
              Sign In
            </Link>
          </div>

          <p className="mt-5 text-sm text-gray-500">Free plan available · No credit card required</p>
        </div>

        {/* Scroll indicator */}
        <a href="#features" className="absolute bottom-10 left-1/2 -translate-x-1/2 flex flex-col items-center gap-1 text-gray-600 hover:text-gray-400 transition-colors">
          <span className="text-xs tracking-widest uppercase">Scroll</span>
          <ChevronDown className="h-4 w-4 animate-bounce" />
        </a>
      </section>

      {/* ── Demo / Product Tour ── */}
      <DemoSection />

      {/* ── Stats bar ── */}
      <section className="border-y border-white/5 bg-white/[0.02] py-14">
        <div className="mx-auto grid max-w-4xl grid-cols-2 gap-10 px-5 sm:grid-cols-4">
          {STATS.map((s, i) => <StatCard key={s.label} s={s} i={i} />)}
        </div>
      </section>

      {/* ── Features ── */}
      <section id="features" className="py-24 px-5">
        <div className="mx-auto max-w-6xl">
          <div
            ref={featuresSection.ref}
            className={`mb-16 text-center transition-all duration-700 ${featuresSection.visible ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0'}`}
          >
            <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-indigo-400">Features</p>
            <h2 className="text-4xl font-extrabold tracking-tight">
              Everything you need for{' '}
              <span className="bg-gradient-to-r from-indigo-400 to-violet-400 bg-clip-text text-transparent">FinOps</span>
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-gray-400">
              One platform to monitor, analyse, and optimise your entire cloud footprint.
            </p>
          </div>

          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {FEATURES.map((f, i) => <FeatureCard key={f.title} f={f} i={i} />)}
          </div>

          {/* How it works — horizontal timeline */}
          <div className="mt-20">
            <p className="mb-10 text-center text-sm font-semibold uppercase tracking-widest text-gray-500">How it works</p>
            <div className="relative grid grid-cols-1 gap-8 sm:grid-cols-3">
              {/* connector line */}
              <div className="absolute left-0 right-0 top-6 hidden h-px bg-gradient-to-r from-transparent via-indigo-500/40 to-transparent sm:block" />
              {[
                { step: '01', title: 'Connect your clouds', desc: 'Link AWS, Azure, or GCP with read-only credentials in under 2 minutes.' },
                { step: '02', title: 'Analyse & detect', desc: 'AI scans your spend, flags anomalies, and surfaces optimisation opportunities.' },
                { step: '03', title: 'Query & act', desc: 'Ask questions in plain English. Export reports. Reduce waste.' },
              ].map((item, i) => {
                const { ref, visible } = useInView()
                return (
                  <div
                    key={item.step}
                    ref={ref}
                    style={{ transitionDelay: `${i * 150}ms` }}
                    className={`relative flex flex-col items-center text-center transition-all duration-700 ${visible ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0'}`}
                  >
                    <div className="relative z-10 mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-indigo-500/40 bg-indigo-600/20 text-sm font-bold text-indigo-300">
                      {item.step}
                    </div>
                    <h4 className="mb-2 font-semibold text-white">{item.title}</h4>
                    <p className="text-sm text-gray-400">{item.desc}</p>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </section>

      {/* ── Usage-Based Billing ── */}
      <UBBSection />

      {/* ── Pricing ── */}
      <section id="pricing" className="py-24 px-5">
        <div className="mx-auto max-w-6xl">
          <div
            ref={pricingSection.ref}
            className={`mb-16 text-center transition-all duration-700 ${pricingSection.visible ? 'translate-y-0 opacity-100' : 'translate-y-6 opacity-0'}`}
          >
            <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-indigo-400">Pricing</p>
            <h2 className="text-4xl font-extrabold tracking-tight">Simple, transparent pricing</h2>
            <p className="mt-4 text-gray-400">Free plan forever. Paid plans via Stripe — cancel anytime.</p>
          </div>

          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {PLANS.map((plan, i) => {
              const { ref, visible } = useInView()
              return (
                <div
                  key={plan.name}
                  ref={ref}
                  style={{ transitionDelay: `${i * 80}ms` }}
                  className={`relative flex flex-col overflow-hidden rounded-2xl border bg-white/5 p-6 backdrop-blur transition-all duration-700 hover:-translate-y-1 ${plan.color} ${visible ? 'translate-y-0 opacity-100' : 'translate-y-8 opacity-0'}`}
                >
                  {plan.badge && (
                    <span className="absolute right-4 top-4 rounded-full bg-indigo-600 px-2.5 py-0.5 text-xs font-bold text-white">
                      {plan.badge}
                    </span>
                  )}

                  <p className="text-lg font-bold text-white">{plan.name}</p>
                  <div className="mt-2 flex items-baseline gap-1">
                    <span className="text-4xl font-extrabold text-white">{plan.price}</span>
                    <span className="text-sm text-gray-500">{plan.period}</span>
                  </div>
                  {plan.name === 'Free' && (
                    <span className="mt-1 text-xs text-emerald-400">Free forever</span>
                  )}

                  <ul className="my-6 flex-1 space-y-2.5">
                    {plan.features.map(f => (
                      <li key={f} className="flex items-start gap-2 text-sm text-gray-300">
                        <CheckCircle className="mt-0.5 h-4 w-4 flex-shrink-0 text-indigo-400" />
                        {f}
                      </li>
                    ))}
                  </ul>

                  <Link
                    to="/login"
                    className={`block w-full rounded-xl py-2.5 text-center text-sm font-semibold transition-colors ${plan.btn}`}
                  >
                    {plan.cta}
                  </Link>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      {/* ── CTA banner ── */}
      <section className="relative overflow-hidden py-24 px-5">
        <div className="pointer-events-none absolute inset-0">
          <div className="absolute inset-0 bg-gradient-to-r from-indigo-900/60 via-violet-900/40 to-indigo-900/60" />
          <div className="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-indigo-600/20 blur-3xl" />
        </div>
        <div className="relative mx-auto max-w-2xl text-center">
          <h2 className="text-4xl font-extrabold tracking-tight">Ready to cut cloud waste?</h2>
          <p className="mt-4 text-gray-300">Join hundreds of FinOps teams using DataPilot.AI to gain visibility and reduce spend.</p>
          <Link
            to="/login"
            className="mt-8 inline-flex items-center gap-2 rounded-xl bg-white px-8 py-3.5 text-base font-bold text-indigo-700 shadow-xl hover:bg-indigo-50 transition-all hover:-translate-y-0.5"
          >
            Start for free <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
      </section>

      {/* ── Contact ── */}
      <section id="contact" className="py-20 px-5 border-t border-white/5">
        <div className="mx-auto max-w-4xl">
          <div className="text-center mb-12">
            <p className="mb-3 text-sm font-semibold uppercase tracking-widest text-indigo-400">Contact</p>
            <h2 className="text-3xl font-extrabold tracking-tight">Get in touch</h2>
            <p className="mt-3 text-gray-400">Questions, partnerships, or enterprise inquiries — we're here.</p>
          </div>
          <div className="grid grid-cols-1 gap-5 sm:grid-cols-3">
            {[
              { label: 'General Support', email: 'support@datapilot.co.in', desc: 'Help with your account, billing, or the platform' },
              { label: 'Sales & Enterprise', email: 'sales@datapilot.co.in', desc: 'Custom plans, volume pricing, and enterprise contracts' },
              { label: 'Privacy & Legal', email: 'legal@datapilot.co.in', desc: 'Data requests, privacy concerns, and legal matters' },
            ].map(c => (
              <div key={c.label} className="rounded-2xl border border-white/8 bg-white/[0.03] p-6 hover:border-indigo-500/30 hover:bg-white/[0.05] transition-all">
                <p className="text-xs font-bold uppercase tracking-widest text-indigo-400 mb-2">{c.label}</p>
                <a href={`mailto:${c.email}`} className="text-base font-semibold text-white hover:text-indigo-300 transition-colors">{c.email}</a>
                <p className="mt-2 text-xs text-gray-500">{c.desc}</p>
              </div>
            ))}
          </div>
          <div className="mt-8 rounded-2xl border border-white/5 bg-white/[0.02] p-6 text-center">
            <p className="text-sm text-gray-500">
              <span className="font-semibold text-gray-400">DataPilot.AI</span> · 19/6, 4th Cross Street, Adambakkam, Chennai - 600088, India
              <span className="mx-3 text-gray-700">·</span>
              <a href="tel:+13025550190" className="hover:text-gray-400 transition-colors">+91 9965099462</a>
            </p>
          </div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="border-t border-white/5 py-8 px-5">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 text-sm text-gray-600 sm:flex-row">
          <div className="flex items-center gap-2">
            <div className="flex h-6 w-6 items-center justify-center rounded bg-indigo-600 text-xs font-black text-white">D</div>
            <span>DataPilot.AI</span>
          </div>
          <p>© {new Date().getFullYear()} DataPilot.AI. All rights reserved.</p>
          <div className="flex gap-5">
            <Link to="/terms" className="hover:text-gray-400 transition-colors">Terms</Link>
            <Link to="/privacy" className="hover:text-gray-400 transition-colors">Privacy</Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
