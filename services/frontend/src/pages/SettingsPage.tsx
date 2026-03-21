import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { queryService } from '../services/query.service'
import api from '../services/api'
import { useAuth } from '../context/AuthContext'
import toast from 'react-hot-toast'
import { Plus, Trash2, Eye, EyeOff, Pencil, X, Copy, Check, Key, BookOpen, Terminal } from 'lucide-react'
import LoadingSpinner from '../components/common/LoadingSpinner'

type Section = 'profile' | 'team' | 'api-keys' | 'docs' | 'smtp' | 'databases' | 'notifications' | 'customize'

const sections: { id: Section; label: string }[] = [
  { id: 'profile', label: 'Profile' },
  { id: 'team', label: 'Team Members' },
  { id: 'api-keys', label: 'API Keys' },
  { id: 'docs', label: 'API Docs' },
  { id: 'customize', label: 'Customize' },
  { id: 'smtp', label: 'SMTP Config' },
  { id: 'databases', label: 'Database Connections' },
  { id: 'notifications', label: 'Notifications' },
]

export default function SettingsPage() {
  const [activeSection, setActiveSection] = useState<Section>('profile')
  const { user } = useAuth()
  const qc = useQueryClient()

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Settings</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Manage your account and preferences</p>
      </div>

      <div className="flex flex-col gap-6 lg:flex-row">
        {/* Sidebar nav */}
        <nav className="flex flex-row gap-1 overflow-x-auto lg:w-48 lg:flex-col lg:overflow-visible">
          {sections.map(s => (
            <button
              key={s.id}
              onClick={() => setActiveSection(s.id)}
              className={`flex-shrink-0 rounded-lg px-3 py-2 text-sm font-medium text-left transition-colors ${
                activeSection === s.id
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                  : 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'
              }`}
            >
              {s.label}
            </button>
          ))}
        </nav>

        {/* Content */}
        <div className="flex-1">
          {activeSection === 'profile' && <ProfileSection user={user} />}
          {activeSection === 'team' && <TeamSection />}
          {activeSection === 'api-keys' && <ApiKeysSection />}
          {activeSection === 'docs' && <DocumentationSection />}
          {activeSection === 'customize' && <CustomizeSection />}
          {activeSection === 'smtp' && <SmtpSection />}
          {activeSection === 'databases' && <DatabasesSection qc={qc} />}
          {activeSection === 'notifications' && <NotificationsSection />}
        </div>
      </div>
    </div>
  )
}

function ProfileSection({ user }: { user: { email: string; role: string } | null }) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
      <h2 className="mb-4 text-base font-semibold text-gray-900 dark:text-white">Profile</h2>
      <div className="space-y-4">
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Email</label>
          <input
            type="email"
            defaultValue={user?.email ?? ''}
            readOnly
            className="w-full rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Role</label>
          <input
            type="text"
            defaultValue={user?.role ?? ''}
            readOnly
            className="w-full rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm capitalize dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
      </div>
    </div>
  )
}

function TeamSection() {
  const { data, isLoading } = useQuery({
    queryKey: ['team-members'],
    queryFn: async () => {
      const { data } = await api.get('/api/auth/users')
      return data
    },
  })

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
      <h2 className="mb-4 text-base font-semibold text-gray-900 dark:text-white">Team Members</h2>
      {isLoading ? (
        <LoadingSpinner size="sm" />
      ) : (
        <div className="space-y-2">
          {(data?.users ?? []).map((u: { id: string; email: string; role: string }) => (
            <div key={u.id} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 dark:bg-gray-700/50">
              <span className="text-sm text-gray-700 dark:text-gray-300">{u.email}</span>
              <span className="text-xs capitalize text-gray-500">{u.role}</span>
            </div>
          ))}
          {(!data?.users || data.users.length === 0) && (
            <p className="text-sm text-gray-500">No team members found.</p>
          )}
        </div>
      )}
    </div>
  )
}

function CopyButton({ text, className = '' }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    toast.success('Copied!')
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <button onClick={copy} className={`flex items-center gap-1 rounded px-2 py-1 text-xs transition-colors hover:bg-gray-100 dark:hover:bg-gray-700 ${className}`}>
      {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5 text-gray-400" />}
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

function ApiKeysSection() {
  const [newKeyName, setNewKeyName] = useState('')
  const [createdKey, setCreatedKey] = useState<{ id: string; name: string; key: string } | null>(null)
  const [revealed, setRevealed] = useState(false)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['api-keys'],
    queryFn: async () => {
      const { data } = await api.get('/api/auth/api-keys')
      return data
    },
  })

  const createKey = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const { data: res } = await api.post('/api/auth/api-keys', { name: newKeyName, expires_in_days: 365 })
      setCreatedKey({ id: res.id, name: res.name, key: res.key ?? res.api_key })
      setRevealed(false)
      setNewKeyName('')
      refetch()
    } catch {
      toast.error('Failed to create API key')
    }
  }

  const revokeKey = async (id: string) => {
    try {
      await api.delete(`/api/auth/api-keys/${id}`)
      refetch()
      toast.success('API key revoked')
    } catch {
      toast.error('Failed to revoke key')
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
        <div className="mb-4 flex items-center gap-2">
          <Key className="h-5 w-5 text-indigo-500" />
          <h2 className="text-base font-semibold text-gray-900 dark:text-white">API Keys</h2>
        </div>
        <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
          Use API keys to authenticate requests from your applications. Keys are shown only once on creation.
        </p>

        {/* Create form */}
        <form onSubmit={createKey} className="mb-5 flex gap-2">
          <input
            type="text"
            value={newKeyName}
            onChange={e => setNewKeyName(e.target.value)}
            placeholder="Key name (e.g. production-app)"
            required
            className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
          <button type="submit" className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700">
            <Plus className="h-4 w-4" /> Generate Key
          </button>
        </form>

        {/* Newly created key banner */}
        {createdKey && (
          <div className="mb-5 rounded-xl border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-semibold text-green-700 dark:text-green-400">
                "{createdKey.name}" — Save this key now, it won't be shown again
              </p>
              <button onClick={() => setCreatedKey(null)} className="text-green-500 hover:text-green-700">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="flex items-center gap-2 rounded-lg bg-white px-3 py-2 dark:bg-gray-900">
              <code className="flex-1 break-all font-mono text-sm text-gray-800 dark:text-gray-200">
                {revealed ? createdKey.key : createdKey.key.slice(0, 8) + '••••••••••••••••••••••••••••••••••••••••••••••••••••••••'}
              </code>
              <button onClick={() => setRevealed(r => !r)} className="shrink-0 text-gray-400 hover:text-gray-600">
                {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
              <CopyButton text={createdKey.key} />
            </div>
          </div>
        )}

        {/* Keys list */}
        {isLoading ? (
          <LoadingSpinner size="sm" />
        ) : (
          <div className="space-y-2">
            {(data?.api_keys ?? []).length === 0 && (
              <p className="text-sm text-gray-400">No API keys yet. Generate one above.</p>
            )}
            {(data?.api_keys ?? []).map((k: { id: string; name: string; key_prefix: string; created_at: string; expires_at: string; last_used_at?: string }) => (
              <div key={k.id} className="flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-700/40">
                <div className="min-w-0">
                  <p className="text-sm font-medium text-gray-800 dark:text-gray-200">{k.name}</p>
                  <div className="mt-0.5 flex flex-wrap gap-3 text-xs text-gray-400">
                    <span>Prefix: <code className="font-mono">{k.key_prefix}...</code></span>
                    <span>Created: {new Date(k.created_at).toLocaleDateString()}</span>
                    <span>Expires: {new Date(k.expires_at).toLocaleDateString()}</span>
                    {k.last_used_at && <span>Last used: {new Date(k.last_used_at).toLocaleDateString()}</span>}
                  </div>
                </div>
                <button onClick={() => revokeKey(k.id)} className="ml-3 shrink-0 rounded-lg p-1.5 text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20">
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function CustomizeSection() {
  const [logoFile, setLogoFile] = useState<File | null>(null)
  const [logoPreview, setLogoPreview] = useState<string | null>(
    () => localStorage.getItem('custom_logo')
  )
  const [appName, setAppName] = useState(() => localStorage.getItem('custom_app_name') || 'DataPilot.AI')
  const [tagline, setTagline] = useState(() => localStorage.getItem('custom_tagline') || 'FinOps Platform')
  const [saved, setSaved] = useState(false)

  const handleLogoChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setLogoFile(file)
    const reader = new FileReader()
    reader.onload = ev => setLogoPreview(ev.target?.result as string)
    reader.readAsDataURL(file)
  }

  const handleSave = () => {
    if (logoPreview) localStorage.setItem('custom_logo', logoPreview)
    localStorage.setItem('custom_app_name', appName)
    localStorage.setItem('custom_tagline', tagline)
    setSaved(true)
    toast.success('Branding saved — reload to see changes everywhere')
    setTimeout(() => setSaved(false), 3000)
  }

  const handleReset = () => {
    localStorage.removeItem('custom_logo')
    localStorage.removeItem('custom_app_name')
    localStorage.removeItem('custom_tagline')
    setLogoPreview(null)
    setLogoFile(null)
    setAppName('DataPilot.AI')
    setTagline('FinOps Platform')
    toast.success('Reset to default branding')
  }

  return (
    <div className="space-y-5">
      <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
        <h2 className="mb-1 text-base font-semibold text-gray-900 dark:text-white">Customize Branding</h2>
        <p className="mb-5 text-sm text-gray-500 dark:text-gray-400">Upload your company logo and set the app name shown to your team.</p>

        {/* Logo upload */}
        <div className="mb-5">
          <label className="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">Company Logo</label>
          <div className="flex items-center gap-4">
            <div className="flex h-16 w-16 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-gray-600 dark:bg-gray-700">
              {logoPreview
                ? <img src={logoPreview} alt="Logo preview" className="h-full w-full object-contain p-1" />
                : <span className="text-2xl font-black text-indigo-500">D</span>
              }
            </div>
            <div>
              <label className="cursor-pointer rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600">
                {logoFile ? logoFile.name : 'Upload Logo'}
                <input type="file" accept="image/*" className="hidden" onChange={handleLogoChange} />
              </label>
              <p className="mt-1 text-xs text-gray-400">PNG, SVG, JPG · Recommended 200×200px</p>
            </div>
          </div>
        </div>

        {/* App name */}
        <div className="mb-4">
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">App Name</label>
          <input
            type="text"
            value={appName}
            onChange={e => setAppName(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>

        {/* Tagline */}
        <div className="mb-6">
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Tagline / Sub-label</label>
          <input
            type="text"
            value={tagline}
            onChange={e => setTagline(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>

        {/* Preview */}
        <div className="mb-6 rounded-xl border border-gray-100 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-900">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-400">Preview</p>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center overflow-hidden rounded-xl bg-indigo-600">
              {logoPreview
                ? <img src={logoPreview} alt="logo" className="h-full w-full object-contain" />
                : <span className="text-lg font-black text-white">{appName.charAt(0)}</span>
              }
            </div>
            <div className="leading-none">
              <p className="text-sm font-black text-gray-900 dark:text-white">{appName}</p>
              <p className="text-[10px] font-semibold uppercase tracking-widest text-gray-400">{tagline}</p>
            </div>
          </div>
        </div>

        <div className="flex gap-3">
          <button
            onClick={handleSave}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          >
            {saved ? <Check className="h-4 w-4" /> : null}
            Save Branding
          </button>
          <button
            onClick={handleReset}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
          >
            Reset to Default
          </button>
        </div>
      </div>
    </div>
  )
}

function DocumentationSection() {  const BASE_URL = window.location.origin.replace('3000', '8080').replace('5173', '8080')

  const snippets: { title: string; description: string; curl: string }[] = [
    {
      title: 'Authentication',
      description: 'All API requests require an API key passed in the X-API-Key header.',
      curl: `curl -X GET "${BASE_URL}/api/finops/cloud-accounts" \\
  -H "X-API-Key: YOUR_API_KEY" \\
  -H "Content-Type: application/json"`,
    },
    {
      title: 'List Cloud Accounts',
      description: 'Retrieve all connected cloud accounts for your organization.',
      curl: `curl -X GET "${BASE_URL}/api/finops/cloud-accounts" \\
  -H "X-API-Key: YOUR_API_KEY"`,
    },
    {
      title: 'Get Cost Summary',
      description: 'Get aggregated cloud cost data for a date range.',
      curl: `curl -X GET "${BASE_URL}/api/finops/costs?start_date=2024-01-01&end_date=2024-01-31" \\
  -H "X-API-Key: YOUR_API_KEY"`,
    },
    {
      title: 'Run AI Query',
      description: 'Ask a natural language question about your cloud data.',
      curl: `curl -X POST "${BASE_URL}/api/query/ask" \\
  -H "X-API-Key: YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"question": "What are my top 5 most expensive services this month?"}'`,
    },
    {
      title: 'List Database Connections',
      description: 'Get all configured database connections.',
      curl: `curl -X GET "${BASE_URL}/api/query/connections" \\
  -H "X-API-Key: YOUR_API_KEY"`,
    },
    {
      title: 'Get Subscription',
      description: 'Retrieve current subscription and plan details.',
      curl: `curl -X GET "${BASE_URL}/api/billing/subscription" \\
  -H "X-API-Key: YOUR_API_KEY"`,
    },
  ]

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
        <div className="mb-2 flex items-center gap-2">
          <BookOpen className="h-5 w-5 text-indigo-500" />
          <h2 className="text-base font-semibold text-gray-900 dark:text-white">API Documentation</h2>
        </div>
        <p className="mb-5 text-sm text-gray-500 dark:text-gray-400">
          Use your API key from the API Keys section to authenticate. Base URL: <code className="rounded bg-gray-100 px-1.5 py-0.5 text-xs dark:bg-gray-700">{BASE_URL}</code>
        </p>

        <div className="space-y-5">
          {snippets.map(s => (
            <div key={s.title} className="rounded-xl border border-gray-100 dark:border-gray-700">
              <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-gray-700">
                <div>
                  <div className="flex items-center gap-2">
                    <Terminal className="h-4 w-4 text-indigo-400" />
                    <span className="text-sm font-semibold text-gray-800 dark:text-gray-200">{s.title}</span>
                  </div>
                  <p className="mt-0.5 text-xs text-gray-400">{s.description}</p>
                </div>
                <CopyButton text={s.curl} className="shrink-0" />
              </div>
              <pre className="overflow-x-auto rounded-b-xl bg-gray-950 px-4 py-3 text-xs leading-relaxed text-green-400">
                <code>{s.curl}</code>
              </pre>
            </div>
          ))}
        </div>
      </div>

      {/* Token usage note */}
      <div className="rounded-xl border border-indigo-100 bg-indigo-50 p-4 dark:border-indigo-900 dark:bg-indigo-900/20">
        <p className="text-sm font-medium text-indigo-700 dark:text-indigo-300">Rate Limits</p>
        <p className="mt-1 text-xs text-indigo-600 dark:text-indigo-400">
          Free plan: 100 req/min · Base: 500 req/min · Pro: 2000 req/min · Enterprise: 10000 req/min.
          Exceeding limits returns HTTP 429.
        </p>
      </div>
    </div>
  )
}

function SmtpSection() {
  const [form, setForm] = useState({ smtp_host: '', smtp_port: '587', smtp_username: '', password: '', from_email: '' })
  const [showPass, setShowPass] = useState(false)

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/api/auth/smtp', { ...form, smtp_port: parseInt(form.smtp_port) })
      toast.success('SMTP settings saved')
    } catch {
      toast.error('Failed to save SMTP settings')
    }
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
      <h2 className="mb-4 text-base font-semibold text-gray-900 dark:text-white">SMTP Configuration</h2>
      <form onSubmit={handleSave} className="space-y-4">
        {[
          { key: 'smtp_host', label: 'SMTP Host', placeholder: 'smtp.example.com' },
          { key: 'smtp_port', label: 'SMTP Port', placeholder: '587' },
          { key: 'smtp_username', label: 'Username', placeholder: 'user@example.com' },
          { key: 'from_email', label: 'From Email', placeholder: 'noreply@example.com' },
        ].map(f => (
          <div key={f.key}>
            <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{f.label}</label>
            <input
              type="text"
              value={form[f.key as keyof typeof form]}
              onChange={e => setForm(p => ({ ...p, [f.key]: e.target.value }))}
              placeholder={f.placeholder}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            />
          </div>
        ))}
        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Password</label>
          <div className="relative">
            <input
              type={showPass ? 'text' : 'password'}
              value={form.password}
              onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
              className="w-full rounded-md border border-gray-300 px-3 py-2 pr-10 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            />
            <button type="button" onClick={() => setShowPass(p => !p)} className="absolute right-3 top-2.5 text-gray-400">
              {showPass ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </div>
        <button type="submit" className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
          Save SMTP Settings
        </button>
      </form>
    </div>
  )
}

function DatabasesSection({ qc }: { qc: ReturnType<typeof useQueryClient> }) {
  const [showAdd, setShowAdd] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const emptyForm = {
    connection_name: '', db_type: 'postgresql', host: '', port: '5432',
    database_name: '', username: '', password: '', ssl_enabled: true,
  }
  const [form, setForm] = useState(emptyForm)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['connections'],
    queryFn: () => queryService.getConnections(),
  })

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await queryService.addConnection({ ...form, port: parseInt(form.port) })
      toast.success('Database connection added')
      setShowAdd(false)
      setForm(emptyForm)
      refetch()
      qc.invalidateQueries({ queryKey: ['connections'] })
    } catch {
      toast.error('Failed to add connection')
    }
  }

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editId) return
    try {
      const payload: Record<string, unknown> = {
        connection_name: form.connection_name,
        host: form.host,
        port: parseInt(form.port),
        database_name: form.database_name,
        username: form.username,
        ssl_enabled: form.ssl_enabled,
      }
      if (form.password) payload.password = form.password
      await queryService.updateConnection(editId, payload)
      toast.success('Connection updated')
      setEditId(null)
      setForm(emptyForm)
      refetch()
      qc.invalidateQueries({ queryKey: ['connections'] })
    } catch {
      toast.error('Failed to update connection')
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await queryService.deleteConnection(id)
      refetch()
      toast.success('Connection removed')
    } catch {
      toast.error('Failed to remove connection')
    }
  }

  const openEdit = (c: { id: string; connection_name: string; db_type: string; host: string; port: number; database_name: string; username: string; ssl_enabled?: boolean }) => {
    setEditId(c.id)
    setShowAdd(false)
    setForm({
      connection_name: c.connection_name,
      db_type: c.db_type,
      host: c.host,
      port: String(c.port),
      database_name: c.database_name,
      username: c.username,
      password: '',
      ssl_enabled: c.ssl_enabled ?? true,
    })
  }

  const cancelForm = () => {
    setShowAdd(false)
    setEditId(null)
    setForm(emptyForm)
  }

  const connectionForm = (isEdit: boolean) => (
    <form onSubmit={isEdit ? handleEdit : handleAdd} className="mb-4 space-y-3 rounded-lg bg-gray-50 p-4 dark:bg-gray-700/50">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
          {isEdit ? 'Edit Connection' : 'New Connection'}
        </span>
        <button type="button" onClick={cancelForm} className="text-gray-400 hover:text-gray-600">
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Name</label>
          <input type="text" value={form.connection_name} onChange={e => setForm(p => ({ ...p, connection_name: e.target.value }))} required
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Type</label>
          <select value={form.db_type} onChange={e => setForm(p => ({ ...p, db_type: e.target.value }))} disabled={isEdit}
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white disabled:opacity-60">
            <option value="postgresql">PostgreSQL</option>
            <option value="mysql">MySQL</option>
            <option value="mongodb">MongoDB</option>
            <option value="sqlserver">SQL Server</option>
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Host</label>
          <input type="text" value={form.host} onChange={e => setForm(p => ({ ...p, host: e.target.value }))} required
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Port</label>
          <input type="number" value={form.port} onChange={e => setForm(p => ({ ...p, port: e.target.value }))} required
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Database</label>
          <input type="text" value={form.database_name} onChange={e => setForm(p => ({ ...p, database_name: e.target.value }))} required
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Username</label>
          <input type="text" value={form.username} onChange={e => setForm(p => ({ ...p, username: e.target.value }))} required
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" />
        </div>
        <div className="sm:col-span-2">
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">
            Password {isEdit && <span className="text-gray-400">(leave blank to keep existing)</span>}
          </label>
          <input type="password" value={form.password} onChange={e => setForm(p => ({ ...p, password: e.target.value }))}
            required={!isEdit}
            placeholder={isEdit ? 'Leave blank to keep existing' : ''}
            className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white" />
        </div>
      </div>
      <div className="flex gap-2">
        <button type="submit" className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700">
          {isEdit ? 'Save Changes' : 'Add Connection'}
        </button>
        <button type="button" onClick={cancelForm} className="rounded-md border border-gray-300 px-3 py-1.5 text-sm hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700">
          Cancel
        </button>
      </div>
    </form>
  )

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-base font-semibold text-gray-900 dark:text-white">Database Connections</h2>
        <button
          onClick={() => { cancelForm(); setShowAdd(p => !p) }}
          className="flex items-center gap-1 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
        >
          <Plus className="h-4 w-4" /> Add
        </button>
      </div>

      {showAdd && connectionForm(false)}
      {editId && connectionForm(true)}

      {isLoading ? (
        <LoadingSpinner size="sm" />
      ) : (
        <div className="space-y-2">
          {(data?.connections ?? []).map(c => (
            <div key={c.id} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 dark:bg-gray-700/50">
              <div>
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300">{c.connection_name}</p>
                <p className="text-xs text-gray-400">{c.db_type} · {c.host}:{c.port}/{c.database_name}</p>
              </div>
              <div className="flex items-center gap-2">
                <span className={`rounded-full px-2 py-0.5 text-xs ${c.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                  {c.status}
                </span>
                <button onClick={() => openEdit(c)} className="text-gray-400 hover:text-blue-500">
                  <Pencil className="h-4 w-4" />
                </button>
                <button onClick={() => handleDelete(c.id)} className="text-red-500 hover:text-red-700">
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            </div>
          ))}
          {(!data?.connections || data.connections.length === 0) && (
            <p className="text-sm text-gray-500">No database connections yet.</p>
          )}
        </div>
      )}
    </div>
  )
}

function NotificationsSection() {
  const [prefs, setPrefs] = useState({
    anomaly_alerts: true,
    billing_notifications: true,
    weekly_report: false,
    query_failures: true,
  })

  const handleSave = async () => {
    try {
      await api.put('/api/auth/notification-preferences', prefs)
      toast.success('Preferences saved')
    } catch {
      toast.error('Failed to save preferences')
    }
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
      <h2 className="mb-4 text-base font-semibold text-gray-900 dark:text-white">Notification Preferences</h2>
      <div className="space-y-3">
        {Object.entries(prefs).map(([key, val]) => (
          <label key={key} className="flex items-center justify-between">
            <span className="text-sm capitalize text-gray-700 dark:text-gray-300">{key.replace(/_/g, ' ')}</span>
            <input
              type="checkbox"
              checked={val}
              onChange={e => setPrefs(p => ({ ...p, [key]: e.target.checked }))}
              className="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
          </label>
        ))}
      </div>
      <button onClick={handleSave} className="mt-4 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
        Save Preferences
      </button>
    </div>
  )
}
