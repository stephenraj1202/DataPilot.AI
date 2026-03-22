import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus, Trash2, Zap, Copy, CheckCircle, Activity, DollarSign,
  FileText, X, Play, RefreshCw, CreditCard, FlaskConical,
  ExternalLink, TrendingUp,
} from 'lucide-react'
import { ubbService, type UBBStream, type DryRunInvoice } from '../services/ubb.service'
import { billingService } from '../services/billing.service'
import LoadingSpinner from '../components/common/LoadingSpinner'
import ConfirmModal from '../components/common/ConfirmModal'
import toast from 'react-hot-toast'
import { usePaymentMode } from '../hooks/usePaymentMode'
import { formatPaise } from '../utils/formatters'

// paise → ₹ with 4 decimal places for micro-amounts
function formatPaiseExact(paise: number): string {
  const inr = paise / 100
  if (inr < 1) return `₹${inr.toFixed(4)}`
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', minimumFractionDigits: 2, maximumFractionDigits: 4 }).format(inr)
}

// ── Create Stream Modal ───────────────────────────────────────────────────────
function CreateStreamModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [streamName, setStreamName] = useState('')
  const [resolverID, setResolverID] = useState('')
  const [unitPriceCents, setUnitPriceCents] = useState(4)

  const mut = useMutation({
    mutationFn: () => ubbService.createStream({
      stream_name: streamName.trim(),
      resolver_id: resolverID.trim(),
      overage_price_cents: unitPriceCents,
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
            <h2 className="text-sm font-bold text-gray-900 dark:text-white">Create Metered Stream</h2>
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
          <div>
            <label className="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">Price per unit (paise)</label>
            <input type="number" min={1} value={unitPriceCents} onChange={e => setUnitPriceCents(Number(e.target.value))}
              className="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-violet-500" />
            <p className="mt-1 text-[10px] text-gray-400">₹{(unitPriceCents / 100).toFixed(4)} charged per unit posted · billed via payment gateway</p>
          </div>
          <div className="rounded-lg bg-violet-50 dark:bg-violet-900/20 p-3 text-xs text-violet-700 dark:text-violet-300">
            Every unit posted is billed at ₹{(unitPriceCents / 100).toFixed(4)} · invoices automatically at period end
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

  const totalUnits = summary?.total_usage ?? 0
  const unitPriceCents = stream.overage_price_cents
  const billedCents = totalUnits * unitPriceCents

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
        {!stream.stripe_sub_item_id && (
          <button onClick={() => refreshSubMut.mutate()} disabled={refreshSubMut.isPending}
            title="Re-link payment sub item"
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
          message="This will permanently delete the stream and all its usage data."
          confirmLabel="Delete Stream"
          onConfirm={() => { setConfirmDelete(false); deleteMut.mutate() }}
          onCancel={() => setConfirmDelete(false)}
        />
      )}

      <div className="p-4 space-y-3">
        {!stream.stripe_sub_item_id && (
          <div className="flex items-center gap-2 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 px-3 py-2">
            <span className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full bg-amber-500 text-[9px] font-black text-white">!</span>
            <p className="text-[11px] text-amber-700 dark:text-amber-300">
              No payment sub item — usage recorded locally. Click <RefreshCw className="inline h-3 w-3" /> to re-link.
            </p>
          </div>
        )}

        {/* Pricing info — simple: price per unit + billed so far */}
        <div className="grid grid-cols-2 gap-2">
          <div className="rounded-lg bg-violet-50 dark:bg-violet-900/20 px-3 py-2.5 text-center">
            <p className="text-sm font-black text-violet-700 dark:text-violet-300">₹{(unitPriceCents / 100).toFixed(4)}</p>
            <p className="text-[10px] text-gray-400 mt-0.5">Per unit</p>
          </div>
          <div className="rounded-lg bg-emerald-50 dark:bg-emerald-900/20 px-3 py-2.5 text-center">
            <p className="text-sm font-black text-emerald-700 dark:text-emerald-300">
              {summaryLoading ? '…' : formatPaiseExact(billedCents)}
            </p>
            <p className="text-[10px] text-gray-400 mt-0.5">Billed this period</p>
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
              Post
            </button>
          </div>
          {qty > 0 && (
            <p className="text-[10px] text-gray-400">
              This will add <span className="font-semibold text-violet-600 dark:text-violet-400">₹{(qty * unitPriceCents / 100).toFixed(4)}</span> to next invoice
            </p>
          )}
        </div>

        {/* Usage meter */}
        <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-gray-700 dark:text-gray-300 flex items-center gap-1">
              <TrendingUp className="h-3 w-3 text-violet-500" /> Units this period
            </span>
            {summaryLoading ? <LoadingSpinner size="sm" /> : (
              <span className="text-xs font-black text-violet-600 dark:text-violet-400">
                {totalUnits.toLocaleString()} units
              </span>
            )}
          </div>
          {/* Simple progress bar — fills as units accumulate */}
          <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-gray-600 overflow-hidden">
            <div className="h-full rounded-full bg-gradient-to-r from-violet-500 to-purple-500 transition-all duration-500"
              style={{ width: `${Math.min((totalUnits / Math.max(totalUnits * 1.5, 1000)) * 100, 100)}%` }} />
          </div>
          <div className="flex items-center justify-between text-[10px] text-gray-400">
            <span>Next invoice: <span className="font-semibold text-emerald-600 dark:text-emerald-400">{formatPaiseExact(billedCents)}</span></span>
            {summary && (
              <span>{summary.stripe_total.toLocaleString()} gateway · Local: {summary.local_total.toLocaleString()}</span>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ── Invoice Preview Panel ─────────────────────────────────────────────────────
function InvoicePreviewPanel() {
  const qc = useQueryClient()
  const { mode: payMode } = usePaymentMode()
  const [dryRunTriggered, setDryRunTriggered] = useState(false)
  const [rzpLoading, setRzpLoading] = useState(false)

  const { data: previewData, isLoading: previewLoading } = useQuery({
    queryKey: ['ubb-invoice-preview'],
    queryFn: () => ubbService.previewInvoice(),
    staleTime: 0,
    enabled: payMode !== 'razorpay',
  })

  const { data: dryRun, isLoading: dryRunLoading, refetch: refetchDryRun } = useQuery({
    queryKey: ['ubb-invoice-dryrun'],
    queryFn: () => ubbService.dryRunInvoice(),
    staleTime: 0,
    enabled: dryRunTriggered,
  })

  const payMut = useMutation({
    mutationFn: () => ubbService.payInvoice(),
    onSuccess: async (res) => {
      if (res.razorpay && res.order_id) {
        // Open Razorpay checkout for UBB overage
        const rzp = new (window as any).Razorpay({
          key: res.key_id,
          order_id: res.order_id,
          amount: res.amount,
          currency: res.currency ?? 'INR',
          name: 'DataPilot.AI',
          description: res.description ?? 'UBB overage charges',
          handler: async (response: { razorpay_order_id: string; razorpay_payment_id: string; razorpay_signature: string }) => {
            setRzpLoading(true)
            try {
              await billingService.verifyUBBOveragePayment({
                razorpay_order_id: response.razorpay_order_id,
                razorpay_payment_id: response.razorpay_payment_id,
                razorpay_signature: response.razorpay_signature,
                amount_paise: res.amount ?? 0,
              })
              toast.success(`UBB overage paid — ₹${(res.amount ?? 0) / 100} charged`)
              qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] })
              qc.invalidateQueries({ queryKey: ['razorpay-payments'] })
            } catch {
              toast.error('Payment verification failed')
            } finally {
              setRzpLoading(false)
            }
          },
          modal: { ondismiss: () => setRzpLoading(false) },
        })
        rzp.open()
      } else if (res.paid) {
        toast.success(`Payment successful — ₹${res.total_usd.toFixed(2)} charged`)
      } else if (res.invoice_url) {
        toast.error('Auto-charge failed — opening invoice')
        window.open(res.invoice_url, '_blank')
      } else {
        toast.error(res.message || 'Payment could not be processed')
      }
      qc.invalidateQueries({ queryKey: ['ubb-invoice-preview'] })
      qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] })
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Payment failed'
      toast.error(msg)
    },
  })

  function refresh() {
    qc.invalidateQueries({ queryKey: ['ubb-invoice-preview'] })
    if (dryRunTriggered) { qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] }); refetchDryRun() }
  }

  const preview = previewData?.preview
  const totalPreview = preview ? preview.amount_due : 0

  return (
    <div className="space-y-3">
      {/* Stripe Upcoming Invoice */}
      <div className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
        <div className="flex items-center justify-between border-b border-gray-100 dark:border-gray-700/60 px-4 py-3">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-emerald-100 dark:bg-emerald-900/40">
              <FileText className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            </div>
            <p className="text-sm font-bold text-gray-900 dark:text-white">Next Invoice</p>
          </div>
          <button onClick={refresh} className="rounded-lg p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
            <RefreshCw className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="p-4 space-y-3">
          {previewLoading ? (
            <div className="flex justify-center py-4"><LoadingSpinner /></div>
          ) : !preview ? (
            <div className="text-center py-4">
              <p className="text-xs text-gray-500 dark:text-gray-400">{previewData?.message ?? 'No active subscription'}</p>
              <p className="mt-1 text-[11px] text-gray-400">Run a preview below to see estimated charges.</p>
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
                  ₹{totalPreview.toFixed(2)} <span className="text-[10px] font-normal text-gray-400 uppercase">{preview.currency?.toUpperCase() ?? 'INR'}</span>
                </span>
              </div>
              <div className="divide-y divide-gray-50 dark:divide-gray-700/40">
                {(preview.lines ?? []).map((l, i) => (
                  <div key={i} className="flex items-start justify-between gap-2 py-1.5">
                    <div className="flex-1 min-w-0">
                      <span className="text-[11px] text-gray-600 dark:text-gray-300 block">{l.description}</span>
                      {l.quantity > 0 && (
                        <span className="text-[10px] text-gray-400">{l.quantity.toLocaleString()} units × rate</span>
                      )}
                    </div>
                    <span className="text-[11px] font-semibold text-gray-800 dark:text-white flex-shrink-0">₹{l.amount_usd.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Dry Run */}
      <button onClick={() => { if (dryRunTriggered) { qc.invalidateQueries({ queryKey: ['ubb-invoice-dryrun'] }); refetchDryRun() } else setDryRunTriggered(true) }}
        disabled={dryRunLoading}
        className="flex w-full items-center justify-center gap-2 rounded-xl border border-violet-300 dark:border-violet-700 bg-violet-50 dark:bg-violet-900/20 px-4 py-2.5 text-sm font-semibold text-violet-700 dark:text-violet-300 hover:bg-violet-100 dark:hover:bg-violet-900/40 disabled:opacity-50 transition-all">
        {dryRunLoading ? <LoadingSpinner size="sm" /> : <FlaskConical className="h-4 w-4" />}
        {dryRunLoading ? 'Calculating…' : dryRun ? 'Re-run Preview' : 'Preview Next Bill'}
      </button>

      {dryRun && !dryRunLoading && (
        <div className="rounded-xl border border-violet-200 dark:border-violet-700 bg-white dark:bg-gray-800 shadow-sm overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 bg-violet-50 dark:bg-violet-900/20 border-b border-violet-100 dark:border-violet-800/60">
            <div className="flex items-center gap-2">
              <FlaskConical className="h-4 w-4 text-violet-600 dark:text-violet-400" />
              <p className="text-sm font-bold text-violet-800 dark:text-violet-200">Preview — {dryRun.period}</p>
            </div>
            <span className="text-base font-black text-violet-700 dark:text-violet-300">₹{dryRun.total_usd.toFixed(2)}</span>
          </div>
          <div className="p-4 space-y-3">
            <div className="divide-y divide-gray-50 dark:divide-gray-700/40">
              {dryRun.lines.map((l, i) => (
                <div key={i} className="py-2 space-y-0.5">
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-[11px] text-gray-700 dark:text-gray-300 flex-1">{l.description}</span>
                    <span className="text-[11px] font-bold flex-shrink-0 text-gray-800 dark:text-white">₹{l.amount_usd.toFixed(2)}</span>
                  </div>
                  {l.overage_units > 0 && (
                    <p className="text-[10px] text-gray-400">
                      {l.units.toLocaleString()} total · {l.included_units.toLocaleString()} free · {l.overage_units.toLocaleString()} billed
                    </p>
                  )}
                  {l.units > 0 && l.overage_units === 0 && l.included_units > 0 && (
                    <p className="text-[10px] text-emerald-500">
                      {l.units.toLocaleString()} units — within free tier ({l.included_units.toLocaleString()} included)
                    </p>
                  )}
                </div>
              ))}
            </div>
            <div className="rounded-lg bg-gray-50 dark:bg-gray-700/40 p-3 space-y-1.5 text-xs">
              <div className="flex justify-between text-gray-500 dark:text-gray-400">
                <span>Plan flat fee</span><span>₹{dryRun.flat_fee_usd.toFixed(2)}</span>
              </div>
              <div className="flex justify-between text-violet-600 dark:text-violet-400 font-semibold">
                <span>Usage charges</span><span>₹{dryRun.overage_usd.toFixed(4)}</span>
              </div>
              <div className="flex justify-between font-black text-gray-900 dark:text-white border-t border-gray-200 dark:border-gray-600 pt-1.5 text-sm">
                <span>Total</span><span>₹{dryRun.total_usd.toFixed(2)}</span>
              </div>
            </div>

            {/* Pay button */}
            {payMut.data?.paid ? (
              <div className="rounded-lg bg-emerald-50 dark:bg-emerald-900/20 border border-emerald-200 dark:border-emerald-800 p-3 flex items-center gap-2">
                <CheckCircle className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                <p className="text-xs font-bold text-emerald-700 dark:text-emerald-300">Paid — ₹{payMut.data.total_usd.toFixed(2)} charged</p>
                {payMut.data.invoice_url && (
                  <a href={payMut.data.invoice_url} target="_blank" rel="noreferrer" className="ml-auto text-[10px] text-emerald-600 hover:underline flex items-center gap-1">
                    <ExternalLink className="h-3 w-3" /> View
                  </a>
                )}
              </div>
            ) : (
              <button onClick={() => payMut.mutate()} disabled={payMut.isPending || rzpLoading || dryRun.total_usd <= dryRun.flat_fee_usd}
                className="flex w-full items-center justify-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-bold text-white hover:bg-emerald-700 disabled:opacity-40 transition-all">
                {(payMut.isPending || rzpLoading) ? <LoadingSpinner size="sm" /> : <CreditCard className="h-4 w-4" />}
                {(payMut.isPending || rzpLoading) ? 'Processing…' : `Pay ₹${dryRun.overage_usd.toFixed(2)} overage`}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Main Page ─────────────────────────────────────────────────────────────────
export default function UBBPage() {
  const { label: payLabel, mode: payMode } = usePaymentMode()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['ubb-streams'],
    queryFn: () => ubbService.listStreams(),
  })

  const streams = data?.streams ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">Usage-Based Billing <span className="ml-2 rounded-full bg-amber-100 dark:bg-amber-900/30 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:text-amber-400">Under Development</span></h1>
          <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
            Post usage events · billed per unit via {payLabel} · invoiced at period end
          </p>
        </div>
        <button onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700 active:scale-95 transition-all shadow-sm">
          <Plus className="h-4 w-4" /> New Stream
        </button>
      </div>

      {/* Model info banner */}
      <div className="rounded-xl border border-violet-200 dark:border-violet-800 bg-violet-50 dark:bg-violet-900/20 p-4">
        <div className="flex flex-wrap gap-6 text-sm">
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Model</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">Pay per unit</p>
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Billing</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">Every unit posted → next invoice</p>
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Invoice</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">{payLabel} · monthly · auto-charge</p>
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-wider text-violet-500 dark:text-violet-400">Default rate</p>
            <p className="font-semibold text-violet-900 dark:text-violet-200">₹0.0004 per unit</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-bold text-gray-700 dark:text-gray-300">
              Active Streams
              <span className="ml-1.5 rounded-full bg-gray-100 dark:bg-gray-700 px-2 py-0.5 text-xs font-semibold text-gray-500 dark:text-gray-400">{streams.length}</span>
            </h2>
          </div>

          {isLoading ? (
            <div className="flex justify-center py-12"><LoadingSpinner size="lg" /></div>
          ) : streams.length === 0 ? (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 dark:border-gray-600 py-16 text-center">
              <Activity className="mb-3 h-10 w-10 text-gray-300 dark:text-gray-600" />
              <p className="text-sm font-semibold text-gray-500 dark:text-gray-400">No streams yet</p>
              <p className="mt-1 text-xs text-gray-400">Create a stream to start metering usage</p>
              <button onClick={() => setShowCreate(true)}
                className="mt-4 flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-700">
                <Plus className="h-4 w-4" /> Create First Stream
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {streams.map(s => (
                <StreamCard key={s.id} stream={s}
                  onDelete={() => qc.invalidateQueries({ queryKey: ['ubb-streams'] })}
                  onRefresh={() => qc.invalidateQueries({ queryKey: ['ubb-streams'] })}
                />
              ))}
            </div>
          )}
        </div>

        <div className="space-y-4">
          <h2 className="text-sm font-bold text-gray-700 dark:text-gray-300">Invoice Preview</h2>
          <InvoicePreviewPanel />

          <div className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 space-y-3">
            <p className="text-xs font-bold text-gray-700 dark:text-gray-300">How it works</p>
            {[
              ['1', 'Create a stream — get a unique API key'],
              ['2', 'Post usage events (units) from your app'],
              ['3', 'Each unit is recorded and priced at your rate'],
              ['4', `${payLabel} invoices total units × rate at period end`],
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
