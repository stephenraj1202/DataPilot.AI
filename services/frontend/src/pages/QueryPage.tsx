import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { queryService, QueryResult } from '../services/query.service'
import QueryInput from '../components/query/QueryInput'
import ResultChart from '../components/query/ResultChart'
import BookmarkPanel from '../components/query/BookmarkPanel'
import TrainedQueriesPanel from '../components/query/TrainedQueriesPanel'
import LoadingSpinner from '../components/common/LoadingSpinner'
import toast from 'react-hot-toast'
import { useAuth } from '../context/AuthContext'

export default function QueryPage() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'Super_Admin' || user?.role === 'Admin'
  const [result, setResult] = useState<QueryResult | null>(null)
  const [activeConnectionId, setActiveConnectionId] = useState<string>('')
  const [activeQueryText, setActiveQueryText] = useState<string>('')
  const [prefilledQuery, setPrefilledQuery] = useState<string>('')

  const { data: connectionsData, isLoading: connectionsLoading } = useQuery({
    queryKey: ['connections'],
    queryFn: () => queryService.getConnections(),
  })

  const { refetch: refetchHistory } = useQuery({
    queryKey: ['query-history'],
    queryFn: () => queryService.getQueryHistory(20),
    enabled: false, // only refetch on demand
  })

  const { data: bookmarksData, refetch: refetchBookmarks } = useQuery({
    queryKey: ['bookmarks'],
    queryFn: () => queryService.getBookmarks(),
  })

  const { mutate: executeQuery, isPending } = useMutation({
    mutationFn: ({ connectionId, query }: { connectionId: string; query: string }) =>
      queryService.executeQuery(connectionId, query),
    onSuccess: (data, variables) => {
      setResult(data)
      setActiveConnectionId(variables.connectionId)
      setActiveQueryText(variables.query)
      refetchHistory()
    },
    onError: (err: unknown) => {
      const msg =
        (err as { response?: { data?: { detail?: string; error?: string } } })?.response?.data?.detail ??
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        'Query failed'
      toast.error(msg)
    },
  })

  const handleSubmit = (connectionId: string, query: string) => {
    executeQuery({ connectionId, query })
  }

  if (connectionsLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  const connections = connectionsData?.connections ?? []
  const bookmarks = bookmarksData ?? []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">AI Query Tool</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Ask questions in plain English and get instant insights
        </p>
      </div>

      {connections.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-300 bg-white p-8 text-center dark:border-gray-600 dark:bg-gray-800">
          <p className="text-gray-500 dark:text-gray-400">
            No database connections found. Add one in{' '}
            <a href="/settings" className="text-blue-600 hover:underline">Settings</a>.
          </p>
        </div>
      ) : (
        <QueryInput
          connections={connections}
          onSubmit={handleSubmit}
          isLoading={isPending}
          prefilledQuery={prefilledQuery}
          onQueryChange={setPrefilledQuery}
        />
      )}

      {isPending && (
        <div className="flex items-center justify-center gap-3 rounded-xl border border-gray-200 bg-white p-8 dark:border-gray-700 dark:bg-gray-800">
          <LoadingSpinner size="md" />
          <p className="text-gray-600 dark:text-gray-400">Generating SQL and executing query...</p>
        </div>
      )}

      {result && !isPending && (
        <ResultChart
          result={result}
          connectionId={activeConnectionId}
          queryText={activeQueryText}
          onBookmarkSaved={refetchBookmarks}
        />
      )}

      <BookmarkPanel
        bookmarks={bookmarks}
        onRefresh={refetchBookmarks}
        onView={r => setResult(r)}
      />

      {/* Admin: trained queries panel */}
      {isAdmin && connections.length > 0 && (
        <TrainedQueriesPanel connections={connections} />
      )}
    </div>
  )
}
