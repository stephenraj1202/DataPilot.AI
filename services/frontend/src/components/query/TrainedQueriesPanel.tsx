import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Pencil, Check, X, BookOpen, ToggleLeft, ToggleRight, ChevronDown, ChevronRight } from 'lucide-react'
import { queryService, type TrainedQuery, type DatabaseConnection } from '../../services/query.service'
import toast from 'react-hot-toast'

interface Props {
  connections: DatabaseConnection[]
}

export default function TrainedQueriesPanel({ connections }: Props) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [selectedConn, setSelectedConn] = useState(connections[0]?.id ?? '')
  const [editId, setEditId] = useState<string | null>(null)
  const [showAdd, setShowAdd] = useState(false)
  const [form, setForm] = useState({ question: '', sql_query: '', description: '' })

  const { data: trained = [], isLoading } = useQuery({
    queryKey: ['trained-queries', selectedConn],
    queryFn: () => queryService.getTrainedQueries(selectedConn || undefined),
    enabled: open,
  })

  const { mutate: create, isPending: creating } = useMutation({
    mutationFn: () => queryService.createTrainedQuery({ ...form, connection_id: selectedConn }),
    onSuccess: () => {
      toast.success('Trained query saved')
      setShowAdd(false)
      setForm({ question: '', sql_query: '', description: '' })
      qc.invalidateQueries({ queryKey: ['trained-queries'] })
    },
    onError: () => toast.error('Failed to save'),
  })

  const { mutate: update } = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: Partial<TrainedQuery> }) =>
      queryService.updateTrainedQuery(id, payload),
    onSuccess: () => { toast.success('Updated'); setEditId(null); qc.invalidateQueries({ queryKey: ['trained-queries'] }) },
    onError: () => toast.error('Failed to update'),
  })

  const { mutate: remove } = useMutation({
    mutationFn: (id: string) => queryService.deleteTrainedQuery(id),
    onSuccess: () => { toast.success('Deleted'); qc.invalidateQueries({ queryKey: ['trained-queries'] }) },
    onError: () => toast.error('Failed to delete'),
  })

  const [editForm, setEditForm] = useState<{ question: string; sql_query: string; description: string }>({ question: '', sql_query: '', description: '' })

  const startEdit = (tq: TrainedQuery) => {
    setEditId(tq.id)
    setEditForm({ question: tq.question, sql_query: tq.sql_query, description: tq.description ?? '' })
  }

  return (
    <div className="rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
      {/* Header toggle */}
      <button
        onClick={() => setOpen(o => !o)}
        className="flex w-full items-center gap-3 px-5 py-4 text-left"
      >
        <BookOpen className="h-5 w-5 text-indigo-500" />
        <div className="flex-1">
          <p className="text-sm font-semibold text-gray-800 dark:text-white">Trained Queries</p>
          <p className="text-xs text-gray-400">Admin-defined question → SQL mappings</p>
        </div>
        {open ? <ChevronDown className="h-4 w-4 text-gray-400" /> : <ChevronRight className="h-4 w-4 text-gray-400" />}
      </button>

      {open && (
        <div className="border-t border-gray-100 px-5 pb-5 pt-4 dark:border-gray-700">
          {/* Connection selector + Add button */}
          <div className="mb-4 flex items-center gap-3">
            <select
              value={selectedConn}
              onChange={e => setSelectedConn(e.target.value)}
              className="flex-1 rounded-lg border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            >
              {connections.map(c => (
                <option key={c.id} value={c.id}>{c.connection_name} ({c.db_type})</option>
              ))}
            </select>
            <button
              onClick={() => setShowAdd(a => !a)}
              className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-700"
            >
              <Plus className="h-3.5 w-3.5" /> Add
            </button>
          </div>

          {/* Add form */}
          {showAdd && (
            <div className="mb-4 rounded-xl border border-indigo-200 bg-indigo-50 p-4 space-y-3 dark:border-indigo-800/40 dark:bg-indigo-900/10">
              <p className="text-xs font-semibold uppercase tracking-wide text-indigo-600 dark:text-indigo-400">New Trained Query</p>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Question (natural language)</label>
                <input
                  value={form.question}
                  onChange={e => setForm(f => ({ ...f, question: e.target.value }))}
                  placeholder="e.g. Show total revenue by month"
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">SQL Query</label>
                <textarea
                  rows={4}
                  value={form.sql_query}
                  onChange={e => setForm(f => ({ ...f, sql_query: e.target.value }))}
                  placeholder="SELECT ..."
                  className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 font-mono text-xs focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">Description (optional)</label>
                <input
                  value={form.description}
                  onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
                  placeholder="Brief description"
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                />
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => create()}
                  disabled={creating || !form.question.trim() || !form.sql_query.trim()}
                  className="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-4 py-2 text-xs font-semibold text-white hover:bg-indigo-700 disabled:opacity-50"
                >
                  <Check className="h-3.5 w-3.5" /> Save
                </button>
                <button
                  onClick={() => { setShowAdd(false); setForm({ question: '', sql_query: '', description: '' }) }}
                  className="flex items-center gap-1.5 rounded-lg border border-gray-300 px-4 py-2 text-xs hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                >
                  <X className="h-3.5 w-3.5" /> Cancel
                </button>
              </div>
            </div>
          )}

          {/* List */}
          {isLoading ? (
            <p className="text-center text-xs text-gray-400 py-4">Loading...</p>
          ) : trained.length === 0 ? (
            <p className="text-center text-xs text-gray-400 py-6">No trained queries yet. Add one above.</p>
          ) : (
            <div className="space-y-2">
              {trained.map(tq => (
                <div key={tq.id} className="rounded-xl border border-gray-100 bg-gray-50 p-3 dark:border-gray-700/50 dark:bg-gray-800/50">
                  {editId === tq.id ? (
                    <div className="space-y-2">
                      <input
                        value={editForm.question}
                        onChange={e => setEditForm(f => ({ ...f, question: e.target.value }))}
                        className="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                      />
                      <textarea
                        rows={3}
                        value={editForm.sql_query}
                        onChange={e => setEditForm(f => ({ ...f, sql_query: e.target.value }))}
                        className="w-full resize-none rounded-lg border border-gray-300 px-3 py-1.5 font-mono text-xs focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                      />
                      <input
                        value={editForm.description}
                        onChange={e => setEditForm(f => ({ ...f, description: e.target.value }))}
                        placeholder="Description"
                        className="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                      />
                      <div className="flex gap-2">
                        <button onClick={() => update({ id: tq.id, payload: editForm })}
                          className="flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-700">
                          <Check className="h-3 w-3" /> Save
                        </button>
                        <button onClick={() => setEditId(null)}
                          className="flex items-center gap-1 rounded-lg border border-gray-300 px-3 py-1.5 text-xs hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300">
                          <X className="h-3 w-3" /> Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-start gap-3">
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-semibold text-gray-800 dark:text-white">{tq.question}</p>
                        {tq.description && <p className="mt-0.5 text-xs text-gray-400">{tq.description}</p>}
                        <pre className="mt-1.5 overflow-x-auto rounded-lg bg-gray-100 px-3 py-2 font-mono text-xs text-gray-700 dark:bg-gray-700 dark:text-gray-300">{tq.sql_query}</pre>
                        <p className="mt-1 text-xs text-gray-400">Matched {tq.match_count} time{tq.match_count !== 1 ? 's' : ''}</p>
                      </div>
                      <div className="flex flex-shrink-0 items-center gap-1">
                        <button
                          onClick={() => update({ id: tq.id, payload: { is_active: !tq.is_active } })}
                          title={tq.is_active ? 'Disable' : 'Enable'}
                          className={`rounded p-1 ${tq.is_active ? 'text-green-500 hover:text-green-600' : 'text-gray-400 hover:text-gray-500'}`}
                        >
                          {tq.is_active ? <ToggleRight className="h-4 w-4" /> : <ToggleLeft className="h-4 w-4" />}
                        </button>
                        <button onClick={() => startEdit(tq)} className="rounded p-1 text-gray-400 hover:text-blue-500">
                          <Pencil className="h-3.5 w-3.5" />
                        </button>
                        <button onClick={() => remove(tq.id)} className="rounded p-1 text-gray-400 hover:text-red-500">
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
