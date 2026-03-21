import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { queryService, type DatabaseConnection } from '../services/query.service'
import DBConnectWizard from '../components/query/DBConnectWizard'
import DBChatPanel from '../components/query/DBChatPanel'
import BookmarkPanel from '../components/query/BookmarkPanel'
import TrainedQueriesPanel from '../components/query/TrainedQueriesPanel'
import LoadingSpinner from '../components/common/LoadingSpinner'
import { useAuth } from '../context/AuthContext'
import { Database, Plus, Trash2, CheckCircle2, Wifi } from 'lucide-react'
import toast from 'react-hot-toast'

const DB_COLOR: Record<string, string> = {
  postgresql: 'from-blue-500 to-indigo-600',
  mysql: 'from-orange-400 to-amber-500',
  mongodb: 'from-green-500 to-emerald-600',
  sqlserver: 'from-red-500 to-rose-600',
}

const DB_DOT: Record<string, string> = {
  postgresql: 'bg-blue-500',
  mysql: 'bg-orange-500',
  mongodb: 'bg-green-500',
  sqlserver: 'bg-red-500',
}

export default function QueryPage() {
  const { user } = useAuth()
  const qc = useQueryClient()
  const isAdmin = user?.role === 'Super_Admin' || user?.role === 'Admin'
  const [showWizard, setShowWizard] = useState(false)
  const [activeConnId, setActiveConnId] = useState<string | null>(null)

  const { data: connectionsData, isLoading: connectionsLoading, refetch: refetchConns } = useQuery({
    queryKey: ['connections'],
    queryFn: () => queryService.getConnections(),
  })

  const { data: bookmarksData, refetch: refetchBookmarks } = useQuery({
    queryKey: ['bookmarks'],
    queryFn: () => queryService.getBookmarks(),
  })

  const connections = connectionsData?.connections ?? []
  const bookmarks = bookmarksData ?? []

  const handleConnected = (conn: DatabaseConnection) => {
    refetchConns()
    qc.invalidateQueries({ queryKey: ['connections'] })
    setActiveConnId(conn.id)
    toast.success(`${conn.connection_name} connected!`)
  }

  const handleDelete = async (id: string, name: string) => {
    try {
      await queryService.deleteConnection(id)
      refetchConns()
      qc.invalidateQueries({ queryKey: ['connections'] })
      toast.success(`${name} removed`)
    } catch {
      toast.error('Failed to remove connection')
    }
  }

  if (connectionsLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">AI Query Tool</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Ask questions in plain English and get instant insights
          </p>
        </div>
        <button
          onClick={() => setShowWizard(true)}
          className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-indigo-500 to-violet-600 px-4 py-2.5 text-sm font-semibold text-white shadow-lg hover:opacity-90 transition-all"
        >
          <Plus className="h-4 w-4" />
          Add Database
        </button>
      </div>

      {/* Connection cards */}
      {connections.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {connections.map(conn => {
            const isActive = activeConnId === conn.id
            const grad = DB_COLOR[conn.db_type] ?? 'from-indigo-500 to-violet-600'
            const dot = DB_DOT[conn.db_type] ?? 'bg-indigo-500'
            return (
              <div
                key={conn.id}
                onClick={() => setActiveConnId(conn.id)}
                className={`group relative cursor-pointer rounded-2xl border-2 p-4 transition-all hover:shadow-lg ${
                  isActive
                    ? 'border-indigo-400 dark:border-indigo-500 shadow-md shadow-indigo-100 dark:shadow-indigo-900/30'
                    : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                }`}
              >
                {/* Gradient icon */}
                <div className={`mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br ${grad} shadow`}>
                  <Database className="h-5 w-5 text-white" />
                </div>

                <p className="text-sm font-semibold text-gray-800 dark:text-white truncate">{conn.connection_name}</p>
                <p className="text-xs text-gray-400 truncate">{conn.db_type} · {conn.host}</p>

                {/* Status dot */}
                <div className="mt-2 flex items-center gap-1.5">
                  <span className={`h-2 w-2 rounded-full ${conn.status === 'active' ? dot : 'bg-gray-300'}`} />
                  <span className="text-[10px] text-gray-400 capitalize">{conn.status}</span>
                </div>

                {/* Active badge */}
                {isActive && (
                  <div className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-indigo-500 shadow">
                    <CheckCircle2 className="h-3.5 w-3.5 text-white" />
                  </div>
                )}

                {/* Delete button */}
                <button
                  onClick={e => { e.stopPropagation(); handleDelete(conn.id, conn.connection_name) }}
                  className="absolute right-2 bottom-2 hidden group-hover:flex h-6 w-6 items-center justify-center rounded-lg bg-red-50 dark:bg-red-900/20 text-red-500 hover:bg-red-100 dark:hover:bg-red-900/40 transition-colors"
                >
                  <Trash2 className="h-3 w-3" />
                </button>
              </div>
            )
          })}

          {/* Add new card */}
          <button
            onClick={() => setShowWizard(true)}
            className="flex flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed border-gray-200 dark:border-gray-700 p-4 text-gray-400 hover:border-indigo-300 hover:text-indigo-500 dark:hover:border-indigo-700 dark:hover:text-indigo-400 transition-all hover:bg-indigo-50/50 dark:hover:bg-indigo-900/10"
          >
            <Plus className="h-6 w-6" />
            <span className="text-xs font-medium">Add Database</span>
          </button>
        </div>
      )}

      {/* Empty state */}
      {connections.length === 0 && (
        <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 py-16 text-center">
          <div className="mb-4 flex h-20 w-20 items-center justify-center rounded-3xl bg-gradient-to-br from-indigo-500 to-violet-600 shadow-xl shadow-indigo-200 dark:shadow-indigo-900/40">
            <Database className="h-10 w-10 text-white" />
          </div>
          <h3 className="mb-1 text-lg font-bold text-gray-800 dark:text-white">No databases connected</h3>
          <p className="mb-6 text-sm text-gray-400">Connect your first database to start querying with AI</p>
          <button
            onClick={() => setShowWizard(true)}
            className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-indigo-500 to-violet-600 px-6 py-3 text-sm font-bold text-white shadow-lg hover:opacity-90 transition-all"
          >
            <Plus className="h-4 w-4" />
            Connect Database
          </button>
        </div>
      )}

      {/* Chat panel — shown when a connection is selected */}
      {connections.length > 0 && (
        <DBChatPanel
          connections={connections}
          onAddConnection={() => setShowWizard(true)}
        />
      )}

      {/* Bookmarks */}
      <BookmarkPanel
        bookmarks={bookmarks}
        onRefresh={refetchBookmarks}
        onView={(_r) => {}}
      />

      {/* Admin: trained queries */}
      {isAdmin && connections.length > 0 && (
        <TrainedQueriesPanel connections={connections} />
      )}

      {/* Wizard modal */}
      {showWizard && (
        <DBConnectWizard
          onClose={() => setShowWizard(false)}
          onConnected={handleConnected}
        />
      )}
    </div>
  )
}
