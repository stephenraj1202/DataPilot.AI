import { useState, useEffect } from 'react'
import { X, Eye, EyeOff, CheckCircle2, Loader2, Database, ChevronRight, Sparkles, BarChart3, Zap } from 'lucide-react'
import { queryService, type DatabaseConnection } from '../../services/query.service'
import toast from 'react-hot-toast'

// ── DB type metadata ──────────────────────────────────────────────────────────

const DB_TYPES = [
  {
    id: 'postgresql',
    label: 'PostgreSQL',
    defaultPort: 5432,
    color: 'from-blue-500 to-indigo-600',
    bg: 'bg-blue-50 dark:bg-blue-900/20',
    border: 'border-blue-200 dark:border-blue-700',
    ring: 'ring-blue-500',
    icon: (
      <svg viewBox="0 0 24 24" className="h-7 w-7" fill="none">
        <ellipse cx="12" cy="5" rx="8" ry="3" fill="#336791" />
        <path d="M4 5v6c0 1.657 3.582 3 8 3s8-1.343 8-3V5" stroke="#336791" strokeWidth="1.5" fill="none" />
        <path d="M4 11v6c0 1.657 3.582 3 8 3s8-1.343 8-3v-6" stroke="#336791" strokeWidth="1.5" fill="none" />
        <ellipse cx="12" cy="17" rx="8" ry="3" fill="none" stroke="#336791" strokeWidth="1.5" />
      </svg>
    ),
  },
  {
    id: 'mysql',
    label: 'MySQL',
    defaultPort: 3306,
    color: 'from-orange-400 to-amber-500',
    bg: 'bg-orange-50 dark:bg-orange-900/20',
    border: 'border-orange-200 dark:border-orange-700',
    ring: 'ring-orange-500',
    icon: (
      <svg viewBox="0 0 24 24" className="h-7 w-7" fill="none">
        <ellipse cx="12" cy="5" rx="8" ry="3" fill="#F29111" />
        <path d="M4 5v6c0 1.657 3.582 3 8 3s8-1.343 8-3V5" stroke="#F29111" strokeWidth="1.5" fill="none" />
        <path d="M4 11v6c0 1.657 3.582 3 8 3s8-1.343 8-3v-6" stroke="#F29111" strokeWidth="1.5" fill="none" />
        <ellipse cx="12" cy="17" rx="8" ry="3" fill="none" stroke="#F29111" strokeWidth="1.5" />
      </svg>
    ),
  },
  {
    id: 'mongodb',
    label: 'MongoDB',
    defaultPort: 27017,
    color: 'from-green-500 to-emerald-600',
    bg: 'bg-green-50 dark:bg-green-900/20',
    border: 'border-green-200 dark:border-green-700',
    ring: 'ring-green-500',
    icon: (
      <svg viewBox="0 0 24 24" className="h-7 w-7" fill="none">
        <path d="M12 2C9 2 6.5 6 6.5 12c0 4 1.8 7 4.5 8.5V22h2v-1.5C15.7 19 17.5 16 17.5 12 17.5 6 15 2 12 2z" fill="#4DB33D" />
        <path d="M12 2v20" stroke="#3FA037" strokeWidth="1" />
      </svg>
    ),
  },
  {
    id: 'sqlserver',
    label: 'SQL Server',
    defaultPort: 1433,
    color: 'from-red-500 to-rose-600',
    bg: 'bg-red-50 dark:bg-red-900/20',
    border: 'border-red-200 dark:border-red-700',
    ring: 'ring-red-500',
    icon: (
      <svg viewBox="0 0 24 24" className="h-7 w-7" fill="none">
        <ellipse cx="12" cy="5" rx="8" ry="3" fill="#CC2927" />
        <path d="M4 5v6c0 1.657 3.582 3 8 3s8-1.343 8-3V5" stroke="#CC2927" strokeWidth="1.5" fill="none" />
        <path d="M4 11v6c0 1.657 3.582 3 8 3s8-1.343 8-3v-6" stroke="#CC2927" strokeWidth="1.5" fill="none" />
        <ellipse cx="12" cy="17" rx="8" ry="3" fill="none" stroke="#CC2927" strokeWidth="1.5" />
      </svg>
    ),
  },
]

// ── Step indicator ────────────────────────────────────────────────────────────

const STEPS = [
  { label: 'Connect', icon: Database, color: 'bg-indigo-500', desc: 'Enter credentials' },
  { label: 'Analyze', icon: Sparkles, color: 'bg-violet-500', desc: 'Schema extracted' },
  { label: 'Query', icon: BarChart3, color: 'bg-emerald-500', desc: 'Ask in plain English' },
]

type WizardStep = 0 | 1 | 2

interface Props {
  onClose: () => void
  onConnected: (conn: DatabaseConnection) => void
}

export default function DBConnectWizard({ onClose, onConnected }: Props) {
  const [step, setStep] = useState<WizardStep>(0)
  const [selectedDB, setSelectedDB] = useState(DB_TYPES[0])
  const [form, setForm] = useState({
    connection_name: '',
    host: 'localhost',
    port: '5432',
    database_name: '',
    username: '',
    password: '',
    ssl_enabled: true,
  })
  const [showPass, setShowPass] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [connected, setConnected] = useState(false)
  const [newConn, setNewConn] = useState<DatabaseConnection | null>(null)
  const [analyzing, setAnalyzing] = useState(false)
  const [analyzed, setAnalyzed] = useState(false)
  const [schemaInfo, setSchemaInfo] = useState<{ tables: number; columns: number } | null>(null)
  const [dots, setDots] = useState('')

  // Animated dots for loading states
  useEffect(() => {
    if (!connecting && !analyzing) return
    const t = setInterval(() => setDots(d => d.length >= 3 ? '' : d + '.'), 400)
    return () => clearInterval(t)
  }, [connecting, analyzing])

  // Auto-fill port when DB type changes
  const selectDB = (db: typeof DB_TYPES[0]) => {
    setSelectedDB(db)
    setForm(f => ({ ...f, port: String(db.defaultPort) }))
  }

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault()
    setConnecting(true)
    try {
      const conn = await queryService.addConnection({
        ...form,
        db_type: selectedDB.id,
        port: parseInt(form.port),
      })
      setNewConn(conn)
      setConnected(true)
      setTimeout(() => {
        setStep(1)
        setAnalyzing(true)
        // Simulate schema analysis (real call happens in background)
        queryService.getSchema(conn.id)
          .then(schema => {
            const tbls = schema?.tables ?? []
            const tables = tbls.length
            const cols = tbls.reduce((a, t) => a + (t.columns?.length ?? 0), 0)
            setSchemaInfo({ tables, columns: cols })
          })
          .catch(() => setSchemaInfo({ tables: 0, columns: 0 }))
          .finally(() => {
            setAnalyzing(false)
            setAnalyzed(true)
            setTimeout(() => setStep(2), 800)
          })
      }, 600)
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string; detail?: string } } })?.response?.data?.error
        ?? (err as { response?: { data?: { detail?: string } } })?.response?.data?.detail
        ?? 'Connection failed'
      toast.error(msg)
    } finally {
      setConnecting(false)
    }
  }

  const handleDone = () => {
    if (newConn) onConnected(newConn)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div
        className="relative w-full max-w-lg rounded-3xl bg-white dark:bg-gray-900 shadow-2xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        {/* Gradient header */}
        <div className={`bg-gradient-to-r ${selectedDB.color} p-6 pb-8`}>
          <button
            onClick={onClose}
            className="absolute right-4 top-4 rounded-full bg-white/20 p-1.5 text-white hover:bg-white/30 transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
          <div className="flex items-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/20 backdrop-blur-sm">
              <Database className="h-6 w-6 text-white" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-white">Connect Database</h2>
              <p className="text-sm text-white/70">3 steps to AI-powered insights</p>
            </div>
          </div>

          {/* Step pills */}
          <div className="mt-5 flex items-center gap-2">
            {STEPS.map((s, i) => {
              const Icon = s.icon
              const done = step > i
              const active = step === i
              return (
                <div key={i} className="flex items-center gap-2">
                  <div className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-semibold transition-all duration-500 ${
                    done ? 'bg-white text-gray-800' :
                    active ? 'bg-white/30 text-white ring-2 ring-white/50' :
                    'bg-white/10 text-white/50'
                  }`}>
                    {done ? <CheckCircle2 className="h-3.5 w-3.5 text-green-500" /> : <Icon className="h-3.5 w-3.5" />}
                    {s.label}
                  </div>
                  {i < STEPS.length - 1 && (
                    <ChevronRight className={`h-3.5 w-3.5 transition-colors ${step > i ? 'text-white' : 'text-white/30'}`} />
                  )}
                </div>
              )
            })}
          </div>
        </div>

        {/* Step content */}
        <div className="p-6">
          {/* ── STEP 0: Connect ── */}
          {step === 0 && (
            <form onSubmit={handleConnect} className="space-y-4">
              {/* DB type selector */}
              <div>
                <label className="mb-2 block text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  Database Type
                </label>
                <div className="grid grid-cols-4 gap-2">
                  {DB_TYPES.map(db => (
                    <button
                      key={db.id}
                      type="button"
                      onClick={() => selectDB(db)}
                      className={`flex flex-col items-center gap-1.5 rounded-xl border-2 p-2.5 transition-all ${
                        selectedDB.id === db.id
                          ? `${db.border} ${db.bg} ring-2 ${db.ring} ring-offset-1`
                          : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                      }`}
                    >
                      {db.icon}
                      <span className="text-[10px] font-semibold text-gray-700 dark:text-gray-300">{db.label}</span>
                    </button>
                  ))}
                </div>
              </div>

              {/* Connection name */}
              <div>
                <label className="mb-1 block text-xs font-semibold text-gray-600 dark:text-gray-400">Connection Name</label>
                <input
                  type="text"
                  required
                  placeholder={`My ${selectedDB.label} DB`}
                  value={form.connection_name}
                  onChange={e => setForm(f => ({ ...f, connection_name: e.target.value }))}
                  className="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              {/* Host + Port */}
              <div className="grid grid-cols-3 gap-3">
                <div className="col-span-2">
                  <label className="mb-1 block text-xs font-semibold text-gray-600 dark:text-gray-400">Host</label>
                  <input
                    type="text"
                    required
                    value={form.host}
                    onChange={e => setForm(f => ({ ...f, host: e.target.value }))}
                    className="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-gray-600 dark:text-gray-400">Port</label>
                  <input
                    type="number"
                    required
                    value={form.port}
                    onChange={e => setForm(f => ({ ...f, port: e.target.value }))}
                    className="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
              </div>

              {/* Database + Username */}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-gray-600 dark:text-gray-400">Database</label>
                  <input
                    type="text"
                    required
                    value={form.database_name}
                    onChange={e => setForm(f => ({ ...f, database_name: e.target.value }))}
                    className="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-gray-600 dark:text-gray-400">Username</label>
                  <input
                    type="text"
                    required
                    value={form.username}
                    onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
                    className="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
              </div>

              {/* Password with eye toggle */}
              <div>
                <label className="mb-1 block text-xs font-semibold text-gray-600 dark:text-gray-400">Password</label>
                <div className="relative">
                  <input
                    type={showPass ? 'text' : 'password'}
                    required
                    value={form.password}
                    onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                    className="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 pr-10 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPass(p => !p)}
                    className="absolute right-3 top-2.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  >
                    {showPass ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              {/* SSL toggle */}
              <label className="flex cursor-pointer items-center gap-2">
                <div
                  onClick={() => setForm(f => ({ ...f, ssl_enabled: !f.ssl_enabled }))}
                  className={`relative h-5 w-9 rounded-full transition-colors ${form.ssl_enabled ? 'bg-indigo-500' : 'bg-gray-300 dark:bg-gray-600'}`}
                >
                  <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${form.ssl_enabled ? 'translate-x-4' : 'translate-x-0.5'}`} />
                </div>
                <span className="text-xs font-medium text-gray-600 dark:text-gray-400">SSL enabled</span>
              </label>

              <button
                type="submit"
                disabled={connecting}
                className={`w-full rounded-xl bg-gradient-to-r ${selectedDB.color} py-3 text-sm font-bold text-white shadow-lg transition-all hover:opacity-90 disabled:opacity-60 flex items-center justify-center gap-2`}
              >
                {connecting ? (
                  <><Loader2 className="h-4 w-4 animate-spin" /> Connecting{dots}</>
                ) : connected ? (
                  <><CheckCircle2 className="h-4 w-4" /> Connected!</>
                ) : (
                  <><Zap className="h-4 w-4" /> Connect</>
                )}
              </button>
            </form>
          )}

          {/* ── STEP 1: Analyze ── */}
          {step === 1 && (
            <div className="flex flex-col items-center py-6 text-center">
              {/* 3 animated boxes */}
              <div className="mb-8 flex items-center gap-4">
                {[
                  { label: 'Connected', color: 'from-indigo-400 to-indigo-600', done: true },
                  { label: 'Extracting Schema', color: 'from-violet-400 to-violet-600', done: analyzed },
                  { label: 'Saving', color: 'from-emerald-400 to-emerald-600', done: analyzed },
                ].map((box, i) => (
                  <div key={i} className="flex flex-col items-center gap-2">
                    <div className={`relative flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br ${box.done ? box.color : 'from-gray-200 to-gray-300 dark:from-gray-700 dark:to-gray-600'} shadow-lg transition-all duration-700`}>
                      {box.done ? (
                        <CheckCircle2 className="h-7 w-7 text-white" />
                      ) : (
                        <Loader2 className="h-7 w-7 animate-spin text-white/60" />
                      )}
                      {box.done && (
                        <span className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-green-400 shadow">
                          <CheckCircle2 className="h-3 w-3 text-white" />
                        </span>
                      )}
                    </div>
                    <span className="text-[10px] font-semibold text-gray-500 dark:text-gray-400">{box.label}</span>
                    {i < 2 && <div className={`absolute mt-8 h-0.5 w-8 ${box.done ? 'bg-green-400' : 'bg-gray-200 dark:bg-gray-700'}`} />}
                  </div>
                ))}
              </div>

              {analyzing ? (
                <>
                  <div className="mb-2 flex items-center gap-2 text-violet-600 dark:text-violet-400">
                    <Loader2 className="h-5 w-5 animate-spin" />
                    <span className="text-sm font-semibold">Analyzing schema{dots}</span>
                  </div>
                  <p className="text-xs text-gray-400">Reading tables, columns, and relationships</p>
                </>
              ) : (
                <>
                  <div className="mb-2 flex items-center gap-2 text-emerald-600 dark:text-emerald-400">
                    <CheckCircle2 className="h-5 w-5" />
                    <span className="text-sm font-semibold">Schema analyzed!</span>
                  </div>
                  {schemaInfo && (
                    <p className="text-xs text-gray-400">
                      Found {schemaInfo.tables} tables · {schemaInfo.columns} columns
                    </p>
                  )}
                </>
              )}
            </div>
          )}

          {/* ── STEP 2: Ready ── */}
          {step === 2 && (
            <div className="flex flex-col items-center py-4 text-center">
              {/* Celebration animation */}
              <div className="relative mb-6">
                <div className="flex h-24 w-24 items-center justify-center rounded-full bg-gradient-to-br from-emerald-400 to-teal-500 shadow-2xl shadow-emerald-200 dark:shadow-emerald-900/40">
                  <CheckCircle2 className="h-12 w-12 text-white" />
                </div>
                {/* Sparkle dots */}
                {[...Array(8)].map((_, i) => (
                  <span
                    key={i}
                    className="absolute h-2 w-2 rounded-full bg-emerald-400 animate-ping"
                    style={{
                      top: `${50 + 45 * Math.sin((i * Math.PI * 2) / 8)}%`,
                      left: `${50 + 45 * Math.cos((i * Math.PI * 2) / 8)}%`,
                      animationDelay: `${i * 0.1}s`,
                      animationDuration: '1.2s',
                    }}
                  />
                ))}
              </div>

              <h3 className="mb-1 text-xl font-bold text-gray-900 dark:text-white">You're all set!</h3>
              <p className="mb-1 text-sm text-gray-500 dark:text-gray-400">
                <span className="font-semibold text-gray-700 dark:text-gray-200">{form.connection_name}</span> is connected
              </p>
              {schemaInfo && (
                <p className="mb-6 text-xs text-gray-400">
                  {schemaInfo.tables} tables · {schemaInfo.columns} columns indexed
                </p>
              )}

              {/* 3 colourful feature boxes */}
              <div className="mb-6 grid w-full grid-cols-3 gap-3">
                {[
                  { icon: Database, label: 'Connected', sub: 'Live DB link', color: 'from-indigo-400 to-indigo-600', done: true },
                  { icon: Sparkles, label: 'Schema', sub: 'AI-indexed', color: 'from-violet-400 to-violet-600', done: true },
                  { icon: BarChart3, label: 'Query', sub: 'Ask anything', color: 'from-emerald-400 to-emerald-600', done: true },
                ].map((box, i) => {
                  const Icon = box.icon
                  return (
                    <div key={i} className={`flex flex-col items-center gap-1.5 rounded-2xl bg-gradient-to-br ${box.color} p-3 shadow-lg`}>
                      <Icon className="h-5 w-5 text-white" />
                      <span className="text-xs font-bold text-white">{box.label}</span>
                      <span className="text-[10px] text-white/70">{box.sub}</span>
                    </div>
                  )
                })}
              </div>

              <button
                onClick={handleDone}
                className="w-full rounded-xl bg-gradient-to-r from-indigo-500 to-violet-600 py-3 text-sm font-bold text-white shadow-lg hover:opacity-90 transition-all flex items-center justify-center gap-2"
              >
                <BarChart3 className="h-4 w-4" />
                Start Querying
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
