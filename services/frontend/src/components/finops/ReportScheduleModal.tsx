import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { X, Plus, Trash2, Send, Clock, Mail, Calendar } from 'lucide-react'
import { finopsService, type ReportSchedule, type ReportSchedulePayload } from '../../services/finops.service'
import ConfirmModal from '../common/ConfirmModal'
import toast from 'react-hot-toast'

const REPORT_TYPES = [
  { value: 'full', label: 'Full Report (Costs + Anomalies + Recommendations)' },
  { value: 'cost_summary', label: 'Cost Summary Only' },
  { value: 'anomalies', label: 'Anomaly Report Only' },
  { value: 'recommendations', label: 'Recommendations Only' },
]

const FREQUENCIES = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
]

const DAYS_OF_WEEK = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const HOURS = Array.from({ length: 24 }, (_, i) => ({
  value: i,
  label: `${i.toString().padStart(2, '0')}:00 UTC`,
}))

interface Props {
  onClose: () => void
}

function ScheduleForm({
  initial,
  onSave,
  onCancel,
  saving,
}: {
  initial?: ReportSchedule
  onSave: (p: ReportSchedulePayload) => void
  onCancel: () => void
  saving: boolean
}) {
  const [name, setName] = useState(initial?.name ?? '')
  const [frequency, setFrequency] = useState<'daily' | 'weekly' | 'monthly'>(initial?.frequency ?? 'weekly')
  const [dayOfWeek, setDayOfWeek] = useState<number>(initial?.day_of_week ?? 1)
  const [dayOfMonth, setDayOfMonth] = useState<number>(initial?.day_of_month ?? 1)
  const [sendHour, setSendHour] = useState<number>(initial?.send_hour ?? 8)
  const [reportType, setReportType] = useState(initial?.report_type ?? 'full')
  const [isActive, setIsActive] = useState(initial?.is_active ?? true)
  const [recipientInput, setRecipientInput] = useState('')
  const [recipients, setRecipients] = useState<string[]>(initial?.recipients ?? [])

  function addRecipient() {
    const email = recipientInput.trim().toLowerCase()
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      toast.error('Enter a valid email address')
      return
    }
    if (recipients.includes(email)) return
    setRecipients([...recipients, email])
    setRecipientInput('')
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) { toast.error('Schedule name is required'); return }
    if (recipients.length === 0) { toast.error('Add at least one recipient'); return }
    onSave({
      name: name.trim(),
      frequency,
      day_of_week: frequency === 'weekly' ? dayOfWeek : null,
      day_of_month: frequency === 'monthly' ? dayOfMonth : null,
      send_hour: sendHour,
      recipients,
      report_type: reportType,
      is_active: isActive,
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {/* Name */}
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Schedule Name
        </label>
        <input
          type="text"
          value={name}
          onChange={e => setName(e.target.value)}
          placeholder="e.g. Weekly Cost Report"
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
      </div>

      {/* Report Type */}
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Report Type
        </label>
        <select
          value={reportType}
          onChange={e => setReportType(e.target.value)}
          className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
        >
          {REPORT_TYPES.map(rt => (
            <option key={rt.value} value={rt.value}>{rt.label}</option>
          ))}
        </select>
      </div>

      {/* Frequency + timing */}
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Frequency
          </label>
          <select
            value={frequency}
            onChange={e => setFrequency(e.target.value as 'daily' | 'weekly' | 'monthly')}
            className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            {FREQUENCIES.map(f => (
              <option key={f.value} value={f.value}>{f.label}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Send Time
          </label>
          <select
            value={sendHour}
            onChange={e => setSendHour(Number(e.target.value))}
            className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            {HOURS.map(h => (
              <option key={h.value} value={h.value}>{h.label}</option>
            ))}
          </select>
        </div>
      </div>

      {frequency === 'weekly' && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Day of Week
          </label>
          <div className="flex flex-wrap gap-2">
            {DAYS_OF_WEEK.map((day, i) => (
              <button
                key={i}
                type="button"
                onClick={() => setDayOfWeek(i)}
                className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                  dayOfWeek === i
                    ? 'bg-indigo-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                }`}
              >
                {day.slice(0, 3)}
              </button>
            ))}
          </div>
        </div>
      )}

      {frequency === 'monthly' && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Day of Month
          </label>
          <input
            type="number"
            min={1}
            max={28}
            value={dayOfMonth}
            onChange={e => setDayOfMonth(Number(e.target.value))}
            className="w-24 px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">(1–28)</span>
        </div>
      )}

      {/* Recipients */}
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Recipients
        </label>
        <div className="flex gap-2 mb-2">
          <input
            type="email"
            value={recipientInput}
            onChange={e => setRecipientInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addRecipient() } }}
            placeholder="user@example.com"
            className="flex-1 px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <button
            type="button"
            onClick={addRecipient}
            className="px-3 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg text-sm flex items-center gap-1"
          >
            <Plus className="w-4 h-4" />
            Add
          </button>
        </div>
        {recipients.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {recipients.map(r => (
              <span
                key={r}
                className="inline-flex items-center gap-1 px-2 py-1 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300 rounded-full text-xs"
              >
                <Mail className="w-3 h-3" />
                {r}
                <button
                  type="button"
                  onClick={() => setRecipients(recipients.filter(x => x !== r))}
                  className="ml-1 hover:text-red-500"
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Active toggle */}
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => setIsActive(!isActive)}
          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
            isActive ? 'bg-indigo-600' : 'bg-gray-300 dark:bg-gray-600'
          }`}
        >
          <span
            className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
              isActive ? 'translate-x-4' : 'translate-x-1'
            }`}
          />
        </button>
        <span className="text-sm text-gray-700 dark:text-gray-300">
          {isActive ? 'Active' : 'Paused'}
        </span>
      </div>

      <div className="flex justify-end gap-2 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={saving}
          className="px-4 py-2 text-sm bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2"
        >
          {saving ? 'Saving…' : initial ? 'Update Schedule' : 'Create Schedule'}
        </button>
      </div>
    </form>
  )
}

export default function ReportScheduleModal({ onClose }: Props) {
  const qc = useQueryClient()
  const [view, setView] = useState<'list' | 'create' | 'edit' | 'send'>('list')
  const [editing, setEditing] = useState<ReportSchedule | null>(null)
  const [confirmSchedule, setConfirmSchedule] = useState<ReportSchedule | null>(null)

  // Send now state
  const [sendRecipientInput, setSendRecipientInput] = useState('')
  const [sendRecipients, setSendRecipients] = useState<string[]>([])
  const [sendReportType, setSendReportType] = useState('full')

  const { data, isLoading } = useQuery({
    queryKey: ['report-schedules'],
    queryFn: () => finopsService.getReportSchedules(),
  })

  const createMut = useMutation({
    mutationFn: finopsService.createReportSchedule,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['report-schedules'] }); setView('list'); toast.success('Schedule created') },
    onError: () => toast.error('Failed to create schedule'),
  })

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: ReportSchedulePayload }) =>
      finopsService.updateReportSchedule(id, payload),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['report-schedules'] }); setView('list'); toast.success('Schedule updated') },
    onError: () => toast.error('Failed to update schedule'),
  })

  const deleteMut = useMutation({
    mutationFn: finopsService.deleteReportSchedule,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['report-schedules'] }); toast.success('Schedule deleted') },
    onError: () => toast.error('Failed to delete schedule'),
  })

  const sendMut = useMutation({
    mutationFn: finopsService.sendReportNow,
    onSuccess: () => { toast.success('Report sent successfully'); setView('list') },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
      toast.error(msg ? `Send failed: ${msg}` : 'Failed to send report')
    },
  })

  function addSendRecipient() {
    const email = sendRecipientInput.trim().toLowerCase()
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) { toast.error('Enter a valid email'); return }
    if (sendRecipients.includes(email)) return
    setSendRecipients([...sendRecipients, email])
    setSendRecipientInput('')
  }

  const schedules = data?.schedules ?? []

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-full max-w-xl max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2">
            <div className="p-2 bg-indigo-100 dark:bg-indigo-900/30 rounded-lg">
              <Mail className="w-5 h-5 text-indigo-600 dark:text-indigo-400" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                {view === 'send' ? 'Send Report Now' : view === 'create' ? 'New Schedule' : view === 'edit' ? 'Edit Schedule' : 'Report Schedules'}
              </h2>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                {view === 'list' ? 'Automate FinOps reports via email' : 'Configure report delivery'}
              </p>
            </div>
          </div>
          <button onClick={onClose} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg">
            <X className="w-5 h-5 text-gray-500" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4">
          {/* LIST VIEW */}
          {view === 'list' && (
            <div className="space-y-4">
              {/* Action buttons */}
              <div className="flex gap-2">
                <button
                  onClick={() => setView('send')}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium transition-colors"
                >
                  <Send className="w-4 h-4" />
                  Send Now
                </button>
                <button
                  onClick={() => setView('create')}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors"
                >
                  <Clock className="w-4 h-4" />
                  New Schedule
                </button>
              </div>

              {isLoading ? (
                <div className="text-center py-8 text-gray-400 text-sm">Loading schedules…</div>
              ) : schedules.length === 0 ? (
                <div className="text-center py-10">
                  <Calendar className="w-10 h-10 text-gray-300 dark:text-gray-600 mx-auto mb-3" />
                  <p className="text-sm text-gray-500 dark:text-gray-400">No schedules yet.</p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">Create one to automate report delivery.</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {schedules.map(s => (
                    <div
                      key={s.id}
                      className="p-4 rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50"
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1">
                            <span className="font-medium text-sm text-gray-900 dark:text-white truncate">{s.name}</span>
                            <span className={`shrink-0 px-2 py-0.5 rounded-full text-xs font-medium ${
                              s.is_active
                                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                                : 'bg-gray-200 text-gray-500 dark:bg-gray-600 dark:text-gray-400'
                            }`}>
                              {s.is_active ? 'Active' : 'Paused'}
                            </span>
                          </div>
                          <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                            <span className="capitalize">{s.frequency}</span>
                            <span>·</span>
                            <span>{REPORT_TYPES.find(r => r.value === s.report_type)?.label.split(' (')[0]}</span>
                            <span>·</span>
                            <span>{s.recipients.length} recipient{s.recipients.length !== 1 ? 's' : ''}</span>
                          </div>
                          <div className="mt-1 text-xs text-gray-400 dark:text-gray-500">
                            Next: {new Date(s.next_run_at).toLocaleString()}
                            {s.last_sent_at && ` · Last sent: ${new Date(s.last_sent_at).toLocaleDateString()}`}
                          </div>
                        </div>
                        <div className="flex gap-1 shrink-0">
                          <button
                            onClick={() => { setEditing(s); setView('edit') }}
                            className="p-1.5 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg text-gray-500 dark:text-gray-400"
                          >
                            <Calendar className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => setConfirmSchedule(s)}
                            className="p-1.5 hover:bg-red-100 dark:hover:bg-red-900/30 rounded-lg text-gray-500 hover:text-red-600 dark:hover:text-red-400"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* CREATE VIEW */}
          {view === 'create' && (
            <ScheduleForm
              onSave={p => createMut.mutate(p)}
              onCancel={() => setView('list')}
              saving={createMut.isPending}
            />
          )}

          {/* EDIT VIEW */}
          {view === 'edit' && editing && (
            <ScheduleForm
              initial={editing}
              onSave={p => updateMut.mutate({ id: editing.id, payload: p })}
              onCancel={() => { setView('list'); setEditing(null) }}
              saving={updateMut.isPending}
            />
          )}

          {/* SEND NOW VIEW */}
          {view === 'send' && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Report Type
                </label>
                <select
                  value={sendReportType}
                  onChange={e => setSendReportType(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                >
                  {REPORT_TYPES.map(rt => (
                    <option key={rt.value} value={rt.value}>{rt.label}</option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Recipients
                </label>
                <div className="flex gap-2 mb-2">
                  <input
                    type="email"
                    value={sendRecipientInput}
                    onChange={e => setSendRecipientInput(e.target.value)}
                    onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addSendRecipient() } }}
                    placeholder="user@example.com"
                    className="flex-1 px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                  <button
                    type="button"
                    onClick={addSendRecipient}
                    className="px-3 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg text-sm flex items-center gap-1"
                  >
                    <Plus className="w-4 h-4" />
                    Add
                  </button>
                </div>
                {sendRecipients.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {sendRecipients.map(r => (
                      <span
                        key={r}
                        className="inline-flex items-center gap-1 px-2 py-1 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300 rounded-full text-xs"
                      >
                        <Mail className="w-3 h-3" />
                        {r}
                        <button
                          type="button"
                          onClick={() => setSendRecipients(sendRecipients.filter(x => x !== r))}
                          className="ml-1 hover:text-red-500"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      </span>
                    ))}
                  </div>
                )}
              </div>

              <div className="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-xs text-blue-700 dark:text-blue-300">
                The report will be sent using your account's configured SMTP settings. If none are configured, the platform default will be used.
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  onClick={() => setView('list')}
                  className="px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                >
                  Back
                </button>
                <button
                  onClick={() => {
                    if (sendRecipients.length === 0) { toast.error('Add at least one recipient'); return }
                    sendMut.mutate({ recipients: sendRecipients, report_type: sendReportType })
                  }}
                  disabled={sendMut.isPending}
                  className="px-4 py-2 text-sm bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white rounded-lg flex items-center gap-2"
                >
                  <Send className="w-4 h-4" />
                  {sendMut.isPending ? 'Sending…' : 'Send Report'}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {confirmSchedule && (
        <ConfirmModal
          title="Delete Schedule"
          message={`Delete schedule "${confirmSchedule.name}"? This cannot be undone.`}
          confirmLabel="Delete"
          onConfirm={() => { deleteMut.mutate(confirmSchedule.id); setConfirmSchedule(null) }}
          onCancel={() => setConfirmSchedule(null)}
        />
      )}
    </div>
  )
}
