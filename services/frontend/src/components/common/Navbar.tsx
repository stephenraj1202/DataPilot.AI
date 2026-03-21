import { useState, useRef, useEffect } from 'react'
import { Bell, Moon, Sun, LogOut, ChevronDown, User } from 'lucide-react'
import { useTheme } from '../../context/ThemeContext'
import { useAuth } from '../../context/AuthContext'
import { authService } from '../../services/auth.service'
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

// Deterministic gradient from name string
function getAvatarGradient(seed: string): string {
  const palettes = [
    ['#6366f1', '#8b5cf6'], // indigo → violet
    ['#3b82f6', '#06b6d4'], // blue → cyan
    ['#10b981', '#14b8a6'], // emerald → teal
    ['#f59e0b', '#ef4444'], // amber → red
    ['#ec4899', '#f43f5e'], // pink → rose
    ['#8b5cf6', '#6366f1'], // violet → indigo
  ]
  let h = 0
  for (let i = 0; i < seed.length; i++) h = seed.charCodeAt(i) + ((h << 5) - h)
  const [a, b] = palettes[Math.abs(h) % palettes.length]
  return `linear-gradient(135deg, ${a}, ${b})`
}

export default function Navbar({ onMenuToggle }: NavbarProps) {
  const { theme, toggleTheme } = useTheme()
  const { user, logout } = useAuth()
  const [dropOpen, setDropOpen] = useState(false)
  const dropRef = useRef<HTMLDivElement>(null)

  const handleLogout = async () => {
    await authService.logout()
    logout()
  }

  useEffect(() => {
    function handler(e: MouseEvent) {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) setDropOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  // Full name from JWT — fall back gracefully
  const fullName = user?.name?.trim() || ''
  const initials = getInitials(fullName || user?.email)
  const avatarGradient = getAvatarGradient(user?.email ?? 'user')
  const roleLabel = user?.role ?? 'user'

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-gray-200 bg-white/95 backdrop-blur px-4 dark:border-gray-700/60 dark:bg-gray-900/95 md:px-6">

      {/* Left — hamburger + brand */}
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
        <button
          className="relative rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          aria-label="Notifications"
        >
          <Bell className="h-[18px] w-[18px]" />
          <span className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-indigo-500 ring-2 ring-white dark:ring-gray-900" />
        </button>

        <div className="mx-2 h-5 w-px bg-gray-200 dark:bg-gray-700" />

        {/* ── User pill ── */}
        <div ref={dropRef} className="relative">
          <button
            onClick={() => setDropOpen(v => !v)}
            className="group flex items-center gap-2 rounded-full border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 pl-1 pr-3 py-1 hover:border-indigo-300 dark:hover:border-indigo-600 hover:bg-white dark:hover:bg-gray-750 transition-all duration-150 shadow-sm"
          >
            {/* Avatar only — no name text in the pill */}
            {user?.avatar ? (
              <img
                src={user.avatar}
                alt={fullName}
                className="h-7 w-7 rounded-full object-cover ring-2 ring-white dark:ring-gray-800"
              />
            ) : (
              <div
                className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full text-[11px] font-bold text-white ring-2 ring-white dark:ring-gray-800 select-none"
                style={{ background: avatarGradient }}
              >
                {initials}
              </div>
            )}

            <ChevronDown
              className={`h-3.5 w-3.5 text-gray-400 transition-transform duration-200 ${dropOpen ? 'rotate-180' : ''}`}
            />
          </button>

          {/* ── Dropdown ── */}
          {dropOpen && (
            <div className="absolute right-0 top-full mt-2.5 w-60 rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-xl shadow-black/10 dark:shadow-black/40 overflow-hidden z-50">

              {/* Identity block */}
              <div className="px-4 pt-4 pb-3">
                <div className="flex items-center gap-3">
                  {user?.avatar ? (
                    <img src={user.avatar} alt={fullName} className="h-10 w-10 rounded-full object-cover flex-shrink-0" />
                  ) : (
                    <div
                      className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full text-sm font-bold text-white select-none"
                      style={{ background: avatarGradient }}
                    >
                      {initials}
                    </div>
                  )}
                  <div className="min-w-0 flex-1">
                    {fullName ? (
                      <p className="truncate text-sm font-bold text-gray-900 dark:text-white leading-tight">
                        {fullName}
                      </p>
                    ) : (
                      <p className="text-sm font-bold text-gray-400 dark:text-gray-500 leading-tight">No name set</p>
                    )}
                    <p className="mt-0.5 truncate text-[11px] text-gray-400 dark:text-gray-500 leading-tight">
                      {user?.email}
                    </p>
                  </div>
                </div>

                {/* Role badge */}
                <div className="mt-3 flex items-center gap-1.5">
                  <User className="h-3 w-3 text-indigo-400" />
                  <span className="text-[10px] font-bold uppercase tracking-widest text-indigo-500 dark:text-indigo-400">
                    {roleLabel}
                  </span>
                </div>
              </div>

              <div className="h-px bg-gray-100 dark:bg-gray-700" />

              {/* Sign out */}
              <button
                onClick={handleLogout}
                className="flex w-full items-center gap-3 px-4 py-3 text-sm font-medium text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
              >
                <LogOut className="h-4 w-4" />
                Sign out
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
