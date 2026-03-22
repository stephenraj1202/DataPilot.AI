import { useState } from 'react'
import {
  LayoutDashboard, Cloud, MessageSquare, CreditCard, Zap, Settings,
  Key, Copy, Check, Terminal, BookOpen,
  BarChart3, Database, Shield, TrendingDown,
  Bot, RefreshCw, DollarSign, Activity, FileText,
} from 'lucide-react'

interface DocSection { id: string; icon: React.ElementType; label: string; color: string; gradient: string }

const SECTIONS: DocSection[] = [
  { id: 'overview',  icon: BookOpen,        label: 'Overview',         color: 'text-indigo-500',  gradient: 'from-indigo-500 to-violet-500' },
  { id: 'dashboard', icon: LayoutDashboard, label: 'Dashboard',        color: 'text-blue-500',    gradient: 'from-blue-500 to-cyan-500' },
  { id: 'finops',    icon: Cloud,           label: 'FinOps Analytics', color: 'text-emerald-500', gradient: 'from-emerald-500 to-teal-500' },
  { id: 'query',     icon: MessageSquare,   label: 'AI Query Tool',    color: 'text-violet-500',  gradient: 'from-violet-500 to-purple-500' },
  { id: 'billing',   icon: CreditCard,      label: 'Billing',          color: 'text-amber-500',   gradient: 'from-amber-500 to-orange-500' },
  { id: 'ubb',       icon: Zap,             label: 'Usage Billing',    color: 'text-pink-500',    gradient: 'from-pink-500 to-rose-500' },
  { id: 'apikeys',   icon: Key,             label: 'API Keys',         color: 'text-cyan-500',    gradient: 'from-cyan-500 to-blue-500' },
  { id: 'settings',  icon: Settings,        label: 'Settings',         color: 'text-gray-500',    gradient: 'from-gray-500 to-slate-500' },
]

function CopyBtn({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 1500) }}
      className="flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-gray-400 hover:bg-gray-700 hover:text-white transition-colors">
      {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

function CodeBlock({ code, lang = 'bash' }: { code: string; lang?: string }) {
  return (
    <div className="rounded-xl overflow-hidden border border-gray-700/50 my-4">
      <div className="flex items-center justify-between bg-gray-800 px-4 py-2">
        <div className="flex items-center gap-2">
          <Terminal className="h-3.5 w-3.5 text-gray-400" />
          <span className="text-xs text-gray-400 font-mono">{lang}</span>
        </div>
        <CopyBtn text={code} />
      </div>
      <pre className="bg-gray-950 px-4 py-4 overflow-x-auto text-xs leading-relaxed text-green-300 font-mono whitespace-pre-wrap"><code>{code}</code></pre>
    </div>
  )
}

function SectionTitle({ icon: Icon, gradient, title, subtitle }: { icon: React.ElementType; gradient: string; title: string; subtitle: string }) {
  return (
    <div className="flex items-start gap-4 mb-8">
      <div className={`flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br ${gradient} shadow-lg`}>
        <Icon className="h-6 w-6 text-white" />
      </div>
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{title}</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
      </div>
    </div>
  )
}

function FeatureCard({ icon: Icon, title, desc, color }: { icon: React.ElementType; title: string; desc: string; color: string }) {
  return (
    <div className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 flex gap-3">
      <div className={`flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-gray-50 dark:bg-gray-700 ${color}`}>
        <Icon className="h-5 w-5" />
      </div>
      <div>
        <p className="text-sm font-semibold text-gray-800 dark:text-gray-200">{title}</p>
        <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{desc}</p>
      </div>
    </div>
  )
}

function InfoBox({ children, color = 'indigo' }: { children: React.ReactNode; color?: string }) {
  const c: Record<string, string> = {
    indigo: 'border-indigo-200 bg-indigo-50 text-indigo-800 dark:border-indigo-800 dark:bg-indigo-900/20 dark:text-indigo-300',
    amber:  'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300',
    emerald:'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300',
  }
  return <div className={`rounded-xl border p-4 text-sm ${c[color]}`}>{children}</div>
}

// ── SVG Illustrations ────────────────────────────────────────────────────────
function DashboardIllustration() {
  return (
    <svg viewBox="0 0 480 220" className="w-full rounded-xl border border-gray-700/40" xmlns="http://www.w3.org/2000/svg">
      <rect width="480" height="220" rx="10" fill="#0f172a"/>
      <rect width="480" height="32" rx="10" fill="#1e293b"/>
      <rect x="10" y="10" width="60" height="12" rx="4" fill="#6366f1"/>
      <circle cx="455" cy="16" r="7" fill="#334155"/><circle cx="438" cy="16" r="7" fill="#334155"/>
      {[0,1,2,3].map(i=>(
        <g key={i}>
          <rect x={10+i*117} y="42" width="109" height="52" rx="8" fill="#1e293b"/>
          <rect x={18+i*117} y="50" width="40" height="5" rx="2" fill="#334155"/>
          <rect x={18+i*117} y="60" width="65" height="10" rx="3" fill={['#6366f1','#10b981','#f59e0b','#ec4899'][i]}/>
          <rect x={18+i*117} y="74" width="30" height="4" rx="2" fill="#334155"/>
        </g>
      ))}
      <rect x="10" y="104" width="295" height="106" rx="8" fill="#1e293b"/>
      <rect x="18" y="112" width="70" height="6" rx="3" fill="#334155"/>
      {[38,62,45,80,55,72,40,68,50,78].map((h,i)=>(
        <rect key={i} x={22+i*27} y={192-h} width="19" height={h} rx="3"
          fill={i===7?'#6366f1':'#334155'} opacity={i===7?1:0.6}/>
      ))}
      <rect x="315" y="104" width="155" height="106" rx="8" fill="#1e293b"/>
      <rect x="323" y="112" width="55" height="6" rx="3" fill="#334155"/>
      {[0,1,2,3].map(i=>(
        <g key={i}>
          <rect x="323" y={128+i*20} width="139" height="12" rx="4" fill="#0f172a"/>
          <rect x="329" y={132+i*20} width={[85,65,95,45][i]} height="4" rx="2"
            fill={['#6366f1','#10b981','#f59e0b','#ec4899'][i]} opacity="0.8"/>
        </g>
      ))}
    </svg>
  )
}

function FinOpsIllustration() {
  return (
    <svg viewBox="0 0 480 220" className="w-full rounded-xl border border-gray-700/40" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient id="gf" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#10b981" stopOpacity="0.4"/>
          <stop offset="100%" stopColor="#10b981" stopOpacity="0"/>
        </linearGradient>
      </defs>
      <rect width="480" height="220" rx="10" fill="#0f172a"/>
      {['AWS','GCP','Azure'].map((n,i)=>(
        <g key={n}>
          <rect x={10+i*100} y="10" width="92" height="32" rx="8" fill="#1e293b"/>
          <rect x={18+i*100} y="18" width="28" height="5" rx="2" fill={['#f59e0b','#4285f4','#0078d4'][i]}/>
          <rect x={18+i*100} y="27" width="50" height="7" rx="3" fill="#334155"/>
        </g>
      ))}
      <rect x="10" y="52" width="310" height="158" rx="8" fill="#1e293b"/>
      <polyline points="28,188 68,162 108,172 148,138 188,148 228,112 268,122 298,96"
        fill="none" stroke="#10b981" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"/>
      <polygon points="28,188 68,162 108,172 148,138 188,148 228,112 268,122 298,96 298,188"
        fill="url(#gf)"/>
      <rect x="330" y="52" width="140" height="158" rx="8" fill="#1e293b"/>
      <rect x="338" y="60" width="65" height="6" rx="3" fill="#334155"/>
      {['EC2','S3','RDS','Lambda','CloudFront'].map((s,i)=>(
        <g key={s}>
          <rect x="338" y={76+i*22} width="124" height="14" rx="4" fill="#0f172a"/>
          <rect x="344" y={80+i*22} width={[90,70,55,40,30][i]} height="6" rx="3" fill="#10b981" opacity="0.7"/>
        </g>
      ))}
    </svg>
  )
}

function QueryIllustration() {
  return (
    <svg viewBox="0 0 480 220" className="w-full rounded-xl border border-gray-700/40" xmlns="http://www.w3.org/2000/svg">
      <rect width="480" height="220" rx="10" fill="#0f172a"/>
      <rect x="10" y="10" width="460" height="32" rx="8" fill="#1e293b"/>
      <rect x="18" y="18" width="180" height="7" rx="3" fill="#334155"/>
      <circle cx="455" cy="26" r="9" fill="#6366f1"/>
      <rect x="120" y="52" width="250" height="36" rx="12" fill="#6366f1"/>
      <rect x="130" y="60" width="190" height="7" rx="3" fill="white" opacity="0.8"/>
      <rect x="130" y="71" width="130" height="5" rx="2" fill="white" opacity="0.5"/>
      <rect x="10" y="98" width="310" height="52" rx="12" fill="#1e293b"/>
      <rect x="20" y="106" width="220" height="6" rx="3" fill="#334155"/>
      <rect x="20" y="116" width="180" height="6" rx="3" fill="#334155"/>
      <rect x="20" y="126" width="140" height="6" rx="3" fill="#334155"/>
      <rect x="10" y="160" width="460" height="50" rx="8" fill="#0d1117"/>
      <rect x="18" y="168" width="28" height="9" rx="3" fill="#6366f1"/>
      <rect x="50" y="168" width="200" height="9" rx="3" fill="#334155"/>
      <rect x="18" y="182" width="280" height="6" rx="3" fill="#22c55e" opacity="0.7"/>
      <rect x="18" y="192" width="240" height="6" rx="3" fill="#60a5fa" opacity="0.7"/>
      <rect x="330" y="98" width="140" height="52" rx="8" fill="#1e293b"/>
      {[18,30,22,40,28,45,35].map((h,i)=>(
        <rect key={i} x={338+i*17} y={140-h} width="11" height={h} rx="2" fill="#6366f1" opacity={0.4+i*0.08}/>
      ))}
    </svg>
  )
}

function BillingIllustration() {
  return (
    <svg viewBox="0 0 480 220" className="w-full rounded-xl border border-gray-700/40" xmlns="http://www.w3.org/2000/svg">
      <rect width="480" height="220" rx="10" fill="#0f172a"/>
      {['Free','Base','Pro','Enterprise'].map((p,i)=>(
        <g key={p}>
          <rect x={10+i*117} y="10" width="109" height="90" rx="10"
            fill={i===2?'#1e1b4b':'#1e293b'} stroke={i===2?'#6366f1':'transparent'} strokeWidth="1.5"/>
          <rect x={18+i*117} y="20" width="50" height="7" rx="3" fill={i===2?'#6366f1':'#334155'}/>
          <rect x={18+i*117} y="32" width="65" height="12" rx="4" fill="#334155"/>
          {[0,1,2].map(j=>(
            <g key={j}>
              <circle cx={22+i*117} cy={56+j*12} r="3" fill={i===2?'#6366f1':'#334155'}/>
              <rect x={28+i*117} y={52+j*12} width={[48,58,68,52][i]} height="5" rx="2" fill="#334155"/>
            </g>
          ))}
          {i===2&&<rect x={18+i*117} y="86" width="75" height="8" rx="4" fill="#6366f1"/>}
        </g>
      ))}
      <rect x="10" y="112" width="460" height="98" rx="10" fill="#1e293b"/>
      <rect x="18" y="120" width="75" height="7" rx="3" fill="#334155"/>
      <rect x="400" y="118" width="62" height="12" rx="6" fill="#10b981" opacity="0.8"/>
      {['Jan 2025','Feb 2025','Mar 2025','Apr 2025'].map((m,i)=>(
        <g key={m}>
          <rect x="18" y={140+i*18} width="444" height="12" rx="4" fill="#0f172a"/>
          <rect x="26" y={144+i*18} width="55" height="4" rx="2" fill="#334155"/>
          <rect x="200" y={144+i*18} width="38" height="4" rx="2" fill="#334155"/>
          <rect x="390" y={144+i*18} width="48" height="4" rx="2" fill={i===3?'#10b981':'#334155'}/>
        </g>
      ))}
    </svg>
  )
}

function UBBIllustration() {
  return (
    <svg viewBox="0 0 480 220" className="w-full rounded-xl border border-gray-700/40" xmlns="http://www.w3.org/2000/svg">
      <rect width="480" height="220" rx="10" fill="#0f172a"/>
      {['api-calls','db-queries','exports'].map((n,i)=>(
        <g key={n}>
          <rect x={10+i*157} y="10" width="149" height="72" rx="10" fill="#1e293b"/>
          <rect x={18+i*157} y="18" width="75" height="7" rx="3" fill="#ec4899" opacity="0.8"/>
          <rect x={18+i*157} y="30" width="48" height="12" rx="4" fill="#334155"/>
          <rect x={18+i*157} y="46" width="115" height="5" rx="2" fill="#334155"/>
          <polyline
            points={`${18+i*157},72 ${38+i*157},64 ${58+i*157},68 ${78+i*157},56 ${98+i*157},60 ${118+i*157},50 ${138+i*157},54`}
            fill="none" stroke="#ec4899" strokeWidth="1.5" opacity="0.7"/>
        </g>
      ))}
      <rect x="10" y="92" width="290" height="118" rx="10" fill="#0d1117"/>
      <rect x="18" y="100" width="38" height="9" rx="3" fill="#ec4899"/>
      <rect x="62" y="100" width="180" height="9" rx="3" fill="#334155"/>
      {[0,1,2,3,4].map(i=>(
        <rect key={i} x="18" y={116+i*16} width={[260,220,240,200,180][i]} height="6" rx="3"
          fill={['#22c55e','#60a5fa','#f59e0b','#22c55e','#60a5fa'][i]} opacity="0.6"/>
      ))}
      <rect x="312" y="92" width="158" height="118" rx="10" fill="#1e293b"/>
      <rect x="320" y="100" width="75" height="7" rx="3" fill="#334155"/>
      {['Flat fee','api-calls','db-queries','exports','Total'].map((l,i)=>(
        <g key={l}>
          <rect x="320" y={116+i*18} width="142" height="12" rx="4" fill="#0f172a"/>
          <rect x="326" y={120+i*18} width="58" height="4" rx="2" fill="#334155"/>
          <rect x="404" y={120+i*18} width="48" height="4" rx="2"
            fill={i===4?'#ec4899':'#334155'} opacity={i===4?1:0.7}/>
        </g>
      ))}
    </svg>
  )
}

function APIKeyIllustration() {
  return (
    <svg viewBox="0 0 480 180" className="w-full rounded-xl border border-gray-700/40" xmlns="http://www.w3.org/2000/svg">
      <rect width="480" height="180" rx="10" fill="#0f172a"/>
      <rect x="10" y="10" width="460" height="52" rx="10" fill="#1e293b"/>
      <rect x="18" y="18" width="75" height="7" rx="3" fill="#334155"/>
      <rect x="18" y="30" width="310" height="12" rx="6" fill="#0d1117"/>
      <rect x="26" y="34" width="200" height="4" rx="2" fill="#6366f1" opacity="0.6"/>
      <rect x="380" y="28" width="40" height="16" rx="8" fill="#6366f1"/>
      <rect x="428" y="28" width="34" height="16" rx="8" fill="#ef4444" opacity="0.8"/>
      <rect x="10" y="72" width="460" height="98" rx="10" fill="#0d1117"/>
      <rect x="18" y="80" width="95" height="7" rx="3" fill="#334155"/>
      {[0,1,2,3,4,5].map(i=>(
        <rect key={i} x="18" y={94+i*12} width={[440,380,320,360,280,200][i]} height="6" rx="3"
          fill={['#60a5fa','#22c55e','#f59e0b','#60a5fa','#22c55e','#ec4899'][i]} opacity="0.5"/>
      ))}
    </svg>
  )
}

// ── Section content ──────────────────────────────────────────────────────────
const BASE = typeof window !== 'undefined'
  ? window.location.origin.replace('3000','8080').replace('5173','8080')
  : 'http://localhost:8080'

function OverviewSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={BookOpen} gradient="from-indigo-500 to-violet-500" title="DataPilot.AI" subtitle="Complete platform documentation — everything you need to get started." />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={LayoutDashboard} color="text-blue-500"   title="Dashboard"        desc="Real-time KPIs, cost trends, and usage metrics at a glance." />
        <FeatureCard icon={Cloud}           color="text-emerald-500" title="FinOps Analytics" desc="Multi-cloud cost visibility with AI-powered recommendations." />
        <FeatureCard icon={MessageSquare}   color="text-violet-500" title="AI Query Tool"    desc="Ask questions in plain English, get SQL + charts instantly." />
        <FeatureCard icon={CreditCard}      color="text-amber-500"  title="Billing"          desc="Subscription plans, invoices, and secure checkout." />
        <FeatureCard icon={Zap}             color="text-pink-500"   title="Usage Billing"    desc="Per-unit metered billing streams with live invoice preview." />
        <FeatureCard icon={Key}             color="text-cyan-500"   title="API Keys"         desc="Secure API key management for external integrations." />
      </div>
      <InfoBox color="indigo">
        <strong>Base URL:</strong> <code className="ml-1 rounded bg-indigo-100 dark:bg-indigo-900/40 px-1.5 py-0.5 text-xs font-mono">{BASE}</code>
        <br/><span className="text-xs mt-1 block">All protected endpoints require either a Bearer token or <code>X-API-Key</code> header.</span>
      </InfoBox>
      <CodeBlock lang="bash" code={`# Authenticate with API key
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/finops/cloud-accounts

# Or with Bearer token
curl -H "Authorization: Bearer YOUR_JWT" ${BASE}/api/billing/subscription`} />
    </div>
  )
}

function DashboardSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={LayoutDashboard} gradient="from-blue-500 to-cyan-500" title="Dashboard" subtitle="Your command center — live KPIs, cost trends, and usage at a glance." />
      <DashboardIllustration />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={Activity}    color="text-blue-500"    title="Live KPI Cards"    desc="Total cloud spend, active streams, query count, and plan status update in real time." />
        <FeatureCard icon={BarChart3}   color="text-indigo-500"  title="Cost Trend Chart"  desc="30-day rolling cost chart with per-provider breakdown." />
        <FeatureCard icon={TrendingDown} color="text-emerald-500" title="Top Services"      desc="Ranked list of your most expensive cloud services this month." />
        <FeatureCard icon={RefreshCw}   color="text-amber-500"   title="Auto Refresh"      desc="Data refreshes every 60 seconds — no manual reload needed." />
      </div>
      <InfoBox color="emerald">The dashboard aggregates data from all connected cloud accounts. Add accounts in FinOps → Cloud Accounts to see data here.</InfoBox>
    </div>
  )
}

function FinOpsSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={Cloud} gradient="from-emerald-500 to-teal-500" title="FinOps Analytics" subtitle="Multi-cloud cost visibility, recommendations, and scheduled reports." />
      <FinOpsIllustration />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={Cloud}       color="text-emerald-500" title="Cloud Accounts"     desc="Connect AWS, GCP, and Azure accounts with read-only credentials." />
        <FeatureCard icon={BarChart3}   color="text-blue-500"    title="Cost Breakdown"     desc="Filter by provider, service, region, and date range." />
        <FeatureCard icon={TrendingDown} color="text-amber-500"  title="AI Recommendations" desc="Automated cost-saving suggestions powered by usage analysis." />
        <FeatureCard icon={RefreshCw}   color="text-violet-500"  title="Report Schedules"   desc="Schedule daily/weekly/monthly cost reports via email." />
      </div>
      <CodeBlock lang="bash" code={`# List cloud accounts
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/finops/cloud-accounts

# Get cost summary (date range)
curl -H "X-API-Key: YOUR_KEY" \\
  "${BASE}/api/finops/costs?start_date=2025-01-01&end_date=2025-01-31"

# Get AI recommendations
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/finops/recommendations`} />
      <InfoBox color="amber">Enable Demo Mode (toggle in the toolbar) to explore the FinOps dashboard with sample data before connecting real accounts.</InfoBox>
    </div>
  )
}

function QuerySection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={MessageSquare} gradient="from-violet-500 to-purple-500" title="AI Query Tool" subtitle="Natural language → SQL → charts. Ask anything about your databases." />
      <QueryIllustration />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={Database}    color="text-violet-500"  title="DB Connections"    desc="Connect PostgreSQL, MySQL, MongoDB, SQL Server via the wizard." />
        <FeatureCard icon={Bot}         color="text-indigo-500"  title="AI SQL Generation" desc="Describe what you want — the AI writes the SQL and runs it." />
        <FeatureCard icon={BarChart3}   color="text-blue-500"    title="Auto Charts"       desc="Results are automatically visualized as bar, line, or pie charts." />
        <FeatureCard icon={Shield}      color="text-emerald-500" title="Bookmarks"         desc="Save useful queries with a name and re-run them anytime." />
      </div>
      <CodeBlock lang="bash" code={`# Ask a natural language question
curl -X POST -H "X-API-Key: YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"question":"Top 5 most expensive services this month","connection_id":"CONN_ID"}' \\
  ${BASE}/api/query/ask

# List saved connections
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/query/connections

# List bookmarks
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/query/bookmarks`} />
    </div>
  )
}

function BillingSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={CreditCard} gradient="from-amber-500 to-orange-500" title="Billing" subtitle="Subscription plans, secure checkout, invoices, and plan limits." />
      <BillingIllustration />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={CreditCard}  color="text-amber-500"   title="Plans"             desc="Free, Base, Pro, Enterprise — upgrade or downgrade anytime." />
        <FeatureCard icon={DollarSign}  color="text-emerald-500" title="Secure Checkout"   desc="Secure hosted checkout. Card details never touch our servers." />
        <FeatureCard icon={FileText}    color="text-blue-500"    title="Invoices"          desc="Download PDF invoices for every billing period." />
        <FeatureCard icon={Shield}      color="text-violet-500"  title="Plan Limits"       desc="API rate limits and feature gates enforced per plan." />
      </div>
      <CodeBlock lang="bash" code={`# Get current subscription
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/billing/subscription

# List invoices
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/billing/invoices

# Get plan limits
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/billing/plan-limits

# Create checkout session
curl -X POST -H "X-API-Key: YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"plan_id":"pro"}' \\
  ${BASE}/api/billing/checkout`} />
    </div>
  )
}

function UBBSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={Zap} gradient="from-pink-500 to-rose-500" title="Usage-Based Billing" subtitle="Create metered billing streams. Charge per unit — no free-tier complexity." />
      <UBBIllustration />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={Zap}         color="text-pink-500"    title="Streams"           desc="Each stream tracks one billable metric (API calls, exports, queries…)." />
        <FeatureCard icon={Activity}    color="text-rose-500"    title="Post Usage"        desc="Send usage events via API. Idempotency keys prevent double-billing." />
        <FeatureCard icon={DollarSign}  color="text-amber-500"   title="Invoice Preview"   desc="See exactly what will be billed before the period closes." />
        <FeatureCard icon={RefreshCw}   color="text-violet-500"  title="Dry Run"           desc="Simulate the invoice including deleted stream charges." />
      </div>
      <CodeBlock lang="bash" code={`# Create a billing stream
curl -X POST -H "X-API-Key: YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"stream_name":"api-calls","resolver_id":"my-app","overage_price_cents":10}' \\
  ${BASE}/api/billing/ubb/streams

# Post usage (idempotent)
curl -X POST -H "X-API-Key: YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"quantity":150,"idempotency_key":"evt_abc123"}' \\
  ${BASE}/api/billing/ubb/streams/STREAM_ID/usage

# Preview next invoice
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/billing/ubb/invoice/preview

# Dry run (includes deleted streams)
curl -H "X-API-Key: YOUR_KEY" ${BASE}/api/billing/ubb/invoice/dryrun`} />
      <InfoBox color="amber">Each unit posted is billed at the stream's <code>overage_price_cents</code> (gateway streams use <code>sub_item_price_cents</code>). There is no free-tier deduction.</InfoBox>
    </div>
  )
}

function APIKeysSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={Key} gradient="from-cyan-500 to-blue-500" title="API Keys" subtitle="Generate and manage API keys for external integrations and SDK usage." />
      <APIKeyIllustration />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={Key}         color="text-cyan-500"    title="Generate Keys"     desc="Create named keys with 1-year expiry. Shown once on creation." />
        <FeatureCard icon={Shield}      color="text-emerald-500" title="SHA-256 Hashed"    desc="Only the hash is stored — the plaintext key is never persisted." />
        <FeatureCard icon={Activity}    color="text-blue-500"    title="Last Used"         desc="Track when each key was last used for security auditing." />
        <FeatureCard icon={RefreshCw}   color="text-rose-500"    title="Revoke Anytime"    desc="Instantly revoke any key — it stops working immediately." />
      </div>
      <CodeBlock lang="bash" code={`# List your API keys
curl -H "Authorization: Bearer YOUR_JWT" ${BASE}/api/auth/api-keys

# Create a new key
curl -X POST -H "Authorization: Bearer YOUR_JWT" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"production-app"}' \\
  ${BASE}/api/auth/api-keys

# Revoke a key
curl -X DELETE -H "Authorization: Bearer YOUR_JWT" \\
  ${BASE}/api/auth/api-keys/KEY_ID`} />
      <InfoBox color="indigo">Pass the key as <code>X-API-Key: YOUR_KEY</code> on all API requests. You can also generate and manage keys directly from the user menu in the top navbar.</InfoBox>
    </div>
  )
}

function SettingsSection() {
  return (
    <div className="space-y-6">
      <SectionTitle icon={Settings} gradient="from-gray-500 to-slate-500" title="Settings" subtitle="Profile, team, SMTP, database connections, notifications, and branding." />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <FeatureCard icon={Shield}      color="text-indigo-500"  title="Profile"           desc="View your email, role, and account details." />
        <FeatureCard icon={Database}    color="text-violet-500"  title="DB Connections"    desc="Add and manage database connections for the AI Query Tool." />
        <FeatureCard icon={RefreshCw}   color="text-blue-500"    title="SMTP Config"       desc="Configure a custom SMTP server for outbound emails." />
        <FeatureCard icon={BarChart3}   color="text-emerald-500" title="Branding"          desc="Upload your logo and set a custom app name and tagline." />
      </div>
      <InfoBox color="emerald">Database connections added in Settings → Database Connections are available in the AI Query Tool for natural language queries.</InfoBox>
    </div>
  )
}

// ── Main page ────────────────────────────────────────────────────────────────
const CONTENT: Record<string, React.ReactNode> = {
  overview:  <OverviewSection />,
  dashboard: <DashboardSection />,
  finops:    <FinOpsSection />,
  query:     <QuerySection />,
  billing:   <BillingSection />,
  ubb:       <UBBSection />,
  apikeys:   <APIKeysSection />,
  settings:  <SettingsSection />,
}

export default function DocsPage() {
  const [active, setActive] = useState('overview')

  return (
    <div className="flex min-h-0 h-[calc(100vh-4rem)] -m-4 md:-m-6">

      {/* ── Docs sidebar ── */}
      <aside className="hidden lg:flex w-56 flex-shrink-0 flex-col border-r border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-y-auto">
        <div className="px-4 py-5 border-b border-gray-100 dark:border-gray-700">
          <div className="flex items-center gap-2">
            <BookOpen className="h-5 w-5 text-indigo-500" />
            <span className="text-sm font-bold text-gray-900 dark:text-white">Documentation</span>
          </div>
          <p className="mt-1 text-xs text-gray-400">Platform guide &amp; API reference</p>
        </div>
        <nav className="flex-1 p-3 space-y-0.5">
          {SECTIONS.map(s => {
            const Icon = s.icon
            const isActive = active === s.id
            return (
              <button
                key={s.id}
                onClick={() => setActive(s.id)}
                className={`w-full flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium text-left transition-colors ${
                  isActive
                    ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400'
                    : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700'
                }`}
              >
                <Icon className={`h-4 w-4 flex-shrink-0 ${isActive ? 'text-indigo-500' : s.color}`} />
                {s.label}
              </button>
            )
          })}
        </nav>
      </aside>

      {/* ── Content ── */}
      <div className="flex flex-1 flex-col min-w-0 overflow-hidden">
        {/* Mobile pill tabs */}
        <div className="lg:hidden flex gap-2 overflow-x-auto px-4 py-2 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 flex-shrink-0">
          {SECTIONS.map(s => {
            const Icon = s.icon
            return (
              <button key={s.id} onClick={() => setActive(s.id)}
                className={`flex-shrink-0 flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-semibold transition-colors ${
                  active === s.id ? 'bg-indigo-500 text-white' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
                }`}>
                <Icon className="h-3.5 w-3.5" />{s.label}
              </button>
            )
          })}
        </div>

        <main className="flex-1 overflow-y-auto p-6 lg:p-8 bg-gray-50 dark:bg-gray-900">
          <div className="max-w-3xl mx-auto">
            {CONTENT[active]}
          </div>
        </main>
      </div>
    </div>
  )
}
