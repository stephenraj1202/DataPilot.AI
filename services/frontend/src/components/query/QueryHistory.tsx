import { Clock } from 'lucide-react'
import { QueryHistoryItem } from '../../services/query.service'
import { formatDate } from '../../utils/formatters'

interface QueryHistoryProps {
  items: QueryHistoryItem[]
  onSelect: (query: string) => void
}

export default function QueryHistory({ items, onSelect }: QueryHistoryProps) {
  if (items.length === 0) {
    return (
      <div className="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800">
        <p className="text-sm text-gray-500">No query history yet.</p>
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="border-b border-gray-200 px-4 py-3 dark:border-gray-700">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Recent Queries</h3>
      </div>
      <ul className="divide-y divide-gray-100 dark:divide-gray-700">
        {items.map(item => (
          <li key={item.id}>
            <button
              onClick={() => onSelect(item.query_text)}
              className="w-full px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-700/50"
            >
              <div className="flex items-start justify-between gap-2">
                <p className="text-sm text-gray-800 dark:text-gray-200 line-clamp-2">{item.query_text}</p>
                <span
                  className={`flex-shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
                    item.status === 'success'
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                  }`}
                >
                  {item.status}
                </span>
              </div>
              <div className="mt-1 flex items-center gap-3 text-xs text-gray-400">
                <span className="flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  {formatDate(item.created_at)}
                </span>
                <span>{item.execution_time_ms}ms</span>
                <span>{item.result_count} rows</span>
              </div>
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
