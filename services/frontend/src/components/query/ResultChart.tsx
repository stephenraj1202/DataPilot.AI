import { useState, useEffect } from 'react'
import {
  BarChart, Bar, LineChart, Line, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
} from 'recharts'
import { Download, Bookmark, BarChart2, Table } from 'lucide-react'
import { QueryResult, queryService } from '../../services/query.service'
import { CHART_COLORS } from '../../utils/chartHelpers'
import { exportToCsv } from '../../utils/formatters'
import toast from 'react-hot-toast'

interface ResultChartProps {
  result: QueryResult
  connectionId?: string
  queryText?: string
  onBookmarkSaved?: () => void
}

export default function ResultChart({ result, connectionId, queryText, onBookmarkSaved }: ResultChartProps) {
  const [page, setPage] = useState(0)
  const [savingBookmark, setSavingBookmark] = useState(false)
  const [bookmarkTitle, setBookmarkTitle] = useState('')
  const [showBookmarkInput, setShowBookmarkInput] = useState(false)
  const pageSize = 10

  const chartData = result.labels.map((label, i) => ({
    name: label,
    value: result.data[i],
  }))

  const handleExport = () => {
    exportToCsv(result.raw_data as Record<string, unknown>[], 'query-results.csv')
  }

  const handleSaveBookmark = async () => {
    if (!bookmarkTitle.trim() || !connectionId || !queryText) return
    setSavingBookmark(true)
    try {
      await queryService.createBookmark({
        title: bookmarkTitle.trim(),
        connection_id: connectionId,
        query_text: queryText,
        generated_sql: result.generated_sql,
        chart_type: result.chart_type,
        labels: result.labels,
        data: result.data,
        raw_data: result.raw_data,
      })
      toast.success('Bookmark saved')
      setShowBookmarkInput(false)
      setBookmarkTitle('')
      onBookmarkSaved?.()
    } catch {
      toast.error('Failed to save bookmark')
    } finally {
      setSavingBookmark(false)
    }
  }

  const hasValidChartData = result.data.length > 0 && result.data.some(v => v !== null && v !== undefined)
  const isChartable = result.chart_type !== 'table' && result.chart_type !== 'metric' && hasValidChartData

  // If chart type is table or data is non-numeric, default to table view
  const defaultView = isChartable ? 'chart' : 'table'
  const [viewMode, setViewMode] = useState<'chart' | 'table'>(defaultView)

  // Reset view mode when result changes
  useEffect(() => {
    setViewMode(isChartable ? 'chart' : 'table')
    setPage(0)
  }, [result.query_id, result.generated_sql])

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      {/* Header */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
            {result.chart_type}
          </span>
          <span className="text-xs text-gray-500">{result.execution_time_ms}ms</span>
          <span className="text-xs text-gray-500">{result.raw_data.length} rows</span>
          {result.cached && (
            <span className="rounded-full bg-yellow-100 px-2 py-0.5 text-xs text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400">
              cached
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {isChartable && (
            <div className="flex rounded-md border border-gray-200 dark:border-gray-600">
              <button
                onClick={() => setViewMode('chart')}
                className={`flex items-center gap-1 rounded-l-md px-2.5 py-1.5 text-xs ${viewMode === 'chart' ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700'}`}
              >
                <BarChart2 className="h-3 w-3" /> Chart
              </button>
              <button
                onClick={() => setViewMode('table')}
                className={`flex items-center gap-1 rounded-r-md px-2.5 py-1.5 text-xs ${viewMode === 'table' ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700'}`}
              >
                <Table className="h-3 w-3" /> Table
              </button>
            </div>
          )}
          <button
            onClick={handleExport}
            className="flex items-center gap-1 rounded-md border border-gray-300 px-3 py-1.5 text-xs hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700"
          >
            <Download className="h-3 w-3" /> Export CSV
          </button>
          {connectionId && queryText && (
            <button
              onClick={() => setShowBookmarkInput(v => !v)}
              className="flex items-center gap-1 rounded-md border border-gray-300 px-3 py-1.5 text-xs hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700"
            >
              <Bookmark className="h-3 w-3" /> Bookmark
            </button>
          )}
        </div>
      </div>

      {/* Bookmark input */}
      {showBookmarkInput && (
        <div className="mb-4 flex gap-2">
          <input
            type="text"
            value={bookmarkTitle}
            onChange={e => setBookmarkTitle(e.target.value)}
            placeholder="Bookmark title..."
            className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            onKeyDown={e => e.key === 'Enter' && handleSaveBookmark()}
          />
          <button
            onClick={handleSaveBookmark}
            disabled={savingBookmark || !bookmarkTitle.trim()}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {savingBookmark ? 'Saving...' : 'Save'}
          </button>
        </div>
      )}

      {/* Metric */}
      {result.chart_type === 'metric' && (
        <div className="flex items-center justify-center py-8">
          <span className="text-5xl font-bold text-blue-600 dark:text-blue-400">
            {result.data[0]?.toLocaleString() ?? '—'}
          </span>
        </div>
      )}

      {/* Charts */}
      {isChartable && viewMode === 'chart' && (
        <>
          {result.chart_type === 'bar' && (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" opacity={0.2} />
                <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Bar dataKey="value" fill={CHART_COLORS[0]} animationDuration={300} radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
          {result.chart_type === 'line' && (
            <ResponsiveContainer width="100%" height={280}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" opacity={0.2} />
                <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Line type="monotone" dataKey="value" stroke={CHART_COLORS[0]} strokeWidth={2} dot={false} animationDuration={300} />
              </LineChart>
            </ResponsiveContainer>
          )}
          {result.chart_type === 'pie' && (
            <ResponsiveContainer width="100%" height={280}>
              <PieChart>
                <Pie data={chartData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} animationDuration={300}
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}>
                  {chartData.map((_, i) => <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />)}
                </Pie>
                <Tooltip />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          )}
        </>
      )}

      {/* Table view — shown when chart_type is table, or user switches to table, or data has no numeric values */}
      {(!isChartable || result.chart_type === 'table' || viewMode === 'table') && result.raw_data.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-700">
                {Object.keys(result.raw_data[0]).map(col => (
                  <th key={col} className="px-3 py-2 text-left text-xs font-semibold text-gray-500 dark:text-gray-400">
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {result.raw_data.slice(page * pageSize, (page + 1) * pageSize).map((row, i) => (
                <tr key={i} className="border-b border-gray-100 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-700/50">
                  {Object.values(row).map((val, j) => (
                    <td key={j} className="px-3 py-2 text-gray-700 dark:text-gray-300">{String(val ?? '')}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
          {result.raw_data.length > pageSize && (
            <div className="mt-3 flex items-center justify-between text-xs text-gray-500">
              <span>{page * pageSize + 1}–{Math.min((page + 1) * pageSize, result.raw_data.length)} of {result.raw_data.length}</span>
              <div className="flex gap-2">
                <button onClick={() => setPage(p => Math.max(0, p - 1))} disabled={page === 0}
                  className="rounded px-2 py-1 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700">Prev</button>
                <button onClick={() => setPage(p => p + 1)} disabled={(page + 1) * pageSize >= result.raw_data.length}
                  className="rounded px-2 py-1 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700">Next</button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Empty state */}
      {result.raw_data.length === 0 && (
        <div className="flex items-center justify-center py-10 text-sm text-gray-400">
          No rows returned.
        </div>
      )}

      {result.generated_sql && (
        <details className="mt-4">
          <summary className="cursor-pointer text-xs text-gray-400 hover:text-gray-600">View generated SQL</summary>
          <pre className="mt-2 overflow-x-auto rounded bg-gray-100 p-3 text-xs dark:bg-gray-900">{result.generated_sql}</pre>
        </details>
      )}
    </div>
  )
}
