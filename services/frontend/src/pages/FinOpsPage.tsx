import { useState, useRef, useEffect, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus, RefreshCw, Trash2, Pencil, TrendingUp, DollarSign,
  Zap, AlertTriangle, Mail, Activity, Cloud, BarChart2, Layers, Server, Settings, X,
} from 'lucide-react'
import {
  AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
  RadialBarChart, RadialBar,
} from 'recharts'
import { finopsService, type CloudAccount, type ResourceTile } from '../services/finops.service'
import AnomalyAlert from '../components/finops/AnomalyAlert'
import RecommendationCard from '../components/finops/RecommendationCard'
import CloudAccountModal from '../components/finops/CloudAccountModal'
import ReportScheduleModal from '../components/finops/ReportScheduleModal'
import LoadingSpinner from '../components/common/LoadingSpinner'
import ConfirmModal from '../components/common/ConfirmModal'
import { formatCurrency } from '../utils/formatters'
import toast from 'react-hot-toast'

function getDateRange() {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth(), 1)
  return { startDate: start.toISOString().split('T')[0], endDate: end.toISOString().split('T')[0] }
}

const PROVIDER_LABELS: Record<string, string> = { aws: 'AWS', azure: 'Azure', gcp: 'GCP' }
const PROVIDER_ICON: Record<string, string> = { aws: 'AWS', azure: 'AZ', gcp: 'GCP' }
const VIVID = ['#6366f1','#f59e0b','#10b981','#ef4444','#3b82f6','#8b5cf6','#ec4899','#14b8a6','#f97316','#84cc16','#06b6d4','#a855f7']
const PROVIDER_CONFIG: Record<string, { from: string; to: string; glow: string; icon: string }> = {
  aws:   { from: '#f97316', to: '#fbbf24', glow: 'rgba(249,115,22,0.35)', icon: '☁' },
  azure: { from: '#3b82f6', to: '#06b6d4', glow: 'rgba(59,130,246,0.35)', icon: '⬡' },
  gcp:   { from: '#10b981', to: '#34d399', glow: 'rgba(16,185,129,0.35)', icon: '◈' },
}

// Portlet wrapper — uniform card shell with minimize/maximize
function Portlet({ title, sub, icon: Icon, iconBg, children, className = '', action }: {
  title: string; sub?: string; icon: React.ElementType; iconBg: string; children: React.ReactNode; className?: string; action?: React.ReactNode
}) {
  const [minimized, setMinimized] = useState(false)
  const [maximized, setMaximized] = useState(false)

  return (
    <div className={`flex flex-col rounded-xl border border-gray-200/60 bg-white shadow-sm dark:border-gray-700/60 dark:bg-gray-800/80 transition-all duration-200 ${maximized ? 'fixed inset-4 z-40 shadow-2xl overflow-auto' : ''} ${className}`}>
      {/* Header */}
      <div className="flex items-center gap-2.5 border-b border-gray-100 px-4 py-2.5 dark:border-gray-700/60">
        <div className={`flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg ${iconBg}`}>
          <Icon className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-bold text-gray-800 dark:text-white leading-none">{title}</p>
          {sub && <p className="mt-0.5 text-[11px] text-gray-400 leading-none truncate">{sub}</p>}
        </div>
        {/* Controls */}
        <div className="flex items-center gap-1 flex-shrink-0">
          {action && <div className="mr-1">{action}</div>}
          <button
            onClick={() => { setMinimized(m => !m); if (maximized) setMaximized(false) }}
            className="flex h-5 w-5 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300 transition-colors"
            title={minimized ? 'Expand' : 'Minimize'}
          >
            <span className="text-[10px] font-black leading-none">{minimized ? '▲' : '▼'}</span>
          </button>
          <button
            onClick={() => { setMaximized(m => !m); if (minimized) setMinimized(false) }}
            className="flex h-5 w-5 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300 transition-colors"
            title={maximized ? 'Restore' : 'Maximize'}
          >
            <span className="text-[10px] font-black leading-none">{maximized ? '⊡' : '⊞'}</span>
          </button>
        </div>
      </div>
      {/* Body */}
      {!minimized && (
        <div className="flex-1 p-4">{children}</div>
      )}
      {/* Maximized backdrop */}
      {maximized && <div className="fixed inset-0 -z-10 bg-black/40 backdrop-blur-sm" onClick={() => setMaximized(false)} />}
    </div>
  )
}

// Animated counter
function useCountUp(target: number, duration = 1000) {
  const [val, setVal] = useState(0)
  const raf = useRef<number>(0)
  useEffect(() => {
    if (target === 0) { setVal(0); return }
    const start = performance.now()
    const tick = (now: number) => {
      const p = Math.min((now - start) / duration, 1)
      const ease = 1 - Math.pow(1 - p, 3)
      setVal(target * ease)
      if (p < 1) raf.current = requestAnimationFrame(tick)
    }
    raf.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf.current)
  }, [target, duration])
  return val
}

// Sparkline
function Sparkline({ data, color }: { data: number[]; color: string }) {
  if (data.length < 2) return null
  const max = Math.max(...data, 1)
  const w = 64, h = 24, pad = 2
  const pts = data.map((v, i) => {
    const x = pad + (i / (data.length - 1)) * (w - pad * 2)
    const y = h - pad - (v / max) * (h - pad * 2)
    return `${x},${y}`
  }).join(' ')
  return (
    <svg width={w} height={h}>
      <polyline points={pts} fill="none" stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.8" />
    </svg>
  )
}

// KPI tile — compact uniform height
function KpiTile({ label, value, prefix = '$', icon: Icon, from, to, glow, sparkData, animate = true }: {
  label: string; value: number; prefix?: string; icon: React.ElementType
  from: string; to: string; glow: string; sparkData?: number[]; animate?: boolean
}) {
  const animated = useCountUp(animate ? value : 0)
  const display = animate ? animated : value
  const isCurrency = prefix === '$'
  return (
    <div
      className="relative flex h-[100px] flex-col justify-between overflow-hidden rounded-xl p-3.5 text-white shadow-md transition-transform hover:scale-[1.02]"
      style={{ background: `linear-gradient(135deg, ${from}, ${to})`, boxShadow: `0 4px 20px ${glow}` }}
    >
      <div className="pointer-events-none absolute -right-3 -top-3 h-14 w-14 rounded-full opacity-20" style={{ background: to }} />
      <div className="flex items-center justify-between">
        <p className="text-[10px] font-bold uppercase tracking-widest opacity-75">{label}</p>
        <div className="flex h-6 w-6 items-center justify-center rounded-md bg-white/20">
          <Icon className="h-3 w-3" />
        </div>
      </div>
      <div className="flex items-end justify-between">
        <p className="text-xl font-black tabular-nums leading-none">
          {prefix}{display.toLocaleString('en-US', { minimumFractionDigits: isCurrency ? 2 : 0, maximumFractionDigits: isCurrency ? 2 : 0 })}
        </p>
        {sparkData && <Sparkline data={sparkData} color="rgba(255,255,255,0.8)" />}
      </div>
    </div>
  )
}

// Provider cost tile — same height as KPI
function ProviderTile({ provider, cost, pct }: { provider: string; cost: number; pct: number }) {
  const cfg = PROVIDER_CONFIG[provider] ?? { from: '#6366f1', to: '#8b5cf6', glow: 'rgba(99,102,241,0.35)', icon: '◉' }
  const animated = useCountUp(cost)
  return (
    <div
      className="relative flex h-[100px] flex-col justify-between overflow-hidden rounded-xl p-3.5 text-white shadow-md transition-transform hover:scale-[1.02]"
      style={{ background: `linear-gradient(135deg, ${cfg.from}, ${cfg.to})`, boxShadow: `0 4px 20px ${cfg.glow}` }}
    >
      <div className="pointer-events-none absolute -right-3 -top-3 h-14 w-14 rounded-full opacity-20" style={{ background: cfg.to }} />
      <div className="flex items-center justify-between">
        <p className="text-[10px] font-bold uppercase tracking-widest opacity-75">{PROVIDER_LABELS[provider] ?? provider}</p>
        <span className="text-base font-black opacity-80">{cfg.icon}</span>
      </div>
      <div>
        <p className="text-xl font-black tabular-nums leading-none">${animated.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</p>
        <div className="mt-1.5 h-1 w-full overflow-hidden rounded-full bg-white/25">
          <div className="h-1 rounded-full bg-white transition-all duration-1000" style={{ width: `${Math.min(pct, 100)}%` }} />
        </div>
        <p className="mt-0.5 text-[10px] opacity-60">{pct.toFixed(1)}% of total</p>
      </div>
    </div>
  )
}

// Currency tooltip
function CurrencyTooltip({ active, payload, label }: { active?: boolean; payload?: Array<{ name: string; value: number; color: string }>; label?: string }) {
  if (!active || !payload?.length) return null
  const fmt = label ? (() => { const d = new Date(label); return isNaN(d.getTime()) ? label : d.toLocaleDateString('default', { month: 'short', day: '2-digit', year: 'numeric' }) })() : undefined
  return (
    <div className="rounded-xl border border-white/10 bg-gray-900/95 px-3 py-2.5 shadow-2xl">
      {fmt && <p className="mb-1.5 text-[10px] font-bold uppercase tracking-wide text-gray-400">{fmt}</p>}
      {payload.map((p, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <span className="h-2 w-2 rounded-full flex-shrink-0" style={{ background: p.color }} />
          <span className="text-[11px] text-gray-400">{p.name}:</span>
          <span className="text-xs font-bold text-white">{formatCurrency(p.value)}</span>
        </div>
      ))}
    </div>
  )
}

// Daily area chart portlet
function DailyAreaChart({ areaData, byProvider }: { areaData: Record<string, unknown>[]; byProvider: Record<string, number> }) {
  const providers = Object.keys(byProvider)
  return (
    <ResponsiveContainer width="100%" height={220}>
      <AreaChart data={areaData} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
        <defs>
          {providers.map(p => {
            const c = PROVIDER_CONFIG[p]?.from ?? '#6366f1'
            return (
              <linearGradient key={p} id={`ag-${p}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor={c} stopOpacity={0.35} />
                <stop offset="95%" stopColor={c} stopOpacity={0.02} />
              </linearGradient>
            )
          })}
        </defs>
        <CartesianGrid strokeDasharray="3 3" opacity={0.07} />
        <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#9ca3af' }} axisLine={false} tickLine={false}
          interval={Math.max(0, Math.floor(areaData.length / 7) - 1)}
          tickFormatter={(v: string) => { const d = new Date(v); return isNaN(d.getTime()) ? v : `${d.toLocaleString('default', { month: 'short' })} ${String(d.getDate()).padStart(2, '0')}` }} />
        <YAxis tick={{ fontSize: 10, fill: '#9ca3af' }} tickFormatter={v => v >= 1000 ? `${(v/1000).toFixed(1)}k` : `${v.toFixed(0)}`} axisLine={false} tickLine={false} width={44} />
        <Tooltip content={<CurrencyTooltip />} />
        <Legend iconType="circle" iconSize={7} wrapperStyle={{ fontSize: 11, paddingTop: 8 }} />
        {providers.map(p => {
          const c = PROVIDER_CONFIG[p]?.from ?? '#6366f1'
          return <Area key={p} type="monotone" dataKey={p} name={PROVIDER_LABELS[p] ?? p} stroke={c} fill={`url(#ag-${p})`} strokeWidth={2} dot={false} activeDot={{ r: 4, strokeWidth: 0, fill: c }} animationDuration={700} />
        })}
      </AreaChart>
    </ResponsiveContainer>
  )
}

// Provider donut portlet
function ProviderDonut({ data }: { data: Array<{ name: string; value: number }> }) {
  const total = data.reduce((s, d) => s + d.value, 0)
  const animTotal = useCountUp(total)
  return (
    <div className="flex flex-col items-center">
      <p className="mb-1 text-xs text-gray-400">Total: <span className="font-bold text-gray-700 dark:text-gray-200">${animTotal.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span></p>
      <ResponsiveContainer width="100%" height={160}>
        <PieChart>
          <defs>
            {data.map((_, i) => (
              <radialGradient key={i} id={`dg-${i}`} cx="50%" cy="50%" r="50%">
                <stop offset="0%" stopColor={VIVID[i % VIVID.length]} stopOpacity="1" />
                <stop offset="100%" stopColor={VIVID[i % VIVID.length]} stopOpacity="0.7" />
              </radialGradient>
            ))}
          </defs>
          <Pie data={data} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={65} innerRadius={38} paddingAngle={3} animationDuration={700} label={false}>
            {data.map((_, i) => <Cell key={i} fill={`url(#dg-${i})`} stroke="none" />)}
          </Pie>
          <Tooltip formatter={(v: number) => formatCurrency(v)} contentStyle={{ borderRadius: 12, border: 'none', background: 'rgba(17,24,39,0.95)', color: '#fff' }} itemStyle={{ color: '#e5e7eb' }} />
        </PieChart>
      </ResponsiveContainer>
      <div className="mt-1 flex flex-wrap justify-center gap-x-3 gap-y-1">
        {data.map((d, i) => (
          <div key={i} className="flex items-center gap-1">
            <span className="h-2 w-2 rounded-full flex-shrink-0" style={{ background: VIVID[i % VIVID.length] }} />
            <span className="text-[11px] text-gray-500 dark:text-gray-400">{d.name}</span>
            <span className="text-[11px] font-bold text-gray-800 dark:text-white">{formatCurrency(d.value)}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// Budget gauge portlet
function BudgetGauge({ spent, forecast }: { spent: number; forecast: number }) {
  const pct = forecast > 0 ? Math.min((spent / forecast) * 100, 100) : 0
  const color = pct > 90 ? '#ef4444' : pct > 70 ? '#f59e0b' : '#10b981'
  const data = [{ name: 'used', value: pct, fill: color }, { name: 'rem', value: 100 - pct, fill: 'transparent' }]
  return (
    <div className="flex flex-col items-center">
      <div className="relative">
        <ResponsiveContainer width={140} height={140}>
          <RadialBarChart cx="50%" cy="50%" innerRadius="58%" outerRadius="88%" startAngle={210} endAngle={-30} data={data} barSize={12}>
            <RadialBar dataKey="value" cornerRadius={6} background={{ fill: '#f1f5f9' }} />
          </RadialBarChart>
        </ResponsiveContainer>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-xl font-black" style={{ color }}>{pct.toFixed(0)}%</span>
          <span className="text-[10px] text-gray-400">used</span>
        </div>
      </div>
      <div className="mt-1 flex gap-5 text-center">
        <div><p className="text-[10px] text-gray-400">Spent</p><p className="text-xs font-bold text-gray-800 dark:text-white">{formatCurrency(spent)}</p></div>
        <div><p className="text-[10px] text-gray-400">Forecast</p><p className="text-xs font-bold text-gray-800 dark:text-white">{formatCurrency(forecast)}</p></div>
      </div>
    </div>
  )
}

// Top services bar portlet
function TopServicesChart({ services }: { services: Array<{ service: string; provider: string; cost: number }> }) {
  const top = services.slice(0, 8)
  const data = top.map((s, i) => ({
    name: s.service.length > 22 ? s.service.slice(0, 20) + '..' : s.service,
    fullName: s.service, cost: s.cost, provider: s.provider, fill: VIVID[i % VIVID.length],
  }))
  return (
    <ResponsiveContainer width="100%" height={240}>
      <BarChart data={data} layout="vertical" margin={{ top: 0, right: 60, left: 4, bottom: 0 }} barSize={12}>
        <CartesianGrid strokeDasharray="3 3" horizontal={false} opacity={0.07} />
        <XAxis type="number" tick={{ fontSize: 10, fill: '#9ca3af' }} tickFormatter={v => v >= 1000 ? `${(v/1000).toFixed(1)}k` : `${v.toFixed(0)}`} axisLine={false} tickLine={false} />
        <YAxis type="category" dataKey="name" width={130} tick={{ fontSize: 10, fill: '#6b7280' }} axisLine={false} tickLine={false} />
        <Tooltip cursor={{ fill: 'rgba(99,102,241,0.05)' }} content={({ active, payload }) => {
          if (!active || !payload?.length) return null
          const d = payload[0].payload
          return (
            <div className="rounded-xl border border-white/10 bg-gray-900/95 px-3 py-2 shadow-2xl">
              <p className="mb-1 text-[11px] font-bold text-white">{d.fullName}</p>
              <div className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full" style={{ background: d.fill }} />
                <span className="text-[11px] text-gray-400">{PROVIDER_LABELS[d.provider] ?? d.provider}</span>
                <span className="text-xs font-bold text-white">{formatCurrency(d.cost)}</span>
              </div>
            </div>
          )
        }} />
        <Bar dataKey="cost" radius={[0, 6, 6, 0]} animationDuration={700}>
          {data.map((d, i) => <Cell key={i} fill={d.fill} />)}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}

// Monthly trend portlet
function MonthlyTrendChart({ data }: { data: Array<{ month: string; cost: number }> }) {
  return (
    <ResponsiveContainer width="100%" height={200}>
      <BarChart data={data.map(m => ({ name: m.month.slice(5), cost: m.cost, fullMonth: m.month }))} barSize={20}>
        <CartesianGrid strokeDasharray="3 3" vertical={false} opacity={0.07} />
        <XAxis dataKey="name" tick={{ fontSize: 10, fill: '#9ca3af' }} axisLine={false} tickLine={false} />
        <YAxis tick={{ fontSize: 10, fill: '#9ca3af' }} tickFormatter={v => v >= 1000 ? `${(v/1000).toFixed(0)}k` : `${v.toFixed(0)}`} axisLine={false} tickLine={false} width={40} />
        <Tooltip cursor={{ fill: 'rgba(99,102,241,0.05)' }} content={({ active, payload }) => {
          if (!active || !payload?.length) return null
          return (
            <div className="rounded-xl border border-white/10 bg-gray-900/95 px-3 py-2 shadow-2xl">
              <p className="mb-0.5 text-[10px] text-gray-400">{payload[0].payload.fullMonth}</p>
              <p className="text-xs font-bold text-white">{formatCurrency(payload[0].value as number)}</p>
            </div>
          )
        }} />
        <Bar dataKey="cost" radius={[6, 6, 0, 0]} animationDuration={700}>
          {data.map((_, i) => <Cell key={i} fill={VIVID[i % VIVID.length]} />)}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}

// Services by provider portlet
function ServicesByProvider({ services, serviceCountByProvider }: {
  services: Array<{ service: string; provider: string; cost: number }>
  serviceCountByProvider: Record<string, number>
}) {
  const providerOrder = ['aws', 'azure', 'gcp']
  const grouped: Record<string, Array<{ service: string; cost: number }>> = {}
  for (const s of services) {
    if (!grouped[s.provider]) grouped[s.provider] = []
    grouped[s.provider].push({ service: s.service, cost: s.cost })
  }
  const providers = [...providerOrder.filter(p => grouped[p]), ...Object.keys(grouped).filter(p => !providerOrder.includes(p))]
  if (!providers.length) return <p className="text-xs text-gray-400">No data</p>
  return (
    <div className="space-y-3">
      {providers.map((p, pi) => {
        const svcs = grouped[p] ?? []
        const maxCost = svcs[0]?.cost ?? 0
        const cfg = PROVIDER_CONFIG[p]
        return (
          <div key={p}>
            <div className="mb-1.5 flex items-center gap-2">
              <span className="rounded-md px-2 py-0.5 text-[10px] font-bold text-white"
                style={{ background: `linear-gradient(135deg, ${cfg?.from ?? '#6366f1'}, ${cfg?.to ?? '#8b5cf6'})` }}>
                {PROVIDER_LABELS[p] ?? p.toUpperCase()}
              </span>
              <span className="text-[11px] text-gray-400">{svcs.length} services · {serviceCountByProvider[p] ?? 0} resources</span>
            </div>
            <div className="space-y-1.5">
              {svcs.slice(0, 6).map((s, i) => {
                const pct = maxCost > 0 ? (s.cost / maxCost) * 100 : 0
                const rowColor = VIVID[(pi * 4 + i) % VIVID.length]
                return (
                  <div key={i} className="flex items-center gap-2">
                    <span className="w-4 text-right text-[10px] font-bold text-gray-300 dark:text-gray-600">{i + 1}</span>
                    <span className="w-36 truncate text-[11px] text-gray-700 dark:text-gray-300" title={s.service}>{s.service}</span>
                    <div className="flex flex-1 items-center gap-1.5">
                      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                        <div className="h-1.5 rounded-full transition-all duration-700" style={{ width: `${pct}%`, background: rowColor }} />
                      </div>
                    </div>
                    <span className="w-16 text-right text-[11px] font-bold text-gray-800 dark:text-white">{formatCurrency(s.cost)}</span>
                  </div>
                )
              })}
            </div>
          </div>
        )
      })}
    </div>
  )
}

// Service count portlet
function ServiceCountPortlet({ serviceCountByProvider, totalServices }: {
  serviceCountByProvider: Record<string, number>; totalServices: number
}) {
  const providers = Object.entries(serviceCountByProvider).filter(([, v]) => v > 0)
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-2xl font-black text-gray-800 dark:text-white">{totalServices}</span>
        <span className="text-[11px] text-gray-400">total services</span>
      </div>
      <div className="space-y-2">
        {providers.map(([p, count], i) => {
          const cfg = PROVIDER_CONFIG[p]
          const pct = totalServices > 0 ? (count / totalServices) * 100 : 0
          return (
            <div key={p}>
              <div className="mb-1 flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <span className="text-sm font-black" style={{ color: cfg?.from ?? VIVID[i] }}>{cfg?.icon ?? '◉'}</span>
                  <span className="text-xs font-semibold text-gray-700 dark:text-gray-300">{PROVIDER_LABELS[p] ?? p}</span>
                </div>
                <span className="text-xs font-bold text-gray-800 dark:text-white">{count}</span>
              </div>
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                <div className="h-1.5 rounded-full transition-all duration-700"
                  style={{ width: `${pct}%`, background: `linear-gradient(90deg, ${cfg?.from ?? VIVID[i]}, ${cfg?.to ?? VIVID[i]})` }} />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// Cloud accounts portlet
type Account = { id: string; account_name: string; provider: string; status: string; last_sync_at: string | null }

function ManageAccountsDrawer({ accounts, syncingId, onEdit, onSync, onDelete, onAdd, onClose }: {
  accounts: Account[]; syncingId: string | null
  onEdit: (a: Account) => void; onSync: (id: string) => void
  onDelete: (id: string) => void; onAdd: () => void; onClose: () => void
}) {
  const [confirmAcc, setConfirmAcc] = useState<Account | null>(null)

  return (
    <>
    {confirmAcc && (
      <ConfirmModal
        title={`Remove "${confirmAcc.account_name}"?`}
        message="This will disconnect the cloud account and remove all associated cost data. This action cannot be undone."
        confirmLabel="Remove Account"
        onConfirm={() => { onDelete(confirmAcc.id); setConfirmAcc(null) }}
        onCancel={() => setConfirmAcc(null)}
      />
    )}
    <div className="fixed inset-0 z-50 flex">
      {/* Backdrop */}
      <div className="flex-1 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      {/* Drawer */}
      <div className="flex w-full max-w-md flex-col bg-white dark:bg-gray-900 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-700 px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-100 dark:bg-blue-900/40">
              <Settings className="h-4 w-4 text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <p className="text-sm font-bold text-gray-900 dark:text-white">Manage Cloud Accounts</p>
              <p className="text-[11px] text-gray-400">{accounts.length} account{accounts.length !== 1 ? 's' : ''} connected</p>
            </div>
          </div>
          <button onClick={onClose} className="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-600">
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Add button */}
        <div className="px-5 py-3 border-b border-gray-100 dark:border-gray-800">
          <button onClick={onAdd}
            className="flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-emerald-500 to-teal-600 py-2.5 text-sm font-semibold text-white hover:opacity-90 active:scale-95 transition-all">
            <Plus className="h-4 w-4" /> Add Cloud Account
          </button>
        </div>

        {/* Account list */}
        <div className="flex-1 overflow-y-auto px-5 py-3 space-y-2">
          {accounts.length === 0 && (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <Cloud className="mb-3 h-10 w-10 text-gray-300 dark:text-gray-600" />
              <p className="text-sm text-gray-500 dark:text-gray-400">No accounts connected yet</p>
              <p className="text-xs text-gray-400 mt-1">Click "Add Cloud Account" to get started</p>
            </div>
          )}
          {accounts.map(acc => {
            const cfg = PROVIDER_CONFIG[acc.provider]
            const isSyncing = syncingId === acc.id
            return (
              <div key={acc.id} className="rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60 p-4">
                <div className="flex items-center gap-3 mb-3">
                  <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg text-sm font-black text-white"
                    style={{ background: `linear-gradient(135deg, ${cfg?.from ?? '#6366f1'}, ${cfg?.to ?? '#8b5cf6'})` }}>
                    {PROVIDER_ICON[acc.provider] ?? acc.provider.slice(0, 2).toUpperCase()}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-semibold text-gray-900 dark:text-white truncate">{acc.account_name}</p>
                    <p className="text-xs text-gray-400">{PROVIDER_LABELS[acc.provider] ?? acc.provider}</p>
                  </div>
                  <span className={`flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold ${
                    acc.status === 'active'
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                      : 'bg-gray-200 text-gray-500 dark:bg-gray-700 dark:text-gray-400'
                  }`}>
                    <span className={`h-1.5 w-1.5 rounded-full ${acc.status === 'active' ? 'bg-emerald-500' : 'bg-gray-400'}`} />
                    {acc.status}
                  </span>
                </div>
                {acc.last_sync_at && (
                  <p className="text-[10px] text-gray-400 mb-3">Last sync: {new Date(acc.last_sync_at).toLocaleString()}</p>
                )}
                <div className="flex gap-2">
                  <button onClick={() => onSync(acc.id)} disabled={isSyncing}
                    className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-blue-50 dark:bg-blue-900/30 py-1.5 text-xs font-semibold text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 disabled:opacity-40 transition-colors">
                    <RefreshCw className={`h-3.5 w-3.5 ${isSyncing ? 'animate-spin' : ''}`} />
                    {isSyncing ? 'Syncing…' : 'Sync'}
                  </button>
                  <button onClick={() => { onEdit(acc); onClose() }}
                    className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-gray-100 dark:bg-gray-700 py-1.5 text-xs font-semibold text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors">
                    <Pencil className="h-3.5 w-3.5" /> Edit
                  </button>
                  <button onClick={() => setConfirmAcc(acc)}
                    className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-red-50 dark:bg-red-900/30 py-1.5 text-xs font-semibold text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors">
                    <Trash2 className="h-3.5 w-3.5" /> Delete
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
    </>
  )
}

function AccountsPortlet({ accounts, syncingId, onEdit, onSync, onDelete, onAdd }: {
  accounts: Account[]; syncingId: string | null
  onEdit: (a: Account) => void; onSync: (id: string) => void
  onDelete: (id: string) => void; onAdd: () => void
}) {
  const [showDrawer, setShowDrawer] = useState(false)

  // Group by provider
  const byProvider = accounts.reduce<Record<string, Account[]>>((acc, a) => {
    if (!acc[a.provider]) acc[a.provider] = []
    acc[a.provider].push(a)
    return acc
  }, {})

  return (
    <>
      {accounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-6">
          <Cloud className="mb-2 h-8 w-8 text-gray-300 dark:text-gray-600" />
          <p className="text-xs text-gray-400 mb-3">No accounts connected</p>
          <button onClick={onAdd}
            className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-700">
            <Plus className="h-3 w-3" /> Connect Account
          </button>
        </div>
      ) : (
        <div className="space-y-2">
          {/* Provider summary cards */}
          {Object.entries(byProvider).map(([provider, accs]) => {
            const cfg = PROVIDER_CONFIG[provider]
            const activeCount = accs.filter(a => a.status === 'active').length
            return (
              <div key={provider} className="flex items-center gap-3 rounded-xl p-3"
                style={{ background: `linear-gradient(135deg, ${cfg?.from ?? '#6366f1'}18, ${cfg?.to ?? '#8b5cf6'}10)`, border: `1px solid ${cfg?.from ?? '#6366f1'}30` }}>
                <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg text-sm font-black text-white"
                  style={{ background: `linear-gradient(135deg, ${cfg?.from ?? '#6366f1'}, ${cfg?.to ?? '#8b5cf6'})` }}>
                  {PROVIDER_ICON[provider] ?? provider.slice(0, 2).toUpperCase()}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-bold text-gray-800 dark:text-white">{PROVIDER_LABELS[provider] ?? provider}</p>
                  <p className="text-[10px] text-gray-500 dark:text-gray-400">
                    {accs.length} account{accs.length !== 1 ? 's' : ''} · {activeCount} active
                  </p>
                </div>
                <div className="flex flex-col gap-0.5 items-end">
                  {accs.map(a => (
                    <span key={a.id} className="text-[10px] font-medium text-gray-600 dark:text-gray-300 truncate max-w-[100px]">{a.account_name}</span>
                  ))}
                </div>
              </div>
            )
          })}
          {/* Manage button */}
          <button onClick={() => setShowDrawer(true)}
            className="flex w-full items-center justify-center gap-2 rounded-xl border border-dashed border-gray-300 dark:border-gray-600 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 hover:border-blue-400 hover:text-blue-500 dark:hover:border-blue-500 dark:hover:text-blue-400 transition-colors">
            <Settings className="h-3.5 w-3.5" /> Manage Accounts
          </button>
        </div>
      )}

      {showDrawer && (
        <ManageAccountsDrawer
          accounts={accounts} syncingId={syncingId}
          onEdit={onEdit} onSync={onSync} onDelete={onDelete}
          onAdd={() => { setShowDrawer(false); onAdd() }}
          onClose={() => setShowDrawer(false)}
        />
      )}
    </>
  )
}

// Empty state helper
function EmptyState({ label }: { label: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-center">
      <div className="mb-2 h-8 w-8 rounded-full bg-gray-100 dark:bg-gray-700 flex items-center justify-center">
        <span className="text-gray-400 text-sm">—</span>
      </div>
      <p className="text-xs text-gray-400">{label}</p>
    </div>
  )
}

// P11: Cost Efficiency Score
function CostEfficiencyPortlet({ spent, forecast, anomalyCount, recCount }: {
  spent: number; forecast: number; anomalyCount: number; recCount: number
}) {
  const budgetPct = forecast > 0 ? (spent / forecast) * 100 : 0
  const score = Math.max(0, Math.round(100 - (budgetPct > 100 ? 30 : budgetPct > 80 ? 15 : 0) - anomalyCount * 5 - recCount * 2))
  const color = score >= 80 ? '#10b981' : score >= 60 ? '#f59e0b' : '#ef4444'
  const label = score >= 80 ? 'Excellent' : score >= 60 ? 'Good' : 'Needs Attention'
  const metrics = [
    { label: 'Budget Usage', value: `${budgetPct.toFixed(1)}%`, ok: budgetPct <= 90 },
    { label: 'Active Anomalies', value: String(anomalyCount), ok: anomalyCount === 0 },
    { label: 'Open Recommendations', value: String(recCount), ok: recCount === 0 },
    { label: 'Accounts Connected', value: spent > 0 ? 'Yes' : 'No', ok: spent > 0 },
  ]
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <div className="relative flex h-20 w-20 flex-shrink-0 items-center justify-center rounded-full border-4" style={{ borderColor: color }}>
          <span className="text-xl font-black" style={{ color }}>{score}</span>
        </div>
        <div>
          <p className="text-lg font-black text-gray-800 dark:text-white">{label}</p>
          <p className="text-xs text-gray-400">Overall efficiency score out of 100</p>
        </div>
      </div>
      <div className="space-y-2">
        {metrics.map((m, i) => (
          <div key={i} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 dark:bg-gray-700/40">
            <span className="text-xs text-gray-600 dark:text-gray-400">{m.label}</span>
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-bold text-gray-800 dark:text-white">{m.value}</span>
              <span className={`h-2 w-2 rounded-full ${m.ok ? 'bg-emerald-500' : 'bg-amber-400'}`} />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// P12: Spend Velocity
function SpendVelocityPortlet({ dailyCosts, forecast }: {
  dailyCosts: Array<{ date: string; cost: number }>; forecast: number
}) {
  const today = new Date()
  const daysElapsed = today.getDate()
  const daysInMonth = new Date(today.getFullYear(), today.getMonth() + 1, 0).getDate()
  const totalSpent = dailyCosts.reduce((s, d) => s + d.cost, 0)
  const dailyAvg = daysElapsed > 0 ? totalSpent / daysElapsed : 0
  const projected = dailyAvg * daysInMonth
  const last7 = dailyCosts.slice(-7)
  const recentAvg = last7.length > 0 ? last7.reduce((s, d) => s + d.cost, 0) / last7.length : 0
  const trend = recentAvg > dailyAvg ? 'up' : recentAvg < dailyAvg * 0.9 ? 'down' : 'stable'
  const trendColor = trend === 'up' ? 'text-red-500' : trend === 'down' ? 'text-emerald-500' : 'text-amber-500'
  const trendIcon = trend === 'up' ? '↑' : trend === 'down' ? '↓' : '→'
  const stats = [
    { label: 'Daily Average', value: formatCurrency(dailyAvg) },
    { label: 'Last 7-day Avg', value: formatCurrency(recentAvg) },
    { label: 'Projected EOM', value: formatCurrency(projected) },
    { label: 'Forecast', value: formatCurrency(forecast) },
  ]
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3 rounded-xl bg-gray-50 px-4 py-3 dark:bg-gray-700/40">
        <span className={`text-2xl font-black ${trendColor}`}>{trendIcon}</span>
        <div>
          <p className="text-sm font-bold text-gray-800 dark:text-white capitalize">{trend} trend</p>
          <p className="text-[11px] text-gray-400">Based on last 7 days vs month average</p>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        {stats.map((s, i) => (
          <div key={i} className="rounded-lg bg-gray-50 px-3 py-2 dark:bg-gray-700/40">
            <p className="text-[10px] text-gray-400">{s.label}</p>
            <p className="text-sm font-bold text-gray-800 dark:text-white">{s.value}</p>
          </div>
        ))}
      </div>
      <div>
        <div className="mb-1 flex justify-between text-[10px] text-gray-400">
          <span>Month progress</span><span>{daysElapsed}/{daysInMonth} days</span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
          <div className="h-2 rounded-full bg-indigo-500 transition-all duration-700"
            style={{ width: `${(daysElapsed / daysInMonth) * 100}%` }} />
        </div>
      </div>
    </div>
  )
}

// P13: Provider Comparison
function ProviderComparisonPortlet({ byProvider, serviceCountByProvider }: {
  byProvider: Record<string, number>; serviceCountByProvider: Record<string, number>
}) {
  const total = Object.values(byProvider).reduce((a, b) => a + b, 0)
  const providers = Object.entries(byProvider).filter(([, v]) => v > 0).sort(([, a], [, b]) => b - a)
  if (!providers.length) return <EmptyState label="No provider data" />
  return (
    <div className="space-y-3">
      {providers.map(([p, cost], i) => {
        const cfg = PROVIDER_CONFIG[p]
        const pct = total > 0 ? (cost / total) * 100 : 0
        const svcCount = serviceCountByProvider[p] ?? 0
        return (
          <div key={p} className="rounded-xl border border-gray-100 p-3 dark:border-gray-700/60">
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="rounded-md px-2 py-0.5 text-[10px] font-black text-white"
                  style={{ background: `linear-gradient(135deg, ${cfg?.from ?? VIVID[i]}, ${cfg?.to ?? VIVID[i]})` }}>
                  {PROVIDER_LABELS[p] ?? p.toUpperCase()}
                </span>
                <span className="text-[11px] text-gray-400">{svcCount} services</span>
              </div>
              <span className="text-sm font-black text-gray-800 dark:text-white">{formatCurrency(cost)}</span>
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                <div className="h-2 rounded-full transition-all duration-700"
                  style={{ width: `${pct}%`, background: `linear-gradient(90deg, ${cfg?.from ?? VIVID[i]}, ${cfg?.to ?? VIVID[i]})` }} />
              </div>
              <span className="w-10 text-right text-[11px] font-bold text-gray-500">{pct.toFixed(1)}%</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

// P14: Top Regions
function TopRegionsPortlet({ services }: { services: Array<{ service: string; provider: string; cost: number }> }) {
  // Derive regions from service names as a proxy (real region data would come from API)
  const providerCosts: Record<string, number> = {}
  for (const s of services) {
    providerCosts[s.provider] = (providerCosts[s.provider] ?? 0) + s.cost
  }
  const regions: Array<{ region: string; provider: string; cost: number }> = Object.entries(providerCosts).map(([p, cost]) => ({
    region: p === 'aws' ? 'us-east-1' : p === 'azure' ? 'eastus' : 'us-central1',
    provider: p, cost,
  }))
  const total = regions.reduce((s, r) => s + r.cost, 0)
  if (!regions.length) return <EmptyState label="No region data available" />
  return (
    <div className="space-y-2">
      {regions.sort((a, b) => b.cost - a.cost).map((r, i) => {
        const cfg = PROVIDER_CONFIG[r.provider]
        const pct = total > 0 ? (r.cost / total) * 100 : 0
        return (
          <div key={i} className="flex items-center gap-3">
            <span className="w-5 text-right text-[10px] font-bold text-gray-300">{i + 1}</span>
            <div className="flex-1">
              <div className="mb-1 flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-semibold text-gray-700 dark:text-gray-300">{r.region}</span>
                  <span className="rounded px-1 text-[9px] font-bold text-white"
                    style={{ background: cfg?.from ?? VIVID[i] }}>{PROVIDER_LABELS[r.provider] ?? r.provider}</span>
                </div>
                <span className="text-xs font-bold text-gray-800 dark:text-white">{formatCurrency(r.cost)}</span>
              </div>
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                <div className="h-1.5 rounded-full transition-all duration-700"
                  style={{ width: `${pct}%`, background: cfg?.from ?? VIVID[i] }} />
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

// P15: Savings Potential
function SavingsPotentialPortlet({ recs, totalCost }: {
  recs: Array<{ type: string; potential_monthly_savings: number; description: string }>; totalCost: number
}) {
  const totalSavings = recs.reduce((s, r) => s + r.potential_monthly_savings, 0)
  const savingsPct = totalCost > 0 ? (totalSavings / totalCost) * 100 : 0
  const byType: Record<string, number> = {}
  for (const r of recs) byType[r.type] = (byType[r.type] ?? 0) + r.potential_monthly_savings
  const color = savingsPct > 20 ? '#10b981' : savingsPct > 10 ? '#f59e0b' : '#6366f1'
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4 rounded-xl p-3" style={{ background: `${color}15` }}>
        <div className="text-center">
          <p className="text-2xl font-black" style={{ color }}>{formatCurrency(totalSavings)}</p>
          <p className="text-[10px] text-gray-400">potential/month</p>
        </div>
        <div className="flex-1">
          <div className="mb-1 flex justify-between text-[10px] text-gray-400">
            <span>Savings opportunity</span><span>{savingsPct.toFixed(1)}% of spend</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
            <div className="h-2 rounded-full transition-all duration-700"
              style={{ width: `${Math.min(savingsPct, 100)}%`, background: color }} />
          </div>
        </div>
      </div>
      {Object.entries(byType).length > 0 && (
        <div className="space-y-1.5">
          <p className="text-[10px] font-bold uppercase tracking-widest text-gray-400">By Type</p>
          {Object.entries(byType).sort(([, a], [, b]) => b - a).slice(0, 4).map(([type, savings], i) => (
            <div key={i} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-1.5 dark:bg-gray-700/40">
              <span className="text-[11px] capitalize text-gray-600 dark:text-gray-400">{type.replace(/_/g, ' ')}</span>
              <span className="text-xs font-bold text-emerald-600 dark:text-emerald-400">{formatCurrency(savings)}</span>
            </div>
          ))}
        </div>
      )}
      {recs.length === 0 && <EmptyState label="No savings recommendations yet" />}
    </div>
  )
}

// P16: Account Health
function AccountHealthPortlet({ accounts }: { accounts: Array<{ id: string; account_name: string; provider: string; status: string; last_sync_at: string | null }> }) {
  if (!accounts.length) return <EmptyState label="No accounts connected" />
  const active = accounts.filter(a => a.status === 'active').length
  const inactive = accounts.length - active
  const healthPct = accounts.length > 0 ? (active / accounts.length) * 100 : 0
  const color = healthPct === 100 ? '#10b981' : healthPct >= 50 ? '#f59e0b' : '#ef4444'
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4">
        <div className="relative flex h-16 w-16 flex-shrink-0 items-center justify-center rounded-full border-4" style={{ borderColor: color }}>
          <span className="text-base font-black" style={{ color }}>{healthPct.toFixed(0)}%</span>
        </div>
        <div className="flex gap-4">
          <div className="text-center">
            <p className="text-xl font-black text-emerald-600">{active}</p>
            <p className="text-[10px] text-gray-400">Active</p>
          </div>
          <div className="text-center">
            <p className="text-xl font-black text-red-500">{inactive}</p>
            <p className="text-[10px] text-gray-400">Inactive</p>
          </div>
        </div>
      </div>
      <div className="space-y-1.5">
        {accounts.map(acc => {
          const cfg = PROVIDER_CONFIG[acc.provider]
          const isActive = acc.status === 'active'
          const lastSync = acc.last_sync_at ? new Date(acc.last_sync_at).toLocaleDateString() : 'Never'
          return (
            <div key={acc.id} className="flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 dark:bg-gray-700/40">
              <span className={`h-2 w-2 flex-shrink-0 rounded-full ${isActive ? 'bg-emerald-500' : 'bg-red-400'}`} />
              <span className="flex-1 truncate text-[11px] font-semibold text-gray-700 dark:text-gray-300">{acc.account_name}</span>
              <span className="text-[9px] font-bold text-white rounded px-1.5 py-0.5"
                style={{ background: cfg?.from ?? '#6366f1' }}>{PROVIDER_LABELS[acc.provider] ?? acc.provider}</span>
              <span className="text-[10px] text-gray-400">{lastSync}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function CostVsForecastPortlet({ byProvider, forecast, totalCost }: {
  byProvider: Record<string, number>; forecast: number; totalCost: number
}) {
  const PROVIDER_COLOR: Record<string, string> = { aws: '#FF9900', azure: '#0078D4', gcp: '#34A853' }
  const providers = Object.entries(byProvider).filter(([, v]) => v > 0)
  if (!providers.length) return <EmptyState label="No cost data available" />
  const totalForecast = forecast || totalCost
  const data = providers.map(([p, mtd]) => {
    const share = totalCost > 0 ? mtd / totalCost : 0
    return {
      name: PROVIDER_LABELS[p] ?? p.toUpperCase(),
      mtd: Math.round(mtd * 100) / 100,
      forecast: Math.round(share * totalForecast * 100) / 100,
      color: PROVIDER_COLOR[p] ?? '#6366f1',
    }
  })
  return (
    <div className="space-y-3">
      {data.map(d => {
        const pct = d.forecast > 0 ? Math.min((d.mtd / d.forecast) * 100, 100) : 0
        const over = d.mtd > d.forecast
        return (
          <div key={d.name}>
            <div className="flex items-center justify-between mb-1">
              <div className="flex items-center gap-2">
                <span className="text-[10px] font-black text-white rounded px-1.5 py-0.5" style={{ background: d.color }}>{d.name}</span>
                <span className="text-xs font-bold text-gray-800 dark:text-white">${d.mtd.toFixed(2)}</span>
              </div>
              <span className={`text-[10px] font-semibold ${over ? 'text-red-500' : 'text-gray-400'}`}>
                of ${d.forecast.toFixed(2)} forecast
              </span>
            </div>
            <div className="h-2.5 w-full rounded-full bg-gray-100 dark:bg-gray-700 overflow-hidden">
              <div className="h-full rounded-full transition-all duration-700"
                style={{ width: `${pct}%`, background: over ? '#ef4444' : d.color }} />
            </div>
            <p className="text-[10px] text-gray-400 mt-0.5 text-right">{pct.toFixed(0)}% of forecast used</p>
          </div>
        )
      })}
      <div className="mt-2 flex items-center justify-between rounded-lg bg-gray-50 dark:bg-gray-700/40 px-3 py-2">
        <span className="text-xs font-semibold text-gray-600 dark:text-gray-300">Total MTD</span>
        <span className="text-sm font-black text-gray-900 dark:text-white">${totalCost.toFixed(2)}</span>
        <span className="text-xs text-gray-400">Forecast: ${totalForecast.toFixed(2)}</span>
      </div>
    </div>
  )
}

function ResourceTilesPortlet({ accounts }: { accounts: CloudAccount[] }) {
  const PROVIDER_COLOR: Record<string, string> = { aws: '#FF9900', azure: '#0078D4', gcp: '#34A853' }
  const qc = useQueryClient()
  const [spinning, setSpinning] = useState(false)

  async function handleRefresh() {
    setSpinning(true)
    await Promise.all(accounts.map(acc => qc.invalidateQueries({ queryKey: ['cloud-account-tiles', acc.id] })))
    setTimeout(() => setSpinning(false), 600)
  }

  if (accounts.length === 0) return <EmptyState label="No cloud accounts connected" />

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <button
          onClick={handleRefresh}
          disabled={spinning}
          className="flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-orange-500 to-amber-500 px-2.5 py-1.5 text-xs font-semibold text-white shadow-sm hover:opacity-90 disabled:opacity-60 active:scale-95 transition-all"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${spinning ? 'animate-spin' : ''}`} />
          {spinning ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>
      {accounts.map(acc => (
        <AccountTileRow key={acc.id} id={acc.id} name={acc.account_name} provider={acc.provider} providerColor={PROVIDER_COLOR[acc.provider] ?? '#6366f1'} />
      ))}
    </div>
  )
}

function AccountTileRow({ id, name, provider, providerColor }: { id: string; name: string; provider: string; providerColor: string }) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['cloud-account-tiles', id],
    queryFn: () => finopsService.getCloudAccountTiles(id),
    staleTime: 5 * 60 * 1000,
  })

  return (
    <div className="rounded-xl border border-gray-100 dark:border-gray-700/60 bg-gray-50/60 dark:bg-gray-800/40 p-3">
      <div className="flex items-center gap-2 mb-2.5">
        <span className="text-[10px] font-black uppercase tracking-wider px-2 py-0.5 rounded-md text-white" style={{ background: providerColor }}>
          {provider}
        </span>
        <span className="text-xs font-semibold text-gray-700 dark:text-gray-200 truncate">{name}</span>
      </div>
      {isLoading && (
        <div className="flex gap-2 flex-wrap">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-[68px] w-[72px] rounded-lg bg-gray-200 dark:bg-gray-700 animate-pulse" />
          ))}
        </div>
      )}
      {isError && <p className="text-xs text-red-400">Failed to load resource counts</p>}
      {data && data.tiles.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {data.tiles.map((t: ResourceTile) => (
            <div
              key={t.label}
              className="flex flex-col items-center justify-center rounded-lg px-3 py-2 min-w-[68px]"
              style={{ background: `${t.color}18`, border: `1px solid ${t.color}40` }}
            >
              <span className="text-base leading-none">{t.icon}</span>
              <span className="text-lg font-bold leading-tight mt-0.5" style={{ color: t.color }}>{t.count}</span>
              <span className="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5 text-center leading-tight">{t.label}</span>
            </div>
          ))}
        </div>
      )}
      {data && data.tiles.length === 0 && <p className="text-xs text-gray-400">No resource data available</p>}
    </div>
  )
}

export default function FinOpsPage() {
  const { startDate, endDate } = getDateRange()
  const queryClient = useQueryClient()
  const [syncingId, setSyncingId] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [selectedAccount, setSelectedAccount] = useState<Account | null>(null)
  const [showPanel, setShowPanel] = useState(false)
  const [showReportModal, setShowReportModal] = useState(false)
  const [showManageDrawer, setShowManageDrawer] = useState(false)

  const { data: costData, isLoading: costLoading, refetch: refetchCost } = useQuery({
    queryKey: ['cost-summary', startDate, endDate],
    queryFn: () => finopsService.getCostSummary(startDate, endDate),
    staleTime: 5 * 60 * 1000,
  })
  const { data: accountsData, isLoading: accountsLoading, refetch: refetchAccounts } = useQuery({
    queryKey: ['cloud-accounts'],
    queryFn: () => finopsService.getCloudAccounts(),
  })
  const { data: anomaliesData } = useQuery({ queryKey: ['anomalies'], queryFn: () => finopsService.getAnomalies(30) })
  const { data: recsData } = useQuery({ queryKey: ['recommendations'], queryFn: () => finopsService.getRecommendations() })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => finopsService.deleteCloudAccount(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['cloud-accounts'] }); if (selectedAccount) setSelectedAccount(null); toast.success('Account removed') },
    onError: () => toast.error('Failed to remove account'),
  })
  const syncMutation = useMutation({
    mutationFn: async (id: string) => { setSyncingId(id); await finopsService.syncCloudAccount(id) },
    onSuccess: () => { setSyncingId(null); queryClient.invalidateQueries({ queryKey: ['cloud-accounts'] }); queryClient.invalidateQueries({ queryKey: ['cost-summary'] }); toast.success('Sync complete') },
    onError: () => { setSyncingId(null); toast.error('Sync failed') },
  })

  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true)
    try {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['cost-summary'] }),
        queryClient.invalidateQueries({ queryKey: ['cloud-accounts'] }),
        queryClient.invalidateQueries({ queryKey: ['anomalies'] }),
        queryClient.invalidateQueries({ queryKey: ['recommendations'] }),
      ])
      await Promise.all([refetchCost(), refetchAccounts()])
    } finally {
      setTimeout(() => setIsRefreshing(false), 600)
    }
  }, [queryClient, refetchCost, refetchAccounts])

  const allAccounts: CloudAccount[] = accountsData?.cloud_accounts ?? []
  const services = costData?.breakdown_by_service ?? []
  const filteredServices = selectedAccount ? services.filter(s => s.provider === selectedAccount.provider) : services
  const byProvider = costData?.breakdown_by_provider ?? {}
  const filteredByProvider = selectedAccount
    ? Object.fromEntries(Object.entries(byProvider).filter(([p]) => p === selectedAccount.provider))
    : byProvider
  const totalCost = selectedAccount ? (filteredByProvider[selectedAccount.provider] ?? 0) : (costData?.total_cost ?? 0)
  const forecast = costData?.forecast_cost ?? 0
  const filteredForecast = selectedAccount && costData?.total_cost ? (totalCost / (costData.total_cost || 1)) * forecast : forecast
  const serviceCountByProvider = costData?.service_count_by_provider ?? {}
  const dailyCosts = costData?.daily_costs ?? []
  const monthlyTrends = costData?.monthly_trends ?? []
  const anomalies = anomaliesData?.anomalies ?? []
  const recs = recsData?.recommendations ?? []

  const areaData = (() => {
    const map: Record<string, Record<string, unknown>> = {}
    for (const d of dailyCosts) {
      if (!map[d.date]) map[d.date] = { date: d.date }
      const total = Object.values(byProvider).reduce((a, b) => a + b, 0)
      for (const [p, v] of Object.entries(filteredByProvider)) {
        const share = total > 0 ? v / total : 0
        map[d.date][p] = ((map[d.date][p] as number) ?? 0) + d.cost * share
      }
    }
    return Object.values(map).sort((a, b) => String(a.date) < String(b.date) ? -1 : 1)
  })()

  const totalSparkData = dailyCosts.map(d => d.cost)
  const providerDonutData = Object.entries(filteredByProvider).filter(([, v]) => v > 0).map(([name, value]) => ({ name: PROVIDER_LABELS[name] ?? name, value }))
  const isLoading = costLoading || accountsLoading
  const spinning = isLoading || isRefreshing
  const totalServiceCount = Object.values(serviceCountByProvider).reduce((a, b) => a + b, 0)

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 via-slate-50 to-blue-50/30 dark:from-gray-950 dark:via-gray-900 dark:to-slate-900">

      {/* Toolbar */}
      <div className="border-b border-gray-200/60 bg-white/80 backdrop-blur-sm dark:border-gray-700/60 dark:bg-gray-900/70">
        <div className="mx-auto max-w-screen-xl flex items-center gap-2 px-4 py-2.5">
          <div className="flex flex-1 flex-wrap items-center gap-1.5 min-w-0">
            <button onClick={() => setSelectedAccount(null)}
              className={`rounded-lg px-2.5 py-1 text-[11px] font-bold transition-all border ${!selectedAccount ? 'bg-indigo-600 text-white border-indigo-600 shadow-sm' : 'border-gray-200 bg-white text-gray-500 hover:border-indigo-300 hover:text-indigo-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400'}`}>
              All Accounts
            </button>
            {allAccounts.map(acc => {
              const cfg = PROVIDER_CONFIG[acc.provider]
              const active = selectedAccount?.id === acc.id
              return (
                <button key={acc.id} onClick={() => setSelectedAccount(a => a?.id === acc.id ? null : acc)}
                  className={`flex items-center gap-1 rounded-lg px-2.5 py-1 text-[11px] font-bold transition-all border ${active ? 'text-white border-transparent shadow-sm' : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}`}
                  style={active ? { background: `linear-gradient(135deg, ${cfg?.from ?? '#6366f1'}, ${cfg?.to ?? '#8b5cf6'})` } : {}}>
                  <span className={`h-1.5 w-1.5 rounded-full ${acc.status === 'active' ? 'bg-emerald-400' : 'bg-gray-300'}`} />
                  <span className="max-w-[100px] truncate">{acc.account_name}</span>
                  <span className={`rounded px-1 text-[9px] font-black ${active ? 'bg-white/25 text-white' : 'bg-gray-100 text-gray-400 dark:bg-gray-700'}`}>{PROVIDER_LABELS[acc.provider] ?? acc.provider}</span>
                </button>
              )
            })}
          </div>
          <div className="flex flex-shrink-0 items-center gap-1.5">
            <button onClick={() => setShowReportModal(true)}
              className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200">
              <Mail className="h-3.5 w-3.5" /><span className="hidden sm:inline">Reports</span>
            </button>
            <button onClick={handleRefresh} disabled={spinning}
              className="flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-indigo-500 to-blue-600 px-2.5 py-1.5 text-xs font-semibold text-white shadow-sm hover:opacity-90 disabled:opacity-70 active:scale-95">
              <RefreshCw className={`h-3.5 w-3.5 ${spinning ? 'animate-spin' : ''}`} />
              <span className="hidden sm:inline">{spinning ? 'Refreshing...' : 'Refresh'}</span>
            </button>
            <button onClick={() => { setSelectedAccount(null); setShowPanel(true) }}
              className="flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-emerald-500 to-teal-600 px-2.5 py-1.5 text-xs font-semibold text-white shadow-sm hover:opacity-90 active:scale-95">
              <Plus className="h-3.5 w-3.5" /><span className="hidden sm:inline">Add Account</span>
            </button>
            <button onClick={() => setShowManageDrawer(true)}
              className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
              title="Manage Accounts">
              <Settings className="h-3.5 w-3.5" /><span className="hidden sm:inline">Manage</span>
            </button>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="mx-auto max-w-screen-xl space-y-3 px-4 py-4">
        {isLoading ? (
          <div className="flex h-64 items-center justify-center"><LoadingSpinner /></div>
        ) : (
          <>
            {/* Row 1: KPI tiles — 4 equal columns */}
            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <KpiTile label="Month-to-Date" value={totalCost} prefix="$" icon={DollarSign} from="#6366f1" to="#8b5cf6" glow="rgba(99,102,241,0.35)" sparkData={totalSparkData} />
              <KpiTile label="Forecast" value={filteredForecast} prefix="$" icon={TrendingUp} from="#f59e0b" to="#f97316" glow="rgba(245,158,11,0.35)" sparkData={totalSparkData} />
              <KpiTile label="Services" value={filteredServices.length} prefix="" icon={Layers} from="#10b981" to="#06b6d4" glow="rgba(16,185,129,0.35)" animate={false} />
              <KpiTile label="Accounts" value={allAccounts.length} prefix="" icon={Cloud} from="#3b82f6" to="#6366f1" glow="rgba(59,130,246,0.35)" animate={false} />
            </div>

            {/* Row 2: Provider tiles */}
            {Object.keys(filteredByProvider).length > 0 && (
              <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
                {Object.entries(filteredByProvider).filter(([, v]) => v > 0).map(([p, v]) => (
                  <ProviderTile key={p} provider={p} cost={v} pct={totalCost > 0 ? (v / totalCost) * 100 : 0} />
                ))}
              </div>
            )}

            {/* Portlet grid — 2 per row, 8 rows = 16 portlets */}
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">

              {/* P1: Daily Cost Breakdown */}
              <Portlet title="Daily Cost Breakdown" sub="Day-by-day spend across all providers this month" icon={Activity} iconBg="bg-indigo-100 text-indigo-600 dark:bg-indigo-900/40 dark:text-indigo-400">
                <DailyAreaChart areaData={areaData} byProvider={filteredByProvider} />
              </Portlet>

              {/* P2: Cost by Provider */}
              <Portlet title="Cost by Provider" sub="Proportional spend distribution across cloud providers" icon={BarChart2} iconBg="bg-violet-100 text-violet-600 dark:bg-violet-900/40 dark:text-violet-400">
                <ProviderDonut data={providerDonutData} />
              </Portlet>

              {/* P3: Top Services by Cost */}
              <Portlet title="Top Services by Cost" sub="Highest-spend services ranked by month-to-date cost" icon={BarChart2} iconBg="bg-rose-100 text-rose-600 dark:bg-rose-900/40 dark:text-rose-400">
                <TopServicesChart services={filteredServices} />
              </Portlet>

              {/* P4: Month Progress / Budget Gauge */}
              <Portlet title="Month Progress" sub="Actual spend vs forecasted budget for the current month" icon={TrendingUp} iconBg="bg-emerald-100 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-400">
                <BudgetGauge spent={totalCost} forecast={filteredForecast} />
              </Portlet>

              {/* P5: Services by Provider */}
              <Portlet title="Services by Provider" sub="Cost breakdown per service grouped by cloud provider" icon={Layers} iconBg="bg-emerald-100 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-400">
                <ServicesByProvider services={filteredServices} serviceCountByProvider={serviceCountByProvider} />
              </Portlet>

              {/* P6: Resource Count */}
              <Portlet title="Resource Count" sub="Total active services and resources per cloud provider" icon={Server} iconBg="bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-400">
                <ServiceCountPortlet serviceCountByProvider={serviceCountByProvider} totalServices={totalServiceCount} />
              </Portlet>

              {/* P7: Monthly Trend */}
              <Portlet title="Monthly Trend" sub="Historical month-over-month cost comparison" icon={TrendingUp} iconBg="bg-amber-100 text-amber-600 dark:bg-amber-900/40 dark:text-amber-400">
                {monthlyTrends.length > 0
                  ? <MonthlyTrendChart data={monthlyTrends} />
                  : <EmptyState label="No monthly trend data yet" />}
              </Portlet>

              {/* P8: Cost vs Forecast by Provider */}
              <Portlet title="Cost vs Forecast" sub="Month-to-date spend vs projected end-of-month per provider" icon={BarChart2} iconBg="bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-400">
                <CostVsForecastPortlet byProvider={filteredByProvider} forecast={filteredForecast} totalCost={totalCost} />
              </Portlet>

              {/* P9: Cost Anomalies */}
              <Portlet title="Cost Anomalies" sub="Unusual spend spikes detected in the last 30 days" icon={AlertTriangle} iconBg="bg-amber-100 text-amber-600 dark:bg-amber-900/40 dark:text-amber-400">
                {anomalies.length > 0
                  ? <div className="space-y-2">{anomalies.slice(0, 4).map(a => <AnomalyAlert key={a.id} anomaly={a} />)}</div>
                  : <EmptyState label="No anomalies detected" />}
              </Portlet>

              {/* P10: Savings Recommendations */}
              <Portlet title="Savings Recommendations" sub="Actionable suggestions to reduce your cloud spend" icon={Zap} iconBg="bg-emerald-100 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-400">
                {recs.length > 0
                  ? <div className="space-y-2">{recs.slice(0, 4).map(r => <RecommendationCard key={r.id} recommendation={r} />)}</div>
                  : <EmptyState label="No recommendations available" />}
              </Portlet>

              {/* P11: Cost Efficiency Score */}
              <Portlet title="Cost Efficiency Score" sub="How well your spend aligns with forecasted budget" icon={Zap} iconBg="bg-indigo-100 text-indigo-600 dark:bg-indigo-900/40 dark:text-indigo-400">
                <CostEfficiencyPortlet spent={totalCost} forecast={filteredForecast} anomalyCount={anomalies.length} recCount={recs.length} />
              </Portlet>

              {/* P12: Spend Velocity */}
              <Portlet title="Spend Velocity" sub="Daily burn rate and projected end-of-month cost" icon={Activity} iconBg="bg-rose-100 text-rose-600 dark:bg-rose-900/40 dark:text-rose-400">
                <SpendVelocityPortlet dailyCosts={dailyCosts} forecast={filteredForecast} />
              </Portlet>

              {/* P13: Provider Comparison */}
              <Portlet title="Provider Comparison" sub="Side-by-side cost and service count across providers" icon={BarChart2} iconBg="bg-cyan-100 text-cyan-600 dark:bg-cyan-900/40 dark:text-cyan-400">
                <ProviderComparisonPortlet byProvider={filteredByProvider} serviceCountByProvider={serviceCountByProvider} />
              </Portlet>

              {/* P14: Top Regions */}
              <Portlet title="Top Regions by Cost" sub="Highest-spend cloud regions across all providers" icon={Cloud} iconBg="bg-teal-100 text-teal-600 dark:bg-teal-900/40 dark:text-teal-400">
                <TopRegionsPortlet services={filteredServices} />
              </Portlet>

              {/* P15: Savings Potential */}
              <Portlet title="Savings Potential" sub="Estimated monthly savings from all recommendations" icon={DollarSign} iconBg="bg-green-100 text-green-600 dark:bg-green-900/40 dark:text-green-400">
                <SavingsPotentialPortlet recs={recs} totalCost={totalCost} />
              </Portlet>

              {/* P16: Account Health */}
              <Portlet title="Account Health" sub="Sync status and connectivity health of all cloud accounts" icon={Server} iconBg="bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-400">
                <AccountHealthPortlet accounts={allAccounts} />
              </Portlet>

              {/* P17: Live Resource Tiles */}
              <Portlet title="Live Resource Counts" sub="Real-time resource inventory per cloud account from live APIs" icon={Server} iconBg="bg-orange-100 text-orange-600 dark:bg-orange-900/40 dark:text-orange-400" className="lg:col-span-2">
                <ResourceTilesPortlet accounts={allAccounts} />
              </Portlet>

            </div>
          </>
        )}
      </div>

      {showPanel && (
        <CloudAccountModal
          account={selectedAccount}
          onClose={() => { setShowPanel(false); setSelectedAccount(null) }}
          onSaved={() => {
            setShowPanel(false); setSelectedAccount(null)
            queryClient.invalidateQueries({ queryKey: ['cloud-accounts'] })
            queryClient.invalidateQueries({ queryKey: ['cost-summary'] })
          }}
        />
      )}
      {showReportModal && <ReportScheduleModal onClose={() => setShowReportModal(false)} />}
      {showManageDrawer && (
        <ManageAccountsDrawer
          accounts={allAccounts} syncingId={syncingId}
          onEdit={acc => { setSelectedAccount(acc); setShowManageDrawer(false); setShowPanel(true) }}
          onSync={id => syncMutation.mutate(id)}
          onDelete={id => deleteMutation.mutate(id)}
          onAdd={() => { setSelectedAccount(null); setShowManageDrawer(false); setShowPanel(true) }}
          onClose={() => setShowManageDrawer(false)}
        />
      )}
    </div>
  )
}
