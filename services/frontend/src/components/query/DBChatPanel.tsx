import { useState, useRef, useEffect } from 'react'
import {
  Send, ChevronDown, Sparkles, BarChart3, Table,
  TrendingUp, PieChart, Hash, Loader2, CheckCircle2, Plus,
  MessageSquare, X, Maximize2, Minimize2, Bookmark, BookmarkCheck,
  RefreshCw, Trash2, Mail,
} from 'lucide-react'
import {
  BarChart, Bar, LineChart, Line, PieChart as RechartsPie, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import {
  queryService, type DatabaseConnection, type QueryResult, type Bookmark as BookmarkType,
} from '../../services/query.service'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CHART_COLORS } from '../../utils/chartHelpers'
import ConfirmModal from '../common/ConfirmModal'
import toast from 'react-hot-toast'

// ── Types ─────────────────────────────────────────────────────────────────────

interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  result?: QueryResult
  queryText?: string   // the user question that produced this result
  loading?: boolean
  error?: string
  ts: number
}

interface Props {
  connections: DatabaseConnection[]
  onAddConnection: () => void
}

type Tab = 'chat' | 'bookmarks'

// ── Constants ─────────────────────────────────────────────────────────────────

const CHART_ICON: Record<string, React.ReactNode> = {
  pie:    <PieChart className="h-3 w-3" />,
  bar:    <BarChart3 className="h-3 w-3" />,
  line:   <TrendingUp className="h-3 w-3" />,
  table:  <Table className="h-3 w-3" />,
  metric: <Hash className="h-3 w-3" />,
}

const CHART_COLOR_CLASS: Record<string, string> = {
  pie:    'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
  bar:    'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  line:   'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
  table:  'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300',
  metric: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300',
}

const DB_DOT: Record<string, string> = {
  postgresql: 'bg-blue-500',
  mysql: 'bg-orange-500',
  mongodb: 'bg-green-500',
  sqlserver: 'bg-red-500',
}

// ── Mini chart ────────────────────────────────────────────────────────────────

function MiniChart({ result }: { result: QueryResult }) {
  const chartData = result.labels.map((label, i) => ({ name: label, value: result.data[i] }))
  const hasData = result.data.length > 0 && result.data.some(v => v != null)

  if (result.chart_type === 'metric') {
    return (
      <div className="flex items-center justify-center py-4">
        <div className="text-center">
          <div className="text-4xl font-black text-indigo-600 dark:text-indigo-400">
            {result.data[0]?.toLocaleString() ?? '—'}
          </div>
          <div className="mt-1 text-xs text-gray-400">{result.labels[0] ?? ''}</div>
        </div>
      </div>
    )
  }

  if (!hasData || result.chart_type === 'table') {
    const cols = result.raw_data.length > 0 ? Object.keys(result.raw_data[0]) : []
    return (
      <div className="overflow-x-auto rounded-xl border border-gray-100 dark:border-gray-700">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-gray-50 dark:bg-gray-800">
              {cols.map(c => (
                <th key={c} className="px-3 py-2 text-left font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide">{c}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {result.raw_data.slice(0, 8).map((row, i) => (
              <tr key={i} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                {cols.map(c => (
                  <td key={c} className="px-3 py-2 text-gray-700 dark:text-gray-300">{String(row[c] ?? '')}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {result.raw_data.length > 8 && (
          <div className="px-3 py-2 text-xs text-gray-400 text-center border-t border-gray-100 dark:border-gray-700">
            +{result.raw_data.length - 8} more rows
          </div>
        )}
      </div>
    )
  }

  if (result.chart_type === 'pie') {
    return (
      <ResponsiveContainer width="100%" height={200}>
        <RechartsPie>
          <Pie data={chartData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={75}
            label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}>
            {chartData.map((_, i) => <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />)}
          </Pie>
          <Tooltip />
        </RechartsPie>
      </ResponsiveContainer>
    )
  }

  if (result.chart_type === 'line') {
    return (
      <ResponsiveContainer width="100%" height={200}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
          <XAxis dataKey="name" tick={{ fontSize: 10 }} />
          <YAxis tick={{ fontSize: 10 }} />
          <Tooltip />
          <Line type="monotone" dataKey="value" stroke="#6366f1" strokeWidth={2} dot={false} />
        </LineChart>
      </ResponsiveContainer>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={200}>
      <BarChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
        <XAxis dataKey="name" tick={{ fontSize: 10 }} />
        <YAxis tick={{ fontSize: 10 }} />
        <Tooltip />
        <Bar dataKey="value" radius={[4, 4, 0, 0]}>
          {chartData.map((_, i) => <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />)}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}

// ── SQL badge ─────────────────────────────────────────────────────────────────

function SQLBadge({ sql }: { sql: string }) {
  const [show, setShow] = useState(false)
  return (
    <div className="mt-2">
      <button onClick={() => setShow(s => !s)}
        className="flex items-center gap-1 text-[10px] font-mono text-indigo-500 hover:text-indigo-700 dark:text-indigo-400">
        {show ? '▾' : '▸'} SQL
      </button>
      {show && (
        <pre className="mt-1 overflow-x-auto rounded-lg bg-gray-900 p-3 text-[10px] text-green-400 font-mono leading-relaxed">
          {sql}
        </pre>
      )}
    </div>
  )
}

// ── Bookmark save button (per message) ───────────────────────────────────────

function BookmarkButton({
  result, queryText, connectionId, onSaved,
}: {
  result: QueryResult
  queryText: string
  connectionId: string
  onSaved: () => void
}) {
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [showInput, setShowInput] = useState(false)
  const [title, setTitle] = useState('')

  const handleSave = async () => {
    if (!title.trim()) return
    setSaving(true)
    try {
      await queryService.createBookmark({
        title: title.trim(),
        connection_id: connectionId,
        query_text: queryText,
        generated_sql: result.generated_sql,
        chart_type: result.chart_type,
        labels: result.labels,
        data: result.data,
        raw_data: result.raw_data,
      })
      setSaved(true)
      setShowInput(false)
      setTitle('')
      onSaved()
      toast.success('Bookmarked!')
    } catch {
      toast.error('Failed to save bookmark')
    } finally {
      setSaving(false)
    }
  }

  if (saved) {
    return (
      <div className="mt-2 flex items-center gap-1 text-[10px] text-amber-500 font-medium">
        <BookmarkCheck className="h-3.5 w-3.5" /> Saved to bookmarks
      </div>
    )
  }

  return (
    <div className="mt-2">
      {!showInput ? (
        <button
          onClick={() => setShowInput(true)}
          className="flex items-center gap-1 text-[10px] text-gray-400 hover:text-amber-500 transition-colors font-medium"
        >
          <Bookmark className="h-3.5 w-3.5" /> Save to bookmarks
        </button>
      ) : (
        <div className="flex items-center gap-1.5 mt-1">
          <input
            autoFocus
            type="text"
            value={title}
            onChange={e => setTitle(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleSave(); if (e.key === 'Escape') setShowInput(false) }}
            placeholder="Bookmark title…"
            className="flex-1 rounded-lg border border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/20 px-2.5 py-1 text-xs text-gray-800 dark:text-white focus:outline-none focus:ring-2 focus:ring-amber-400"
          />
          <button
            onClick={handleSave}
            disabled={saving || !title.trim()}
            className="flex h-6 w-6 items-center justify-center rounded-lg bg-amber-500 text-white hover:bg-amber-600 disabled:opacity-40 transition-colors"
          >
            {saving ? <Loader2 className="h-3 w-3 animate-spin" /> : <BookmarkCheck className="h-3 w-3" />}
          </button>
          <button
            onClick={() => { setShowInput(false); setTitle('') }}
            className="flex h-6 w-6 items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
          >
            <X className="h-3 w-3" />
          </button>
        </div>
      )}
    </div>
  )
}

// ── Bookmarks tab panel ───────────────────────────────────────────────────────

function BookmarksTab({
  bookmarks, onRefresh, onReplay,
}: {
  bookmarks: BookmarkType[]
  onRefresh: () => void
  onReplay: (bm: BookmarkType) => void
}) {
  const [loadingId, setLoadingId] = useState<string | null>(null)
  const [emailModal, setEmailModal] = useState<{ id: string; title: string } | null>(null)
  const [emailInput, setEmailInput] = useState('')
  const [sendingEmail, setSendingEmail] = useState(false)
  const [confirmBm, setConfirmBm] = useState<BookmarkType | null>(null)

  const handleRefresh = async (bm: BookmarkType) => {
    setLoadingId(bm.id)
    try {
      await queryService.refreshBookmark(bm.id)
      onRefresh()
      toast.success(`"${bm.title}" refreshed`)
    } catch {
      toast.error('Failed to refresh')
    } finally {
      setLoadingId(null)
    }
  }

  const handleDelete = async () => {
    if (!confirmBm) return
    const bm = confirmBm
    setConfirmBm(null)
    try {
      await queryService.deleteBookmark(bm.id)
      onRefresh()
      toast.success('Bookmark deleted')
    } catch {
      toast.error('Failed to delete')
    }
  }

  const handleSendEmail = async () => {
    if (!emailModal || !emailInput.trim()) return
    setSendingEmail(true)
    try {
      await queryService.sendEmailReport(emailModal.id, emailInput.trim())
      toast.success(`Sent to ${emailInput.trim()}`)
      setEmailModal(null)
      setEmailInput('')
    } catch {
      toast.error('Failed to send')
    } finally {
      setSendingEmail(false)
    }
  }

  if (bookmarks.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-16 text-center">
        <div className="mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 dark:bg-amber-900/30">
          <Bookmark className="h-7 w-7 text-amber-500" />
        </div>
        <p className="text-sm font-semibold text-gray-700 dark:text-gray-300">No bookmarks yet</p>
        <p className="mt-1 text-xs text-gray-400">Click the bookmark icon on any query result to save it</p>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto px-3 py-3 space-y-2 bg-gray-50 dark:bg-gray-950">
      {bookmarks.map(bm => (
        <div key={bm.id}
          className="group rounded-2xl border border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-800 p-3 shadow-sm hover:shadow-md transition-shadow">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${CHART_COLOR_CLASS[bm.chart_type] ?? CHART_COLOR_CLASS.table}`}>
                  {bm.chart_type}
                </span>
                <p className="truncate text-sm font-semibold text-gray-800 dark:text-white">{bm.title}</p>
              </div>
              <p className="mt-0.5 truncate text-xs text-gray-400">{bm.query_text}</p>
              <p className="mt-0.5 text-[10px] text-gray-300 dark:text-gray-600">
                {new Date(bm.created_at).toLocaleDateString()}
              </p>
            </div>
            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                onClick={() => onReplay(bm)}
                title="Re-run in chat"
                className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-50 dark:bg-indigo-900/30 text-indigo-500 hover:bg-indigo-100 dark:hover:bg-indigo-900/50 transition-colors"
              >
                <Send className="h-3 w-3" />
              </button>
              <button
                onClick={() => handleRefresh(bm)}
                disabled={loadingId === bm.id}
                title="Refresh data"
                className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-500 hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors disabled:opacity-40"
              >
                <RefreshCw className={`h-3 w-3 ${loadingId === bm.id ? 'animate-spin' : ''}`} />
              </button>
              <button
                onClick={() => setEmailModal({ id: bm.id, title: bm.title })}
                title="Email report"
                className="flex h-7 w-7 items-center justify-center rounded-lg bg-green-50 dark:bg-green-900/30 text-green-500 hover:bg-green-100 dark:hover:bg-green-900/50 transition-colors"
              >
                <Mail className="h-3 w-3" />
              </button>
              <button
                onClick={() => setConfirmBm(bm)}
                title="Delete"
                className="flex h-7 w-7 items-center justify-center rounded-lg bg-red-50 dark:bg-red-900/30 text-red-500 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors"
              >
                <Trash2 className="h-3 w-3" />
              </button>
            </div>
          </div>
        </div>
      ))}

      {/* Email modal */}
      {emailModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
          <div className="w-full max-w-sm rounded-2xl bg-white dark:bg-gray-800 p-6 shadow-2xl">
            <h3 className="mb-1 text-base font-bold text-gray-900 dark:text-white">Send Report</h3>
            <p className="mb-4 text-sm text-gray-500">"{emailModal.title}"</p>
            <input type="email" value={emailInput} onChange={e => setEmailInput(e.target.value)}
              placeholder="recipient@example.com"
              className="mb-4 w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:text-white" />
            <div className="flex gap-2">
              <button onClick={() => { setEmailModal(null); setEmailInput('') }}
                className="flex-1 rounded-xl border border-gray-200 dark:border-gray-600 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">
                Cancel
              </button>
              <button onClick={handleSendEmail} disabled={sendingEmail || !emailInput.trim()}
                className="flex-1 flex items-center justify-center gap-2 rounded-xl bg-indigo-600 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50">
                <Mail className="h-3.5 w-3.5" />
                {sendingEmail ? 'Sending…' : 'Send'}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmBm && (
        <ConfirmModal
          title="Delete Bookmark"
          message={`Delete "${confirmBm.title}"? This cannot be undone.`}
          confirmLabel="Delete"
          onConfirm={handleDelete}
          onCancel={() => setConfirmBm(null)}
        />
      )}
    </div>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export default function DBChatPanel({ connections, onAddConnection }: Props) {
  const qc = useQueryClient()
  const [selectedConn, setSelectedConn] = useState<DatabaseConnection | null>(connections[0] ?? null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [suggestions, setSuggestions] = useState<{ question: string; chart_hint: string }[]>([])
  const [expanded, setExpanded] = useState(false)
  const [activeTab, setActiveTab] = useState<Tab>('chat')
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const { data: bookmarks = [], refetch: refetchBookmarks } = useQuery({
    queryKey: ['bookmarks'],
    queryFn: () => queryService.getBookmarks(),
  })

  useEffect(() => {
    if (!selectedConn) return
    queryService.getSuggestions(selectedConn.id)
      .then(setSuggestions)
      .catch(() => setSuggestions([]))
  }, [selectedConn?.id])

  useEffect(() => {
    if (activeTab === 'chat') bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, activeTab])

  useEffect(() => {
    if (!selectedConn && connections.length > 0) setSelectedConn(connections[0])
  }, [connections])

  const sendMessage = async (text: string) => {
    if (!text.trim() || !selectedConn || loading) return
    setActiveTab('chat')
    const userMsg: ChatMessage = { id: crypto.randomUUID(), role: 'user', text, ts: Date.now() }
    const loadingMsg: ChatMessage = { id: crypto.randomUUID(), role: 'assistant', text: '', loading: true, ts: Date.now() }
    setMessages(m => [...m, userMsg, loadingMsg])
    setInput('')
    setLoading(true)

    try {
      const result = await queryService.executeQuery(selectedConn.id, text)
      setMessages(m => m.map(msg =>
        msg.id === loadingMsg.id
          ? { ...msg, loading: false, text: `Found ${result.raw_data.length} result${result.raw_data.length !== 1 ? 's' : ''} · ${result.execution_time_ms}ms`, result, queryText: text }
          : msg
      ))
    } catch (err: unknown) {
      const errMsg = (err as { response?: { data?: { detail?: string; error?: string } } })?.response?.data?.detail
        ?? (err as { response?: { data?: { error?: string } } })?.response?.data?.error
        ?? 'Query failed'
      setMessages(m => m.map(msg =>
        msg.id === loadingMsg.id ? { ...msg, loading: false, text: '', error: errMsg } : msg
      ))
      toast.error(errMsg)
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(input) }
  }

  const handleReplay = (bm: BookmarkType) => {
    setActiveTab('chat')
    sendMessage(bm.query_text)
  }

  const conn = selectedConn
  const connDot = conn ? (DB_DOT[conn.db_type] ?? 'bg-indigo-500') : 'bg-gray-400'

  return (
    <div className={`flex flex-col rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 shadow-xl overflow-hidden transition-all duration-300 ${expanded ? 'fixed inset-4 z-40' : 'h-[620px]'}`}>

      {/* ── Header ── */}
      <div className="flex-shrink-0 bg-gradient-to-r from-indigo-600 via-violet-600 to-purple-600">
        {/* Top row: icon + title + controls */}
        <div className="flex items-center gap-3 px-4 pt-3 pb-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-white/20">
            <MessageSquare className="h-4 w-4 text-white" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-bold text-white">AI Query Chat</p>
            <p className="text-[10px] text-white/60 truncate">
              {conn ? `${conn.connection_name} · ${conn.db_type}` : 'No connection selected'}
            </p>
          </div>

          {/* Connection switcher */}
          <div className="relative">
            <select
              value={selectedConn?.id ?? ''}
              onChange={e => { const c = connections.find(c => c.id === e.target.value); if (c) setSelectedConn(c) }}
              className="appearance-none rounded-lg bg-white/20 pl-7 pr-6 py-1.5 text-xs font-semibold text-white focus:outline-none focus:ring-2 focus:ring-white/50 cursor-pointer"
            >
              {connections.map(c => (
                <option key={c.id} value={c.id} className="text-gray-900 bg-white">{c.connection_name}</option>
              ))}
            </select>
            <div className={`absolute left-2 top-2 h-3 w-3 rounded-full ${connDot}`} />
            <ChevronDown className="absolute right-1.5 top-2 h-3 w-3 text-white/70 pointer-events-none" />
          </div>

          <button onClick={onAddConnection} title="Add database"
            className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/20 text-white hover:bg-white/30 transition-colors">
            <Plus className="h-3.5 w-3.5" />
          </button>
          <button onClick={() => setExpanded(e => !e)}
            className="flex h-7 w-7 items-center justify-center rounded-lg bg-white/20 text-white hover:bg-white/30 transition-colors">
            {expanded ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
          </button>
        </div>

        {/* Tab bar */}
        <div className="flex px-4 gap-1 pb-0">
          {([
            { id: 'chat' as Tab, label: 'Chat', icon: MessageSquare },
            { id: 'bookmarks' as Tab, label: `Bookmarks${bookmarks.length > 0 ? ` (${bookmarks.length})` : ''}`, icon: Bookmark },
          ] as { id: Tab; label: string; icon: React.ElementType }[]).map(tab => {
            const Icon = tab.icon
            const active = activeTab === tab.id
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-1.5 px-4 py-2 text-xs font-semibold rounded-t-xl transition-all ${
                  active
                    ? 'bg-white dark:bg-gray-900 text-indigo-600 dark:text-indigo-400 shadow-sm'
                    : 'text-white/70 hover:text-white hover:bg-white/10'
                }`}
              >
                <Icon className="h-3.5 w-3.5" />
                {tab.label}
                {tab.id === 'bookmarks' && bookmarks.length > 0 && !active && (
                  <span className="flex h-4 w-4 items-center justify-center rounded-full bg-amber-400 text-[9px] font-bold text-white">
                    {bookmarks.length > 9 ? '9+' : bookmarks.length}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </div>

      {/* ── Tab content ── */}
      {activeTab === 'bookmarks' ? (
        <BookmarksTab
          bookmarks={bookmarks}
          onRefresh={() => { refetchBookmarks(); qc.invalidateQueries({ queryKey: ['bookmarks'] }) }}
          onReplay={handleReplay}
        />
      ) : (
        <>
          {/* ── Chat messages ── */}
          <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4 bg-gray-50 dark:bg-gray-950">
            {messages.length === 0 && (
              <div className="flex flex-col items-center justify-center h-full text-center py-8">
                <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-violet-600 shadow-lg shadow-indigo-200 dark:shadow-indigo-900/40">
                  <Sparkles className="h-8 w-8 text-white" />
                </div>
                <p className="text-sm font-semibold text-gray-700 dark:text-gray-300">Ask anything about your data</p>
                <p className="mt-1 text-xs text-gray-400">Natural language → SQL → Chart, instantly</p>
                {suggestions.length > 0 && (
                  <div className="mt-5 flex flex-wrap justify-center gap-2 max-w-sm">
                    {suggestions.slice(0, 6).map((s, i) => (
                      <button key={i} onClick={() => sendMessage(s.question)}
                        className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-all hover:scale-105 ${CHART_COLOR_CLASS[s.chart_hint] ?? CHART_COLOR_CLASS.table}`}>
                        {CHART_ICON[s.chart_hint] ?? <BarChart3 className="h-3 w-3" />}
                        {s.question}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            {messages.map(msg => (
              <div key={msg.id} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                {msg.role === 'assistant' && (
                  <div className="mr-2 mt-1 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-violet-600 shadow">
                    <Sparkles className="h-3.5 w-3.5 text-white" />
                  </div>
                )}
                <div className={`${msg.role === 'user' ? 'max-w-[70%]' : 'w-full max-w-[85%]'}`}>
                  {msg.role === 'user' ? (
                    <div className="rounded-2xl rounded-tr-sm bg-gradient-to-br from-indigo-500 to-violet-600 px-4 py-2.5 text-sm text-white shadow-md">
                      {msg.text}
                    </div>
                  ) : (
                    <div className="rounded-2xl rounded-tl-sm bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 px-4 py-3 shadow-sm">
                      {msg.loading ? (
                        <div className="flex items-center gap-2 text-gray-400">
                          <Loader2 className="h-4 w-4 animate-spin text-indigo-500" />
                          <span className="text-xs">Generating SQL and querying</span>
                          <span className="flex gap-0.5">
                            {[0, 1, 2].map(i => (
                              <span key={i} className="h-1.5 w-1.5 rounded-full bg-indigo-400 animate-bounce" style={{ animationDelay: `${i * 0.15}s` }} />
                            ))}
                          </span>
                        </div>
                      ) : msg.error ? (
                        <div className="flex items-start gap-2">
                          <X className="h-4 w-4 text-red-500 mt-0.5 flex-shrink-0" />
                          <p className="text-xs text-red-600 dark:text-red-400">{msg.error}</p>
                        </div>
                      ) : (
                        <>
                          <div className="flex items-center gap-2 mb-2">
                            <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500 flex-shrink-0" />
                            <span className="text-xs text-gray-500 dark:text-gray-400">{msg.text}</span>
                            {msg.result && (
                              <span className={`ml-auto rounded-full px-2 py-0.5 text-[10px] font-semibold ${CHART_COLOR_CLASS[msg.result.chart_type] ?? CHART_COLOR_CLASS.table}`}>
                                {msg.result.chart_type}
                              </span>
                            )}
                          </div>
                          {msg.result && <MiniChart result={msg.result} />}
                          {msg.result && <SQLBadge sql={msg.result.generated_sql} />}
                          {msg.result && conn && (
                            <BookmarkButton
                              result={msg.result}
                              queryText={msg.queryText ?? msg.text}
                              connectionId={conn.id}
                              onSaved={() => { refetchBookmarks(); qc.invalidateQueries({ queryKey: ['bookmarks'] }) }}
                            />
                          )}
                        </>
                      )}
                    </div>
                  )}
                  <div className={`mt-1 text-[10px] text-gray-400 ${msg.role === 'user' ? 'text-right' : 'text-left'}`}>
                    {new Date(msg.ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </div>
                </div>
              </div>
            ))}
            <div ref={bottomRef} />
          </div>

          {/* ── Input ── */}
          <div className="flex-shrink-0 border-t border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900 px-4 py-3">
            {messages.length > 0 && suggestions.length > 0 && (
              <div className="mb-2 flex gap-1.5 overflow-x-auto pb-1">
                {suggestions.slice(0, 4).map((s, i) => (
                  <button key={i} onClick={() => sendMessage(s.question)}
                    className="flex-shrink-0 flex items-center gap-1 rounded-full border border-gray-200 dark:border-gray-700 px-2.5 py-1 text-[10px] font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                    {CHART_ICON[s.chart_hint] ?? <BarChart3 className="h-2.5 w-2.5" />}
                    {s.question.length > 30 ? s.question.slice(0, 30) + '…' : s.question}
                  </button>
                ))}
              </div>
            )}
            <div className="flex items-end gap-2">
              <textarea
                ref={inputRef}
                rows={1}
                value={input}
                onChange={e => {
                  setInput(e.target.value)
                  e.target.style.height = 'auto'
                  e.target.style.height = Math.min(e.target.scrollHeight, 120) + 'px'
                }}
                onKeyDown={handleKeyDown}
                placeholder={conn ? `Ask about ${conn.connection_name}…` : 'Select a connection first'}
                disabled={!conn || loading}
                className="flex-1 resize-none rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-4 py-2.5 text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50 transition-all"
                style={{ minHeight: '42px', maxHeight: '120px' }}
              />
              <button
                onClick={() => sendMessage(input)}
                disabled={!input.trim() || !conn || loading}
                className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-violet-600 text-white shadow-md hover:opacity-90 disabled:opacity-40 transition-all"
              >
                {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              </button>
            </div>
            <p className="mt-1.5 text-[10px] text-gray-400 text-center">Enter to send · Shift+Enter for new line</p>
          </div>
        </>
      )}
    </div>
  )
}
