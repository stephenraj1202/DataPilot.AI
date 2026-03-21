import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Zap, Copy, CheckCircle, Activity, DollarSign, FileText, X, Play, RefreshCw, CreditCard, FlaskConical, ExternalLink } from 'lucide-react'
import { ubbService, type UBBStream, type DryRunInvoice } from '../services/ubb.service'
import LoadingSpinner from '../components/common/LoadingSpinner'
import ConfirmModal from '../components/common/ConfirmModal'
import toast from 'react-hot-toast'

// ── Create Stream Modal ───────────────────────────────────────────────────────
function CreateStreamModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [streamName, setStreamName] = useState('')
  const [resolverID, setResolverID] = useState('')
  const [includedUnits, setIncludedUnits] = useState(1000)
  const [overageCents, setOverageCents] = useState(4)

  const mut = useMutation({
    mutationFn: () => ubbService.createStream({
      stream_name: streamName.trim(),
      resolver_id: resolverID.trim(),
      included_units: includedUnits,
      overage_price_cents: overageCents,
    }),
    onSuccess: () => { toast.success('Stream created'); onCreated(); onClose() },
    onError: () => toast.error('Failed to create stream'),
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-md rounded-2xl bg-white dark:bg-gray-800 shadow-2xl">
        <div className="flex items-center justify-between border-b border-gray-200 dark:border-gray-700 px-6 py-4">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-violet-100 dark:bg-violet-900/40">
              <Zap className="h-4 w-4 text-violet-600 dark:text-violet-400" />
            </div>
            <h2 className="text-sm font-bold text-gray-900 dark:text-white">Create UBB Stream</h2>
          </div>
          <button onClick={onClose} className="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700">
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="space-y-4 px-6 py-5">
          <div>
            <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">Stream Name</label>
            <input value={streamName} onChange={e => setStreamName(e.target.value)}
              placeholder="e.g. API Requests"
              className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-violet-500" />
          </div>
          <div>
            <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">Resolver ID</label>
            <input value={resolverID} onChange={e => setResolverID(e.target.value)}
              placeholder="e.g. resolver_abc123"
              className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-violet-500" />
            <p className="mt-1 text-[10px] text-gray-400">Unique identifier for the metered resource or endpoint</p>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">Included Units / mo</label>
              <input type="number" min={0} value={includedUnits} onChange={e => setIncludedUnits(Number(e.target.value))}
                className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-violet-500" />
            </div>
            <div>
              <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">Overage (¢ / unit)</label>
              <input type="number" min={1} value={overageCents} onChange={e => setOverageCents(Number(e.target.value))}
                className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-violet-500" />
              <p className="mt-1 text-[10px] text-gray-400">${(overageCents / 100).toFixed(2)} per unit over limit</p>
            </div>
          </div>
          <div className="rounded-lg bg-violet-50 dark:bg-violet-900/20 p-3 text-xs text-violet-700 dark:text-violet-300">
            First {includedUnits.toLocaleString()} units free · ${(overageCents / 100).toFixed(2)} per unit after · billed via Stripe at month end
          </div>
        </div>
        <div className="flex justify-end gap-2 border-t border-gray-100 dark:border-gray-700 px-6 py-4">
          <button onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700">Cancel</button>
          <button onClick={() => mut.mutate()} disabled={!streamName.trim() || !resolverID.trim() || mut.isPending}
            className="flex items-center gap-2 rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700 disabled:opacity-50">
            {mut.isPending ? <LoadingSpinner size="sm" /> : <Plus className="h-4 w-4" />}
            Create Stream
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Stream Card ───────────────────────────────────────────────────────────────
function StreamCard({ stream, onDelete, onRefresh }: { stream: UBBStream; onDelete: () => void; onRefresh: () => void }) {
  const qc = useQueryClient()
  const [copied, setCopied] = useState(false)
  const [qty, setQty] = useState(1)
  const [action, setAction] = useState<'increment' | 'set'>('increment')
  const [confirmDelete, setConfirmDelete] = useState(false)

  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ['ubb-usage', stream.id],
    queryFn: () => ubbService.getUsageSummary(stream.id),
    staleTime: 0,
    refetchOnWindowFocus: true,
  })

  const postMut = useMutation({
    mutationFn: () => ubbService.postUsage(stream.id, { quantity: qty, action }),
    onSuccess: (res) => {
      if (res.idempotent_skip) {
        toast('Already recorded (duplicate event)', { icon: '⚠️' })
      } else {
        toast.success(`Recorded ${res.quantity ?? qty} units via ${res.billed_via ?? 'local'}`)
      }
      qc.invalidateQueries({ queryKey: ['ubb-usage', stream.id] })
      qc.refetchQueries({ queryKey: ['ubb-usage', stream.id] })
    },
    onError: () => toast.error('Failed to post usage'),
  })

  const deleteMut = useMutation({
    mutationFn: () => ubbService.deleteStream(stream.id),
    onSuccess: () => { toast.success('Stream deleted'); onDelete() },
    onError: () => toast.error('Failed to delete stream'),
  })

  const refreshSubMut = useMutation({
    mutationFn: () => ubbService.refreshStreamSubItem(stream.id),
    onSuccess: () => { toast.success('Sub item refreshed'); onRefresh() },
    onError: () => toast.error('Failed to refresh sub item'),
  })

  function copyKey() {
    navigator.clipboard.writeText(stream.api_key)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const usagePct = summary ? Math.min((summary.total_usage / (summary.included_units || 1)) * 100, 100) : 0
  const isOver = summary ? summary.overage_units > 0 : false

  return (
    <div className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
      {/* Header */}
      <div className="flex items-center gap-3 border-b border-gray-100 dark:border-gray-700/60 px-4 py-3">
        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-violet-100 dark:bg-violet-900/40">
          <Activity className="h-4 w-4 text-violet-600 dark:text-violet-400" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-bold text-gray-900 dark:text-white truncate">{stream.stream_name}</p>
          <p className="text-[10px] text-gray-400">Resolver: <span className="font-mono">{stream.resolver_id}</span></p>
        </div>
        <span className={`rounded-full px-2 py-0.5 text-[10px] font-bold ${stream.status === 'active' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400' : 'bg-gray-100 text-gray-500'}`}>
          {stream.status}
        </span>
        {/* Refresh sub item — shown when no Stripe sub item is linked (e.g. after plan change) */}
        {!stream.stripe_sub_item_id && (
          <button
            onClick={() => refreshSubMut.mutate()}
            disabled={refreshSubMut.isPending}
            title="Re-link Stripe metered sub item (needed after plan change)"
            className="rounded-lg p-1.5 text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/30 transition-colors">
            <RefreshCw className="h-3.5 w-3.5" />
          </button>
        )}
        <button onClick={() => setConfirmDelete(true)}
          className="rounded-lg p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/30 transition-colors">
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>

      {confirmDelete && (
        <ConfirmModal
          title={`Delete "${stream.stream_name}"?`}
          message="This will permanently delete the stream and all its usage data. This action cannot be undone."
          confirmLabel="Delete Stream"
          onConfirm={() => { setConfirmDelete(false); deleteMut.mutate() }}
          onCancel={() => setConfirmDelete(false)}
        />
      )}

      <div className="p-4 space-y-3">
        {/* Warning: no Stripe sub item — local billing only */}
        {!stream.stripe_sub_item_id && (
          <div className="flex items-center gap-2 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 px-3 py-2">
            <span className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full bg-amber-500 text-[9px] font-black text-white">!</span>
            <p className="text-[11px] text-amber-700 dark:text-amber-300">
              No Stripe sub item linked — usage is recorded locally only. Click <RefreshCw className="inline h-3 w-3" /> to re-link after a plan change.
            </p>
          </div>
        )}
        {/* Pricing info */}
        <div className="grid grid-cols-3 gap-2">
          <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 px-2 py-2.5 text-center">
            <p className="text-sm font-black text-gray-800 dark:text-white leading-tight">{stream.included_units.toLocaleString()}</p>
            <p className="text-[10px] text-gray-400 mt-0.5">Included / mo</p>
          </div>
          <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 px-2 py-2.5 text-center">
            <p className="text-sm font-black text-gray-800 dark:text-white leading-tight">${(stream.overage_price_cents / 100).toFixed(2)}</p>
            <p className="text-[10px] text-gray-400 mt-0.5">Per overage unit</p>
          </div>
          <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 px-2 py-2.5 text-center">
            <p className="text-sm font-black text-gray-600 dark:text-gray-300 leading-tight truncate">{stream.plan_name || 'free'}</p>
            <p className="text-[10px] text-gray-400 mt-0.5">Plan</p>
          </div>
        </div>

        {/* API Key */}
        <div className="flex items-center gap-2 rounded-lg border border-dashed border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-700/30 px-3 py-2">
          <span className="text-[10px] font-semibold text-gray-500 dark:text-gray-400 flex-shrink-0">API Key</span>
          <code className="flex-1 truncate text-[10px] font-mono text-gray-700 dark:text-gray-300">{stream.api_key}</code>
          <button onClick={copyKey} className="flex-shrink-0 rounded p-1 text-gray-400 hover:text-violet-600 transition-colors">
            {copied ? <CheckCircle className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
          </button>
        </div>

        {/* Post Usage */}
        <div className="rounded-lg border border-gray-200 dark:border-gray-700 p-3 space-y-2">
          <p className="text-[11px] font-bold text-gray-700 dark:text-gray-300">Post Usage Event</p>
          <div className="flex gap-2">
            <input type="number" min={1} value={qty} onChange={e => setQty(Number(e.target.value))}
              className="w-24 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-2 py-1.5 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-violet-500"
              placeholder="Units" />
            <select value={action} onChange={e => setAction(e.target.value as 'increment' | 'set')}
              className="rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-2 py-1.5 text-xs text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-violet-500">
              <option value="increment">Increment</option>
              <option value="set">Set</option>
            </select>
            <button onClick={() => postMut.mutate()} disabled={postMut.isPending || qty < 1}
              className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-violet-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-violet-700 disabled:opacity-50 transition-colors">
              {postMut.isPending ? <LoadingSpinner size="sm" /> : <Play className="h-3.5 w-3.5" />}
              Execute
            </button>
          </div>
        </div>

        {/* Usage Summary — always visible */}
        <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-gray-700 dark:text-gray-300 flex items-center gap-1">
              <DollarSign className="h-3 w-3 text-violet-500" /> Usage this period
            </span>
            {summaryLoading ? (
              <LoadingSpinner size="sm" />
            ) : (
              <span className={`text-xs font-black ${isOver ? 'text-red-500' : 'text-emerald-600'}`}>
                {(summary?.total_usage ?? 0).toLocaleString()} / {(summary?.included_units ?? stream.included_units).toLocaleString()}
              </span>
            )}
          </div>
          <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-gray-600 overflow-hidden">
            <div className="h-full rounded-full transition-all duration-500"
              style={{ width: `${usagePct}%`, background: isOver ? '#ef4444' : '#8b5cf6' }} />
          </div>
          {isOver && summary && (
            <div className="flex items-center justify-between rounded-lg bg-red-50 dark:bg-red-900/20 px-3 py-2">
              <span className="text-xs text-red-600 dark:text-red-400">Overage: {summary.overage_units.toLocaleString()} units</span>
              <span className="text-xs font-black text-red-600 dark:text-red-400">${summary.overage_cost_usd}</span>
            </div>
          )}
          {summary && (
            <div className="flex gap-2 text-[10px] text-gray-400">
              <span>Stripe: {summary.stripe_total.toLocaleString()}</span>
              <span>·</span>
              <span>Local: {summary.local_total.toLocaleString()}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ── Invoice Preview Panel ─────────────────────────────────────────────────────
function InvoicePreviewPanel() {
  const qc = useQueryClient()
  const [dryRunTriggered, setDryRunTriggered] = useState(false)

  const { data: previewData, isLoading: previewLoading } = useQuery({
    queryKey: ['ubb-invoice-preview'],
    queryFn: () => ubbService.previewInvoice(),
    staleTime: 0,
  })

  const { data: dryRun, isLoading: dryRunLoading, refetch: refetchDryRun } = useQuery({
    queryKey: ['ubb-invoice-dryrun'],
    queryFn: () => ubbService.dryRunInvoice(),
    staleTime: 0,
    enabled: dryRunTriggered,
  })

  const payMut = useMutation({
    mutationFn: () => ubbService.payInvoice(),
    onSuccess: (res) => {
      if (res.paid) {
        toast.success(`Payment successful — $${res.total_usd.toFixed(2)} charged`)
      } else if (res.invoice_url) {
        toast.error('Auto-charge failed — opening invoice to pay manually')
        window.open(res.invoice_url, '_blank')
      } else {
        toast.error(res.message || 'Payment could not be processed')
      }
      qc.invalidateQueries({ queryKey: ['ubb-invoice-preview'] })
      qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] })
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { error?: string; message?: string } } })
        ?.response?.data?.error
        || (err as { response?: { data?: { message?: string } } })?.response?.data?.message
        || 'Payment request failed'
      toast.error(msg)
    },
  })

  function refresh() {
    qc.invalidateQueries({ queryKey: ['ubb-invoice-preview'] })
    if (dryRunTriggered) {
      qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] })
      refetchDryRun()
    }
  }

  function runDryRun() {
    if (dryRunTriggered) {
      qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] })
      refetchDryRun()
    } else {
      setDryRunTriggered(true)
    }
  }

  const preview = previewData?.preview
  // Stripe's amount_due is the single source of truth when a subscription exists.
  const totalPreview = preview ? preview.amount_due : 0
  const payAmount = dryRun ? dryRun.overage_usd : 0

  return (
    <div className="space-y-3">
      {/* Stripe Upcoming Invoice */}
      <div className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
        <div className="flex items-center justify-between border-b border-gray-100 dark:border-gray-700/60 px-4 py-3">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-emerald-100 dark:bg-emerald-900/40">
              <FileText className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            </div>
            <p className="text-sm font-bold text-gray-900 dark:text-white">Upcoming Invoice</p>
          </div>
          <button onClick={refresh} className="rounded-lg p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors">
            <RefreshCw className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="p-4 space-y-3">
          {previewLoading ? (
            <div className="flex justify-center py-4"><LoadingSpinner /></div>
          ) : !preview ? (
            <div className="text-center py-4">
              <p className="text-xs text-gray-500 dark:text-gray-400">{previewData?.message ?? 'No active Stripe subscription'}</p>
              <p className="mt-1 text-[11px] text-gray-400">Run a Dry Run below to see your estimated charges.</p>
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between">
                <span className="text-[10px] text-gray-400">
                  {preview.period_start ? new Date(preview.period_start * 1000).toLocaleDateString() : '—'}
                  {' → '}
                  {preview.period_end ? new Date(preview.period_end * 1000).toLocaleDateString() : '—'}
                </span>
                <span className="text-base font-black text-gray-900 dark:text-white">
                  ${totalPreview.toFixed(2)} <span className="text-[10px] font-normal text-gray-400 uppercase">{preview.currency}</span>
                </span>
              </div>
              <div className="divide-y divide-gray-50 dark:divide-gray-700/40">
                {(preview.lines ?? []).map((l, i) => (
                  <div key={i} className="flex items-start justify-between gap-2 py-1.5">
                    <div className="flex-1 min-w-0">
                      <span className="text-[11px] text-gray-600 dark:text-gray-300 block">{l.description}</span>
                      {l.quantity > 0 && !l.unit_amount_zero && (
                        <span className="text-[10px] text-gray-400">{l.quantity.toLocaleString()} units metered by Stripe</span>
                      )}
                      {l.unit_amount_zero && (
                        <span className="text-[10px] text-amber-500">Legacy sub item — click ↻ on stream card to fix pricing</span>
                      )}
                    </div>
                    <span className="text-[11px] font-semibold text-gray-800 dark:text-white flex-shrink-0">${l.amount_usd.toFixed(2)}</span>
                  </div>
                ))}
              </div>
              <p className="text-[10px] text-gray-400 dark:text-gray-500 pt-1">
                Metered lines reflect all usage Stripe has recorded for this billing period.
              </p>
            </>
          )}
        </div>
      </div>

      {/* Dry Run trigger button */}
      <button
        onClick={runDryRun}
        disabled={dryRunLoading}
        className="flex w-full items-center justify-center gap-2 rounded-xl border border-violet-300 dark:border-violet-700 bg-violet-50 dark:bg-violet-900/20 px-4 py-2.5 text-sm font-semibold text-violet-700 dark:text-violet-300 hover:bg-violet-100 dark:hover:bg-violet-900/40 disabled:opacity-50 transition-all">
        {dryRunLoading ? <LoadingSpinner size="sm" /> : <FlaskConical className="h-4 w-4" />}
        {dryRunLoading ? 'Calculating...' : dryRun ? 'Re-run Dry Run' : 'Run Dry Run'}
      </button>

      {/* Dry Run result — persists after run */}
      {dryRun && !dryRunLoading && (
        <div className="rounded-xl border border-violet-200 dark:border-violet-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 bg-violet-50 dark:bg-violet-900/20 border-b border-violet-100 dark:border-violet-800/60">
            <div className="flex items-center gap-2">
              <FlaskConical className="h-4 w-4 text-violet-600 dark:text-violet-400" />
              <p className="text-sm font-bold text-violet-800 dark:text-violet-200">Dry Run — {dryRun.period}</p>
            </div>
            <span className="text-base font-black text-violet-700 dark:text-violet-300">${dryRun.total_usd.toFixed(2)}</span>
          </div>
          <div className="p-4 space-y-3">
            {/* Line items */}
            <div className="divide-y divide-gray-50 dark:divide-gray-700/40">
              {dryRun.lines.map((l, i) => (
                <div key={i} className="py-2 space-y-0.5">
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-[11px] text-gray-700 dark:text-gray-300 flex-1">{l.description}</span>
                    <span className={`text-[11px] font-bold flex-shrink-0 ${l.is_overage ? 'text-red-600 dark:text-red-400' : 'text-gray-800 dark:text-white'}`}>
                      ${l.amount_usd.toFixed(2)}
                    </span>
                  </div>
                  {l.units > 0 && (
                    <div className="flex flex-wrap items-center gap-1.5 text-[10px] text-gray-400">
                      <span>{l.units.toLocaleString()} used</span>
                      {l.included_units > 0 && <><span>·</span><span>{l.included_units.toLocaleString()} included</span></>}
                      {l.overage_units > 0 && (
                        <span className="font-semibold text-red-400">{l.overage_units.toLocaleString()} overage</span>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>

            {/* Subtotals */}
            <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 p-3 space-y-1.5 text-xs">
              <div className="flex justify-between text-gray-500 dark:text-gray-400">
                <span>Flat fee</span><span>${dryRun.flat_fee_usd.toFixed(2)}</span>
              </div>
              <div className={`flex justify-between ${dryRun.overage_usd > 0 ? 'text-red-600 dark:text-red-400 font-semibold' : 'text-gray-500 dark:text-gray-400'}`}>
                <span>Overage</span><span>${dryRun.overage_usd.toFixed(2)}</span>
              </div>
              <div className="flex justify-between font-black text-gray-900 dark:text-white border-t border-gray-200 dark:border-gray-600 pt-1.5 text-sm">
                <span>Total</span><span>${dryRun.total_usd.toFixed(2)}</span>
              </div>
            </div>

            <PayNowButton
              payAmount={payAmount}
              isPaying={payMut.isPending}
              onPay={() => payMut.mutate()}
              lastResult={payMut.data}
            />
          </div>
        </div>
      )}

      {/* Pay button shown only when dry run has overage and no Stripe sub item */}
      {!dryRun && payAmount > 0 && (
        <PayNowButton
          payAmount={payAmount}
          isPaying={payMut.isPending}
          onPay={() => payMut.mutate()}
          lastResult={payMut.data}
        />
      )}
    </div>
  )
}


// ── Pay Now Button ────────────────────────────────────────────────────────────
function PayNowButton({
  payAmount, isPaying, onPay, lastResult,
}: {
  payAmount: number
  isPaying: boolean
  onPay: () => void
  lastResult?: import('../services/ubb.service').PayInvoiceResult
}) {
  const hasOverage = payAmount > 0

  if (lastResult?.paid) {
    return (
      <div className="rounded-lg bg-emerald-50 dark:bg-emerald-900/20 border border-emerald-200 dark:border-emerald-800 p-3 space-y-1.5">
        <div className="flex items-center gap-2">
          <CheckCircle className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
          <p className="text-xs font-bold text-emerald-700 dark:text-emerald-300">Payment successful — ${lastResult.total_usd.toFixed(2)} charged</p>
        </div>
        {lastResult.invoice_url && (
          <a href={lastResult.invoice_url} target="_blank" rel="noreferrer"
            className="flex items-center gap-1 text-[10px] text-emerald-600 dark:text-emerald-400 hover:underline">
            <ExternalLink className="h-3 w-3" /> View invoice
          </a>
        )}
      </div>
    )
  }

  if (lastResult && !lastResult.paid && lastResult.message) {
    // Detect "already paid" message and show as success
    const isAlreadyPaid = lastResult.message.toLowerCase().includes('already paid')
    if (isAlreadyPaid) {
      return (
        <div className="rounded-lg bg-emerald-50 dark:bg-emerald-900/20 border border-emerald-200 dark:border-emerald-800 p-3 space-y-1.5">
          <div className="flex items-center gap-2">
            <CheckCircle className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
            <p className="text-xs font-bold text-emerald-700 dark:text-emerald-300">Invoice already paid — no further action needed</p>
          </div>
          {lastResult.invoice_url && (
            <a href={lastResult.invoice_url} target="_blank" rel="noreferrer"
              className="flex items-center gap-1 text-[10px] text-emerald-600 dark:text-emerald-400 hover:underline">
              <ExternalLink className="h-3 w-3" /> View invoice
            </a>
          )}
        </div>
      )
    }

    return (
      <div className="rounded-lg border border-amber-200 dark:border-amber-800 overflow-hidden">
        <div className="flex items-start gap-2.5 bg-amber-50 dark:bg-amber-900/20 px-3 py-2.5">
          <span className="mt-0.5 flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full bg-amber-500 text-[9px] font-black text-white">!</span>
          <div className="flex-1 min-w-0">
            <p className="text-xs font-bold text-amber-800 dark:text-amber-300">Payment could not be auto-charged</p>
            <p className="mt-0.5 text-[11px] text-amber-700 dark:text-amber-400 leading-relaxed">{lastResult.message}</p>
          </div>
        </div>
        {lastResult.invoice_url && (
          <div className="bg-amber-50/50 dark:bg-amber-900/10 px-3 py-2 border-t border-amber-100 dark:border-amber-800/50">
            <a href={lastResult.invoice_url} target="_blank" rel="noreferrer"
              className="flex items-center justify-center gap-1.5 rounded-lg bg-amber-600 hover:bg-amber-700 px-3 py-2 text-xs font-bold text-white transition-colors w-full">
              <ExternalLink className="h-3.5 w-3.5" /> Pay via Stripe Invoice
            </a>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <button
        onClick={onPay}
        disabled={isPaying || !hasOverage}
        className="flex w-full items-center justify-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-40 disabled:cursor-not-allowed transition-all active:scale-95 shadow-sm">
        {isPaying ? <LoadingSpinner size="sm" /> : <CreditCard className="h-4 w-4" />}
        {isPaying ? 'Processing...' : hasOverage ? `Pay Overage — $${payAmount.toFixed(2)}` : 'No Overage to Pay'}
      </button>
      {!hasOverage && (
        <p className="text-center text-[10px] text-gray-400">All usage within included limits</p>
      )}
    </div>
  )
}


// ── Main Page ─────────────────────────────────────────────────────────────────
export default function UBBPage() {
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['ubb-streams'],
    queryFn: () => ubbService.listStreams(),
  })

  const streams = data?.streams ?? []

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">Usage-Based Billing</h1>
          <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
            Flat fee + overages · metered via Stripe · billed at month end
          </p>
        </div>
        <button onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700 active:scale-95 transition-all shadow-sm">
          <Plus className="h-4 w-4" /> New Stream
        </button>
      </div>

      {/* Pricing model info banner */}
      <div className="rounded-xl border border-violet-200 dark:border-violet-800 bg-violet-50 dark:bg-violet-900/20 p-4">
        <div className="flex flex-wrap gap-6 text-sm">
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Model</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">Flat Fee + Overages</p>
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Included</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">1,000 units / stream / mo</p>
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Overage</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">$0.04 per unit (default)</p>
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Billing</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">Stripe · monthly invoice</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Streams */}
        <div className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-bold text-gray-700 dark:text-gray-300">
              Active Streams <span className="ml-1.5 rounded-full bg-gray-100 dark:bg-gray-700 px-2 py-0.5 text-xs font-semibold text-gray-500 dark:text-gray-400">{streams.length}</span>
            </h2>
          </div>

          {isLoading ? (
            <div className="flex justify-center py-12"><LoadingSpinner size="lg" /></div>
          ) : streams.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 dark:border-gray-600 py-16 text-center">
              <Activity className="mb-3 h-10 w-10 text-gray-300 dark:text-gray-600" />
              <p className="text-sm font-semibold text-gray-500 dark:text-gray-400">No streams yet</p>
              <p className="mt-1 text-xs text-gray-400">Create a stream to start metering API usage</p>
              <button onClick={() => setShowCreate(true)}
                className="mt-4 flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700">
                <Plus className="h-4 w-4" /> Create First Stream
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {streams.map(s => (
                <StreamCard
                  key={s.id}
                  stream={s}
                  onDelete={() => qc.invalidateQueries({ queryKey: ['ubb-streams'] })}
                  onRefresh={() => qc.invalidateQueries({ queryKey: ['ubb-streams'] })}
                />
              ))}
            </div>
          )}
        </div>

        {/* Invoice preview sidebar */}
        <div className="space-y-4">
          <h2 className="text-sm font-bold text-gray-700 dark:text-gray-300">Invoice Preview</h2>
          <InvoicePreviewPanel />

          {/* How it works */}
          <div className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 space-y-3">
            <p className="text-xs font-bold text-gray-700 dark:text-gray-300">How it works</p>
            {[
              ['1', 'Create a stream with a resolver ID and API key'],
              ['2', 'Post usage events via the Execute button or API'],
              ['3', 'Stripe meters usage and bills overages at month end'],
              ['4', 'Invoice combines flat fee + any overage charges'],
            ].map(([n, t]) => (
              <div key={n} className="flex gap-2.5">
                <span className="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-violet-100 dark:bg-violet-900/40 text-[10px] font-black text-violet-600 dark:text-violet-400">{n}</span>
                <p className="text-xs text-gray-600 dark:text-gray-400">{t}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {showCreate && (
        <CreateStreamModal
          onClose={() => setShowCreate(false)}
          onCreated={() => qc.invalidateQueries({ queryKey: ['ubb-streams'] })}
        />
      )}
    </div>
  )
}
