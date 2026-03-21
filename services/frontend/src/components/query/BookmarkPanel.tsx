import { useState } from 'react'
import { Bookmark, RefreshCw, Trash2, Mail, ChevronDown, ChevronUp } from 'lucide-react'
import { Bookmark as BookmarkType, QueryResult, queryService } from '../../services/query.service'
import ConfirmModal from '../common/ConfirmModal'
import toast from 'react-hot-toast'

interface BookmarkPanelProps {
  bookmarks: BookmarkType[]
  onRefresh: () => void
  onView: (result: QueryResult) => void
}

export default function BookmarkPanel({ bookmarks, onRefresh, onView }: BookmarkPanelProps) {
  const [loadingId, setLoadingId] = useState<string | null>(null)
  const [emailModal, setEmailModal] = useState<{ id: string; title: string } | null>(null)
  const [emailInput, setEmailInput] = useState('')
  const [sendingEmail, setSendingEmail] = useState(false)
  const [expanded, setExpanded] = useState(true)
  const [confirmBm, setConfirmBm] = useState<BookmarkType | null>(null)

  const handleRefresh = async (bm: BookmarkType) => {
    setLoadingId(bm.id)
    try {
      const result = await queryService.refreshBookmark(bm.id)
      onView(result)
      toast.success(`"${bm.title}" refreshed`)
    } catch {
      toast.error('Failed to refresh bookmark')
    } finally {
      setLoadingId(null)
    }
  }

  const handleDelete = async (bm: BookmarkType) => {
    setConfirmBm(bm)
  }

  const handleConfirmDelete = async () => {
    if (!confirmBm) return
    const bm = confirmBm
    setConfirmBm(null)
    try {
      await queryService.deleteBookmark(bm.id)
      onRefresh()
      toast.success('Bookmark deleted')
    } catch {
      toast.error('Failed to delete bookmark')
    }
  }

  const handleSendEmail = async () => {
    if (!emailModal || !emailInput.trim()) return
    setSendingEmail(true)
    try {
      await queryService.sendEmailReport(emailModal.id, emailInput.trim())
      toast.success(`Report sent to ${emailInput.trim()}`)
      setEmailModal(null)
      setEmailInput('')
    } catch {
      toast.error('Failed to send email report')
    } finally {
      setSendingEmail(false)
    }
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <button
        onClick={() => setExpanded(e => !e)}
        className="flex w-full items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-gray-700"
      >
        <div className="flex items-center gap-2">
          <Bookmark className="h-4 w-4 text-blue-500" />
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">
            Bookmarks ({bookmarks.length})
          </span>
        </div>
        {expanded ? <ChevronUp className="h-4 w-4 text-gray-400" /> : <ChevronDown className="h-4 w-4 text-gray-400" />}
      </button>

      {expanded && (
        bookmarks.length === 0 ? (
          <p className="px-4 py-5 text-sm text-gray-400">No bookmarks yet. Save a query result to bookmark it.</p>
        ) : (
          <ul className="divide-y divide-gray-100 dark:divide-gray-700">
            {bookmarks.map(bm => (
              <li key={bm.id} className="flex items-center justify-between gap-2 px-4 py-3">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-gray-800 dark:text-gray-200">{bm.title}</p>
                  <p className="truncate text-xs text-gray-400">{bm.query_text}</p>
                  <span className="mt-0.5 inline-block rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                    {bm.chart_type}
                  </span>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <button
                    onClick={() => handleRefresh(bm)}
                    disabled={loadingId === bm.id}
                    title="Refresh live data"
                    className="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-blue-600 disabled:opacity-40 dark:hover:bg-gray-700"
                  >
                    <RefreshCw className={`h-3.5 w-3.5 ${loadingId === bm.id ? 'animate-spin' : ''}`} />
                  </button>
                  <button
                    onClick={() => setEmailModal({ id: bm.id, title: bm.title })}
                    title="Send email report"
                    className="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-green-600 dark:hover:bg-gray-700"
                  >
                    <Mail className="h-3.5 w-3.5" />
                  </button>
                  <button
                    onClick={() => handleDelete(bm)}
                    title="Delete bookmark"
                    className="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-red-600 dark:hover:bg-gray-700"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )
      )}

      {/* Email modal */}
      {emailModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="w-full max-w-sm rounded-xl bg-white p-6 shadow-xl dark:bg-gray-800">
            <h3 className="mb-1 text-base font-semibold text-gray-900 dark:text-white">Send Report</h3>
            <p className="mb-4 text-sm text-gray-500">Send "{emailModal.title}" as an email report</p>
            <input
              type="email"
              value={emailInput}
              onChange={e => setEmailInput(e.target.value)}
              placeholder="recipient@example.com"
              className="mb-4 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => { setEmailModal(null); setEmailInput('') }}
                className="rounded-md px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                onClick={handleSendEmail}
                disabled={sendingEmail || !emailInput.trim()}
                className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                <Mail className="h-3.5 w-3.5" />
                {sendingEmail ? 'Sending...' : 'Send'}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmBm && (
        <ConfirmModal
          title="Delete Bookmark"
          message={`Delete bookmark "${confirmBm.title}"? This cannot be undone.`}
          confirmLabel="Delete"
          onConfirm={handleConfirmDelete}
          onCancel={() => setConfirmBm(null)}
        />
      )}
    </div>
  )
}
