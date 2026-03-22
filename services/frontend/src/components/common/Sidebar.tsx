import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard, Cloud, MessageSquare, CreditCard, Settings, X, Zap, BookOpen,
} from 'lucide-react'
import Logo from './Logo'

interface SidebarProps {
  isOpen: boolean
  collapsed: boolean
  onClose: () => void
}

const navItems = [
  { to: '/dashboard',   icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/finops',      icon: Cloud,           label: 'FinOps Analytics' },
  { to: '/query',       icon: MessageSquare,   label: 'AI Query Tool' },
  { to: '/billing',     icon: CreditCard,      label: 'Billing' },
  { to: '/billing/ubb', icon: Zap,             label: 'Usage Billing' },
  { to: '/settings',    icon: Settings,        label: 'Settings' },
]

const bottomItems = [
  { to: '/docs', icon: BookOpen, label: 'Documentation' },
]

export default function Sidebar({ isOpen, collapsed, onClose }: SidebarProps) {
  return (
    <>
      {/* Mobile overlay */}
      {isOpen && (
        <div className="fixed inset-0 z-20 bg-black/50 md:hidden" onClick={onClose} />
      )}

      <aside
        className={`
          fixed inset-y-0 left-0 z-20 flex flex-col bg-white shadow-lg
          transition-all duration-300 ease-in-out
          dark:bg-gray-800
          md:static md:shadow-none md:translate-x-0
          ${isOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}
          ${collapsed ? 'md:w-[60px]' : 'md:w-64'}
          w-64
        `}
      >
        {/* Header */}
        <div className={`flex h-16 flex-shrink-0 items-center border-b border-gray-200 dark:border-gray-700 transition-all duration-300 ${collapsed ? 'md:justify-center px-0' : 'justify-between px-4'}`}>
          <div className={`transition-all duration-300 overflow-hidden ${collapsed ? 'md:hidden' : ''}`}>
            <Logo size={28} />
          </div>
          {/* Collapsed: show small logo mark */}
          {collapsed && (
            <div className="hidden md:flex items-center justify-center">
              <Logo size={22} iconOnly />
            </div>
          )}
          <button
            onClick={onClose}
            className="rounded-md p-1 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 md:hidden"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Nav */}
        <nav className="flex flex-1 flex-col overflow-y-auto overflow-x-hidden py-4">
          <ul className={`flex-1 space-y-1 ${collapsed ? 'md:px-2' : 'px-3'}`}>
            {navItems.map(({ to, icon: Icon, label }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  onClick={onClose}
                  title={collapsed ? label : undefined}
                  className={({ isActive }) =>
                    `flex items-center gap-3 rounded-lg py-2 text-sm font-medium transition-colors
                    ${collapsed ? 'md:justify-center md:px-0 px-3' : 'px-3'}
                    ${isActive
                      ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400'
                      : 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'
                    }`
                  }
                >
                  <Icon className="h-5 w-5 flex-shrink-0" />
                  <span className={`transition-all duration-300 overflow-hidden whitespace-nowrap ${collapsed ? 'md:hidden' : ''}`}>
                    {label}
                  </span>
                </NavLink>
              </li>
            ))}
          </ul>

          {/* Bottom pinned items */}
          <ul className={`mt-auto border-t border-gray-100 dark:border-gray-700 pt-3 space-y-1 ${collapsed ? 'md:px-2' : 'px-3'}`}>
            {bottomItems.map(({ to, icon: Icon, label }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  onClick={onClose}
                  title={collapsed ? label : undefined}
                  className={({ isActive }) =>
                    `flex items-center gap-3 rounded-lg py-2 text-sm font-medium transition-colors
                    ${collapsed ? 'md:justify-center md:px-0 px-3' : 'px-3'}
                    ${isActive
                      ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400'
                      : 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'
                    }`
                  }
                >
                  <Icon className="h-5 w-5 flex-shrink-0" />
                  <span className={`transition-all duration-300 overflow-hidden whitespace-nowrap ${collapsed ? 'md:hidden' : ''}`}>
                    {label}
                  </span>
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
      </aside>
    </>
  )
}
