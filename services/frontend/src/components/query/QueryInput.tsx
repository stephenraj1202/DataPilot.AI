import { useEffect, useState } from 'react'
import { Send, Database, Lightbulb, BarChart2, PieChart, TrendingUp, Table, Hash } from 'lucide-react'
import { DatabaseConnection, queryService } from '../../services/query.service'

interface Suggestion {
  question: string
  chart_hint: string
}

interface QueryInputProps {
  connections: DatabaseConnection[]
  onSubmit: (connectionId: string, query: string) => void
  isLoading: boolean
  prefilledQuery?: string
  onQueryChange?: (q: string) => void
}

const CHART_HINT_ICON: Record<string, React.ReactNode> = {
  pie:    <PieChart className="h-3 w-3" />,
  bar:    <BarChart2 className="h-3 w-3" />,
  line:   <TrendingUp className="h-3 w-3" />,
  table:  <Table className="h-3 w-3" />,
  metric: <Hash className="h-3 w-3" />,
}

const CHART_HINT_COLOR: Record<string, string> = {
  pie:    'bg-purple-100 text-purple-700 hover:bg-purple-200 dark:bg-purple-900/30 dark:text-purple-300',
  bar:    'bg-blue-100 text-blue-700 hover:bg-blue-200 dark:bg-blue-900/30 dark:text-blue-300',
  line:   'bg-green-100 text-green-700 hover:bg-green-200 dark:bg-green-900/30 dark:text-green-300',
  table:  'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300',
  metric: 'bg-orange-100 text-orange-700 hover:bg-orange-200 dark:bg-orange-900/30 dark:text-orange-300',
}

export default function QueryInput({
  connections,
  onSubmit,
  isLoading,
  prefilledQuery,
  onQueryChange,
}: QueryInputProps) {
  const [query, setQuery] = useState(prefilledQuery ?? '')
  const [connectionId, setConnectionId] = useState(connections[0]?.id ?? '')
  const [suggestions, setSuggestions] = useState<Suggestion[]>([])
  const [loadingSuggestions, setLoadingSuggestions] = useState(false)

  // Sync prefilled query from parent (history click)
  useEffect(() => {
    if (prefilledQuery !== undefined) setQuery(prefilledQuery)
  }, [prefilledQuery])

  // Load suggestions when connection changes
  useEffect(() => {
    if (!connectionId) return
    setLoadingSuggestions(true)
    queryService.getSuggestions(connectionId)
      .then(setSuggestions)
      .catch(() => setSuggestions([]))
      .finally(() => setLoadingSuggestions(false))
  }, [connectionId])

  const handleQueryChange = (val: string) => {
    setQuery(val)
    onQueryChange?.(val)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!query.trim() || !connectionId) return
    onSubmit(connectionId, query.trim())
  }

  const handleSuggestionClick = (s: Suggestion) => {
    handleQueryChange(s.question)
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <form onSubmit={handleSubmit} className="p-4">
        {/* Connection selector */}
        <div className="mb-3 flex items-center gap-2">
          <Database className="h-4 w-4 shrink-0 text-gray-500" />
          <select
            value={connectionId}
            onChange={e => setConnectionId(e.target.value)}
            className="flex-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          >
            {connections.length === 0 && <option value="">No connections available</option>}
            {connections.map(c => (
              <option key={c.id} value={c.id}>
                {c.connection_name} ({c.db_type})
              </option>
            ))}
          </select>
        </div>

        {/* Textarea + Run button */}
        <div className="flex gap-2">
          <textarea
            value={query}
            onChange={e => handleQueryChange(e.target.value)}
            placeholder="Ask a question in plain English, e.g. 'Show me top 10 customers by revenue this month'"
            rows={3}
            className="flex-1 resize-none rounded-md border border-gray-300 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            onKeyDown={e => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleSubmit(e as unknown as React.FormEvent)
            }}
          />
          <button
            type="submit"
            disabled={isLoading || !query.trim() || !connectionId}
            className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            <Send className="h-4 w-4" />
            {isLoading ? 'Running...' : 'Run'}
          </button>
        </div>
        <p className="mt-1 text-xs text-gray-400">Tip: Press Ctrl+Enter to submit</p>
      </form>

      {/* Suggested questions */}
      <div className="border-t border-gray-100 px-4 py-3 dark:border-gray-700">
        <div className="mb-2 flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
          <Lightbulb className="h-3.5 w-3.5" />
          {loadingSuggestions ? 'Loading suggestions...' : 'Suggested questions'}
        </div>
        {!loadingSuggestions && suggestions.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {suggestions.map((s, i) => (
              <button
                key={i}
                onClick={() => handleSuggestionClick(s)}
                className={`flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors ${CHART_HINT_COLOR[s.chart_hint] ?? CHART_HINT_COLOR.table}`}
              >
                {CHART_HINT_ICON[s.chart_hint]}
                {s.question}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
