import { useState, useRef, useEffect } from 'react'
import {
  Bell, Moon, Sun, LogOut, ChevronDown, User, Key, Eye, EyeOff,
  Copy, Check, Plus, Trash2, Clock, Mail, Shield, X, Zap,
} from 'lucide-react'
import { useTheme } from '../../context/ThemeContext'
import { useAuth } from '../../context/AuthContext'
import { authService } from '../../services/auth.service'
import api from '../../services/api'
import Logo from './Logo'

interface NavbarProps {
  onMenuToggle?: () => void
}

function getInitials(name?: string): string {
  if (!name?.trim()) return 'U'
  const parts = name.trim().split(/\s+/)
  return parts.length >= 2
    ? (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    : parts[0].slice(0, 2).toUpperCase()
}

function getAvatarGradient(seed: string): string {
  const palettes = [
    ['#6366f1', '#8b5cf6'],
    ['#3b82f6', '#06b6d4'],
    ['#10b981', '#14b8a6'],
    ['#f59e0b', '#ef4444'],
    ['#ec4899', '#f43f5e'],
    ['#8b5cf6', '#6366f1'],
  ]
  let h = 0
  for (let i = 0; i < seed.length; i++) h = seed.charCodeAt(i) + ((h << 5) - h)
  const [a, b] = palettes[Math.abs(h) % palettes.length]
  return `linear-gradient(135deg, ${a}, ${b})`
}

function useNow() {
  const [now, setNow] = useState(new Date())
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(t)
  }, [])
  return now
}

// ── Notifications Panel ──────────────────────────────────────────────────────

const MOCK_NOTIFICATIONS = [
  { id: '1', icon: Zap, color: 'text-indigo-500', bg: 'bg-indigo-50 dark:bg-indigo-900/30', title: 'Usage spike detected', body: 'Stream "api-calls" exceeded 10k units today.', time: '2m ago', unread: true },
  { id: '2', icon: Shield, color: 'text-emerald-500', bg: 'bg-emerald-50 dark:bg-emerald-900/30', title: 'Login from new device', body: 'Chrome on Windows — if this wasn\'t you, reset your password.', time: '1h ago', unread: true },
  { id: '3', icon: Mail, color: 'text-blue-500', bg: 'bg-blue-50 dark:bg-blue-900/30', title: 'Invoice generated', body: 'Your June invoice of $42.00 is ready.', time: '3h ago', unread: false },
  { id: '4', icon: Key, color: 'text-amber-500', bg: 'bg-amber-50 dark:bg-amber-900/30', title: 'API key expiring soon', body: 'Key "prod-key" expires in 7 days.', time: '1d ago', unread: false },
]

function NotificationsPanel({ onClose }: { onClose: () => void }) {
  const [items, setItems] = useState(MOCK_NOTIFICATIONS)
  const unreadCount = items.filter(n => n.unread).length

  const markAll = () => setItems(prev => prev.map(n => ({ ...n, unread: false })))
  const dismiss = (id: string) => setItems(prev => prev.filter(n => n.id !== id))

  return (
    <div className="absolute right-0 top-full mt-2.5 w-80 rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-xl shadow-black/10 dark:shadow-black/40 overflow-hidden z-50">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100 dark:border-gray-700">
        <div className="flex items-center gap-2">
          <Bell className="h-4 w-4 text-indigo-500" />
          <span className="text-sm font-bold text-gray-900 dark:text-white">Notifications</span>
          {unreadCount > 0 && (
            <span className="rounded-full bg-indigo-500 px-1.5 py-0.5 text-[10px] font-bold text-white">{unreadCount}</span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {unreadCount > 0 && (
            <button onClick={markAll} className="text-[11px] text-indigo-500 hover:text-indigo-700 font-medium px-1">
              Mark all read
            </button>
          )}
          <button onClick={onClose} className="rounded-lg p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700">
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* List */}
      <div className="max-h-80 overflow-y-auto divide-y divide-gray-50 dark:divide-gray-700/50">
        {items.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-10 text-gray-400">
            <Bell className="h-8 w-8 mb-2 opacity-30" />
            <p className="text-sm">All caught up</p>
          </div>
        ) : items.map(n => {
          const Icon = n.icon
          return (
            <div key={n.id} className={`group flex items-start gap-3 px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors ${n.unread ? 'bg-indigo-50/30 dark:bg-indigo-900/10' : ''}`}>
              <div className={`mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-xl ${n.bg}`}>
                <Icon className={`h-4 w-4 ${n.color}`} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-1">
                  <p className={`text-xs font-semibold leading-tight ${n.unread ? 'text-gray-900 dark:text-white' : 'text-gray-600 dark:text-gray-300'}`}>
                    {n.title}
                    {n.unread && <span className="ml-1.5 inline-block h-1.5 w-1.5 rounded-full bg-indigo-500 align-middle" />}
                  </p>
                  <button onClick={() => dismiss(n.id)} className="opacity-0 group-hover:opacity-100 flex-shrink-0 rounded p-0.5 text-gray-300 hover:text-gray-500 transition-opacity">
                    <X className="h-3 w-3" />
                  </button>
                </div>
                <p className="mt-0.5 text-[11px] text-gray-400 dark:text-gray-500 leading-snug">{n.body}</p>
                <p className="mt-1 text-[10px] text-gray-300 dark:text-gray-600">{n.time}</p>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ── API Key Row ──────────────────────────────────────────────────────────────

interface ApiKey { id: string; name: string; key_prefix: string; created_at: string; expires_at: string }

function ApiKeyRow({ k, onRevoke }: { k: ApiKey; onRevoke: (id: string) => void }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(k.key_prefix + '••••••••••••••••••••••••')
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <div className="flex items-center justify-between gap-2 rounded-xl border border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/40 px-3 py-2">
      <div className="min-w-0 flex-1">
        <p className="text-xs font-semibold text-gray-800 dark:text-gray-200 truncate">{k.name}</p>
        <p className="text-[10px] font-mono text-gray-400 dark:text-gray-500">{k.key_prefix}••••••••</p>
      </div>
      <div className="flex items-center gap-1">
        <button onClick={copy} className="rounded-lg p-1.5 text-gray-400 hover:text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 transition-colors">
          {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
        </button>
        <button onClick={() => onRevoke(k.id)} className="rounded-lg p-1.5 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors">
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  )
}

// ── User Dropdown ────────────────────────────────────────────────────────────

function UserDropdown({ user, avatarGradient, initials, fullName, roleLabel, onClose, onLogout }: {
  user: { email: string; role: string; name?: string; avatar?: string } | null
  avatarGradient: string
  initials: string
  fullName: string
  roleLabel: string
  onClose: () => void
  onLogout: () => void
}) {
  const now = useNow()
  const [tab, setTab] = useState<'profile' | 'apikeys'>('profile')
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([])
  const [loadingKeys, setLoadingKeys] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [showKey, setShowKey] = useState(false)
  const [creating, setCreating] = useState(false)
  const [copiedKey, setCopiedKey] = useState(false)

  const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  const dateStr = now.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })

  const loadKeys = async () => {
    setLoadingKeys(true)
    try {
      const res = await api.get('/api/auth/api-keys')
      setApiKeys(res.data.api_keys ?? [])
    } catch { /* ignore */ }
    finally { setLoadingKeys(false) }
  }

  useEffect(() => {
    if (tab === 'apikeys') loadKeys()
  }, [tab])

  const createKey = async () => {
    if (!newKeyName.trim()) return
    setCreating(true)
    try {
      const res = await api.post('/api/auth/api-keys', { name: newKeyName.trim() })
      setCreatedKey(res.data.key)
      setShowKey(true)
      setNewKeyName('')
      loadKeys()
    } catch { /* ignore */ }
    finally { setCreating(false) }
  }

  const revokeKey = async (id: string) => {
    try {
      await api.delete(`/api/auth/api-keys/${id}`)
      setApiKeys(prev => prev.filter(k => k.id !== id))
    } catch { /* ignore */ }
  }

  const copyCreatedKey = () => {
    if (createdKey) {
      navigator.clipboard.writeText(createdKey)
      setCopiedKey(true)
      setTimeout(() => setCopiedKey(false), 1500)
    }
  }

  return (
    <div className="absolute right-0 top-full mt-2.5 w-72 rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-xl shadow-black/10 dark:shadow-black/40 overflow-hidden z-50">

      {/* ── Hero banner ── */}
      <div className="relative px-4 pt-4 pb-3 overflow-hidden"
        style={{ background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #06b6d4 100%)' }}>
        {/* decorative circles */}
        <div className="absolute -top-4 -right-4 h-20 w-20 rounded-full bg-white/10" />
        <div className="absolute -bottom-6 -left-4 h-16 w-16 rounded-full bg-white/10" />

        <div className="relative flex items-center gap-3">
          {user?.avatar ? (
            <img src={user.avatar} alt={fullName} className="h-12 w-12 rounded-2xl object-cover ring-2 ring-white/40 flex-shrink-0" />
          ) : (
            <div
              className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-2xl text-base font-bold text-white ring-2 ring-white/40 select-none"
              style={{ background: avatarGradient }}
            >
              {initials}
            </div>
          )}
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-bold text-white leading-tight">
              {fullName || 'No name set'}
            </p>
            <p className="mt-0.5 truncate text-[11px] text-white/70 leading-tight">{user?.email}</p>
            <span className="mt-1.5 inline-flex items-center gap-1 rounded-full bg-white/20 px-2 py-0.5">
              <Shield className="h-2.5 w-2.5 text-white/80" />
              <span className="text-[9px] font-bold uppercase tracking-widest text-white/90">{roleLabel}</span>
            </span>
          </div>
        </div>

        {/* Live clock */}
        <div className="relative mt-3 flex items-center gap-2 rounded-xl bg-white/10 px-3 py-2">
          <Clock className="h-3.5 w-3.5 text-white/70 flex-shrink-0" />
          <div>
            <p className="text-xs font-bold text-white tabular-nums">{timeStr}</p>
            <p className="text-[10px] text-white/60">{dateStr}</p>
          </div>
        </div>
      </div>

      {/* ── Tabs ── */}
      <div className="flex border-b border-gray-100 dark:border-gray-700">
        {(['profile', 'apikeys'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex-1 py-2.5 text-xs font-semibold transition-colors ${
              tab === t
                ? 'border-b-2 border-indigo-500 text-indigo-600 dark:text-indigo-400'
                : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
            }`}
          >
            {t === 'profile' ? (
              <span className="flex items-center justify-center gap-1.5"><User className="h-3.5 w-3.5" />Profile</span>
            ) : (
              <span className="flex items-center justify-center gap-1.5"><Key className="h-3.5 w-3.5" />API Keys</span>
            )}
          </button>
        ))}
      </div>

      {/* ── Tab content ── */}
      {tab === 'profile' && (
        <div className="px-4 py-3 space-y-2.5">
          {/* Email row */}
          <div className="flex items-center gap-2.5 rounded-xl bg-gray-50 dark:bg-gray-700/40 px-3 py-2.5">
            <Mail className="h-4 w-4 text-indigo-400 flex-shrink-0" />
            <div className="min-w-0">
              <p className="text-[10px] text-gray-400 dark:text-gray-500 uppercase tracking-wide font-medium">Email</p>
              <p className="text-xs font-semibold text-gray-800 dark:text-gray-200 truncate">{user?.email}</p>
            </div>
          </div>
          {/* Role row */}
          <div className="flex items-center gap-2.5 rounded-xl bg-gray-50 dark:bg-gray-700/40 px-3 py-2.5">
            <Shield className="h-4 w-4 text-emerald-400 flex-shrink-0" />
            <div>
              <p className="text-[10px] text-gray-400 dark:text-gray-500 uppercase tracking-wide font-medium">Role</p>
              <p className="text-xs font-semibold text-gray-800 dark:text-gray-200 capitalize">{roleLabel}</p>
            </div>
          </div>
        </div>
      )}

      {tab === 'apikeys' && (
        <div className="px-4 py-3 space-y-3">
          {/* New key created banner */}
          {createdKey && (
            <div className="rounded-xl border border-emerald-200 dark:border-emerald-700 bg-emerald-50 dark:bg-emerald-900/20 p-3">
              <p className="text-[10px] font-bold text-emerald-700 dark:text-emerald-400 uppercase tracking-wide mb-1.5">
                Copy now — shown once
              </p>
              <div className="flex items-center gap-2">
                <code className="flex-1 min-w-0 truncate rounded-lg bg-white dark:bg-gray-800 border border-emerald-200 dark:border-emerald-700 px-2 py-1.5 text-[11px] font-mono text-gray-700 dark:text-gray-300">
                  {showKey ? createdKey : '••••••••••••••••••••••••••••••••'}
                </code>
                <button onClick={() => setShowKey(v => !v)} className="flex-shrink-0 rounded-lg p-1.5 text-gray-400 hover:text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 transition-colors">
                  {showKey ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                </button>
                <button onClick={copyCreatedKey} className="flex-shrink-0 rounded-lg p-1.5 text-gray-400 hover:text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/30 transition-colors">
                  {copiedKey ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                </button>
                <button onClick={() => setCreatedKey(null)} className="flex-shrink-0 rounded-lg p-1.5 text-gray-400 hover:text-red-400 transition-colors">
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          )}

          {/* Create new key */}
          <div className="flex gap-2">
            <input
              value={newKeyName}
              onChange={e => setNewKeyName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && createKey()}
              placeholder="Key name…"
              className="flex-1 min-w-0 rounded-xl border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-700/40 px-3 py-2 text-xs text-gray-800 dark:text-gray-200 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-400"
            />
            <button
              onClick={createKey}
              disabled={creating || !newKeyName.trim()}
              className="flex-shrink-0 flex items-center gap-1 rounded-xl bg-indigo-500 hover:bg-indigo-600 disabled:opacity-50 px-3 py-2 text-xs font-semibold text-white transition-colors"
            >
              <Plus className="h-3.5 w-3.5" />
              {creating ? '…' : 'Create'}
            </button>
          </div>

          {/* Keys list */}
          <div className="space-y-1.5 max-h-40 overflow-y-auto">
            {loadingKeys ? (
              <p className="text-center text-xs text-gray-400 py-4">Loading…</p>
            ) : apiKeys.length === 0 ? (
              <p className="text-center text-xs text-gray-400 py-4">No API keys yet</p>
            ) : apiKeys.map(k => (
              <ApiKeyRow key={k.id} k={k} onRevoke={revokeKey} />
            ))}
          </div>
        </div>
      )}

      <div className="h-px bg-gray-100 dark:bg-gray-700" />

      {/* Sign out */}
      <button
        onClick={onLogout}
        className="flex w-full items-center gap-3 px-4 py-3 text-sm font-medium text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
      >
        <LogOut className="h-4 w-4" />
        Sign out
      </button>
    </div>
  )
}

// ── Navbar ───────────────────────────────────────────────────────────────────

export default function Navbar({ onMenuToggle }: NavbarProps) {
  const { theme, toggleTheme } = useTheme()
  const { user, logout } = useAuth()
  const [dropOpen, setDropOpen] = useState(false)
  const [notifOpen, setNotifOpen] = useState(false)
  const dropRef = useRef<HTMLDivElement>(null)
  const notifRef = useRef<HTMLDivElement>(null)

  const handleLogout = async () => {
    await authService.logout()
    logout()
  }

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) setDropOpen(false)
      if (notifRef.current && !notifRef.current.contains(e.target as Node)) setNotifOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const fullName = user?.name?.trim() || ''
  const initials = getInitials(fullName || user?.email)
  const avatarGradient = getAvatarGradient(user?.email ?? 'user')
  const roleLabel = user?.role ?? 'user'

  const unreadCount = MOCK_NOTIFICATIONS.filter(n => n.unread).length

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-gray-200 bg-white/95 backdrop-blur px-4 dark:border-gray-700/60 dark:bg-gray-900/95 md:px-6">

      {/* Left */}
      <div className="flex items-center gap-3">
        <button
          onClick={onMenuToggle}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 md:hidden transition-colors"
          aria-label="Toggle menu"
        >
          <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span className="hidden md:block">
          <Logo size={28} />
        </span>
      </div>

      {/* Right */}
      <div className="flex items-center gap-1">

        {/* Theme toggle */}
        <button
          onClick={toggleTheme}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          aria-label="Toggle theme"
        >
          {theme === 'dark' ? <Sun className="h-[18px] w-[18px]" /> : <Moon className="h-[18px] w-[18px]" />}
        </button>

        {/* Notifications */}
        <div ref={notifRef} className="relative">
          <button
            onClick={() => { setNotifOpen(v => !v); setDropOpen(false) }}
            className="relative rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            aria-label="Notifications"
          >
            <Bell className="h-[18px] w-[18px]" />
            {unreadCount > 0 && (
              <span className="absolute right-1.5 top-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-indigo-500 text-[9px] font-bold text-white ring-2 ring-white dark:ring-gray-900">
                {unreadCount}
              </span>
            )}
          </button>
          {notifOpen && <NotificationsPanel onClose={() => setNotifOpen(false)} />}
        </div>

        <div className="mx-2 h-5 w-px bg-gray-200 dark:bg-gray-700" />

        {/* User pill */}
        <div ref={dropRef} className="relative">
          <button
            onClick={() => { setDropOpen(v => !v); setNotifOpen(false) }}
            className="group flex items-center gap-2 rounded-full border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 pl-1 pr-3 py-1 hover:border-indigo-300 dark:hover:border-indigo-600 hover:bg-white dark:hover:bg-gray-750 transition-all duration-150 shadow-sm"
          >
            {user?.avatar ? (
              <img src={user.avatar} alt={fullName} className="h-7 w-7 rounded-full object-cover ring-2 ring-white dark:ring-gray-800" />
            ) : (
              <div
                className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full text-[11px] font-bold text-white ring-2 ring-white dark:ring-gray-800 select-none"
                style={{ background: avatarGradient }}
              >
                {initials}
              </div>
            )}
            <ChevronDown className={`h-3.5 w-3.5 text-gray-400 transition-transform duration-200 ${dropOpen ? 'rotate-180' : ''}`} />
          </button>

          {dropOpen && (
            <UserDropdown
              user={user}
              avatarGradient={avatarGradient}
              initials={initials}
              fullName={fullName}
              roleLabel={roleLabel}
              onClose={() => setDropOpen(false)}
              onLogout={handleLogout}
            />
          )}
        </div>
      </div>
    </header>
  )
}
