import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { useLocation } from 'react-router-dom'
import { billingService } from '../services/billing.service'
import { ubbService } from '../services/ubb.service'
import LoadingSpinner from '../components/common/LoadingSpinner'
import { formatCurrency, formatPaise, formatDate } from '../utils/formatters'
import { CheckCircle, Download, Zap, Shield, Building2, Sparkles, AlertTriangle, Receipt, RefreshCw } from 'lucide-react'
import toast from 'react-hot-toast'
import { usePaymentMode } from '../hooks/usePaymentMode'

const PLAN_META: Record<string, {
  label: string
  icon: React.ReactNode
  gradient: string
  features: string[]
  highlight?: boolean
}> = {
  free: {
    label: 'Starter',
    icon: <Sparkles className="h-5 w-5" />,
    gradient: 'from-gray-500 to-gray-600',
    features: ['1 Cloud Account', '2 Database Connections', '100 req/min', '30-day free trial'],
  },
  base: {
    label: 'Base',
    icon: <Zap className="h-5 w-5" />,
    gradient: 'from-blue-500 to-indigo-600',
    features: ['3 Cloud Accounts', '5 Database Connections', '500 req/min', 'Email support'],
    highlight: true,
  },
  pro: {
    label: 'Pro',
    icon: <Shield className="h-5 w-5" />,
    gradient: 'from-violet-500 to-purple-700',
    features: ['10 Cloud Accounts', 'Unlimited Databases', '2000 req/min', 'Priority support'],
  },
  enterprise: {
    label: 'Enterprise',
    icon: <Building2 className="h-5 w-5" />,
    gradient: 'from-orange-500 to-rose-600',
    features: ['Unlimited Cloud Accounts', 'Unlimited Databases', '10000 req/min', 'Dedicated support'],
  },
}

export default function BillingPage() {
  const location = useLocation()
  const trialExpired = (location.state as { trialExpired?: boolean })?.trialExpired ?? false
  const [stripeLoading, setStripeLoading] = useState<string | null>(null)
  const { label: payLabel } = usePaymentMode()

  const { data: paymentMode } = useQuery({
    queryKey: ['payment-mode'],
    queryFn: () => billingService.getPaymentMode(),
    staleTime: Infinity,
  })

  const { data: subData, isLoading: subLoading, refetch } = useQuery({
    queryKey: ['subscription'],
    queryFn: () => billingService.getSubscription(),
  })

  const { data: plansData, isLoading: plansLoading } = useQuery({
    queryKey: ['plans'],
    queryFn: () => billingService.getPlans(),
  })

  const { data: invoicesData, isLoading: invoicesLoading } = useQuery({
    queryKey: ['invoices'],
    queryFn: () => billingService.getInvoices(),
    enabled: paymentMode?.mode !== 'razorpay',
  })

  const { data: rzpPaymentsData, isLoading: rzpPaymentsLoading } = useQuery({
    queryKey: ['razorpay-payments'],
    queryFn: () => billingService.getRazorpayPayments(),
    enabled: paymentMode?.mode === 'razorpay',
  })

  const { data: upcomingInvoice, isLoading: upcomingLoading, refetch: refetchUpcoming } = useQuery({
    queryKey: ['upcoming-invoice'],
    queryFn: () => ubbService.previewInvoice(),
    staleTime: 30_000,
    // Only fetch Stripe preview when on Stripe mode — Razorpay has no Stripe invoice
    enabled: !!subData?.subscription && paymentMode?.mode !== 'razorpay',
  })

  // nextBill is a fallback for when Stripe preview is unavailable (local/free plans)
  const { data: nextBill, isLoading: nextBillLoading } = useQuery({
    queryKey: ['ubb-next-bill'],
    queryFn: () => billingService.getNextBillSummary(),
    staleTime: 30_000,
    enabled: !!subData?.subscription,
  })

  const { mutate: changePlan } = useMutation({
    mutationFn: (planName: string) => billingService.updateSubscription(planName),
    onSuccess: () => { toast.success('Plan updated'); refetch() },
    onError: () => toast.error('Failed to update plan'),
  })

  const handleSelectPlan = async (plan: { name: string; price_cents: number; stripe_price_id?: string }) => {
    if (plan.name === 'free') return
    setStripeLoading(plan.name)
    try {
      if (paymentMode?.mode === 'razorpay') {
        // Razorpay recurring subscription flow
        const order = await billingService.createRazorpayOrder(plan.name)
        const rzp = new (window as any).Razorpay({
          key: order.key_id,
          subscription_id: order.subscription_id,
          name: 'DataPilot.AI',
          description: `${plan.name} plan — monthly recurring`,
          recurring: 1,
          handler: async (response: {
            razorpay_subscription_id?: string
            razorpay_payment_id: string
            razorpay_signature: string
          }) => {
            try {
              await billingService.verifyRazorpayPayment({
                razorpay_subscription_id: response.razorpay_subscription_id,
                razorpay_payment_id: response.razorpay_payment_id,
                razorpay_signature: response.razorpay_signature,
                plan_name: plan.name,
              })
              await refetch()
              toast.success(`Upgraded to ${plan.name} plan`)
            } catch (err: unknown) {
              console.error('[razorpay] verify failed:', err)
              const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Payment verification failed'
              toast.error(msg)
            } finally {
              setStripeLoading(null)
            }
          },
          modal: { ondismiss: () => setStripeLoading(null) },
        })
        rzp.open()
      } else {
        // Stripe flow
        const result = await billingService.createCheckoutSession(plan.name)
        if ((result as any).local) {
          await refetch()
          toast.success(`Upgraded to ${plan.name} plan`)
          setStripeLoading(null)
        } else {
          window.location.href = result.checkout_url
        }
      }
    } catch {
      toast.error('Failed to start checkout. Please try again.')
      setStripeLoading(null)
    }
  }

  const isLoading = subLoading || plansLoading
  if (isLoading) {
    return <div className="flex h-64 items-center justify-center"><LoadingSpinner size="lg" /></div>
  }

  const currentPlan = subData?.subscription?.plan
  const plans = plansData?.plans ?? []
  const sub = subData?.subscription

  return (
    <div className="space-y-8">
      {/* Trial expired banner */}
      {trialExpired && (
        <div className="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20">
          <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600 dark:text-amber-400" />
          <div>
            <p className="font-semibold text-amber-800 dark:text-amber-300">Your free trial has ended</p>
            <p className="mt-0.5 text-sm text-amber-700 dark:text-amber-400">
              Choose a plan below to continue using DataPilot.AI. All your data is safe.
            </p>
          </div>
        </div>
      )}

      {/* Page header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Billing & Plans</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Manage your subscription and payment history</p>
      </div>

      {/* Current subscription status */}
      {sub && (
        <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-gray-400">Current Plan</p>
              <p className="mt-1 text-2xl font-bold capitalize text-gray-900 dark:text-white">
                {PLAN_META[currentPlan?.name ?? '']?.label ?? currentPlan?.name ?? 'Free'}
              </p>
              <p className="text-sm text-gray-500">
                {currentPlan?.price_cents ? `${formatPaise(currentPlan.price_cents)}/month` : 'Free'}
              </p>
            </div>
            <div className="flex flex-col items-end gap-2">
              <span className={`rounded-full px-3 py-1 text-sm font-semibold ${
                sub.status === 'active' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                : 'bg-yellow-100 text-yellow-700'
              }`}>
                {sub.razorpay_status ?? sub.status}
              </span>
              <p className="text-xs text-gray-400">
                Next charge {formatDate(sub.next_charge_at ?? sub.current_period_end)}
              </p>
            </div>
          </div>

          {/* Razorpay live details */}
          {sub.razorpay_sub_id && (
            <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4 border-t border-gray-100 dark:border-gray-700 pt-4">
              <div className="rounded-xl bg-indigo-50 dark:bg-indigo-900/20 px-3 py-2.5 text-center">
                <p className="text-xs font-semibold uppercase tracking-wide text-indigo-400">Next charge</p>
                <p className="mt-0.5 text-lg font-black text-indigo-700 dark:text-indigo-300">
                  {sub.next_charge_amount ? formatPaise(sub.next_charge_amount) : formatPaise(currentPlan?.price_cents ?? 0)}
                </p>
              </div>
              <div className="rounded-xl bg-gray-50 dark:bg-gray-700/40 px-3 py-2.5 text-center">
                <p className="text-xs font-semibold uppercase tracking-wide text-gray-400">Charge date</p>
                <p className="mt-0.5 text-sm font-bold text-gray-700 dark:text-gray-300">
                  {sub.next_charge_at ? formatDate(sub.next_charge_at) : formatDate(sub.current_period_end)}
                </p>
              </div>
              <div className="rounded-xl bg-gray-50 dark:bg-gray-700/40 px-3 py-2.5 text-center">
                <p className="text-xs font-semibold uppercase tracking-wide text-gray-400">Payments made</p>
                <p className="mt-0.5 text-lg font-black text-gray-700 dark:text-gray-300">{sub.paid_count ?? '—'}</p>
              </div>
              <div className="rounded-xl bg-gray-50 dark:bg-gray-700/40 px-3 py-2.5 text-center">
                <p className="text-xs font-semibold uppercase tracking-wide text-gray-400">Remaining</p>
                <p className="mt-0.5 text-lg font-black text-gray-700 dark:text-gray-300">{sub.remaining_count ?? '—'}</p>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Upcoming Invoice */}
      {sub && (
        <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-100 dark:bg-indigo-900/40">
                <Receipt className="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
              </div>
              <div>
                <p className="text-sm font-bold text-gray-900 dark:text-white">Upcoming Invoice</p>
                <p className="text-xs text-gray-400">Estimated charges at next billing date</p>
              </div>
            </div>
            <button
              onClick={() => refetchUpcoming()}
              className="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              title="Refresh"
            >
              <RefreshCw className={`h-4 w-4 ${upcomingLoading || nextBillLoading ? 'animate-spin' : ''}`} />
            </button>
          </div>

          {upcomingLoading || nextBillLoading ? (
            <div className="flex justify-center py-4"><LoadingSpinner size="md" /></div>
          ) : (() => {
            // Razorpay mode: use live next_charge_amount + next_charge_at from subscription
            if (paymentMode?.mode === 'razorpay' && sub?.razorpay_sub_id) {
              const amount = sub.next_charge_amount ?? currentPlan?.price_cents ?? 0
              const dueDate = sub.next_charge_at ?? sub.current_period_end
              return (
                <div className="space-y-3">
                  <div className="flex items-end justify-between rounded-xl bg-indigo-50 dark:bg-indigo-900/20 px-4 py-3">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-wide text-indigo-500 dark:text-indigo-400">Amount Due</p>
                      <p className="text-3xl font-black text-indigo-700 dark:text-indigo-300">{formatPaise(amount)}</p>
                    </div>
                    <div className="text-right">
                      <p className="text-xs text-gray-400">Due {formatDate(dueDate)}</p>
                      <p className="text-xs text-gray-400 uppercase">INR · Razorpay</p>
                    </div>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-2.5">
                    <p className="text-sm text-gray-700 dark:text-gray-300 capitalize">
                      {currentPlan?.name ?? 'Plan'} — monthly recurring
                    </p>
                    <p className="text-sm font-bold text-gray-900 dark:text-white">{formatPaise(amount)}</p>
                  </div>
                </div>
              )
            }

            const preview = upcomingInvoice?.preview
            const hasStripe = !!preview
            const totalUsd = hasStripe
              ? preview!.amount_due
              : (nextBill?.total_usd ?? (currentPlan?.price_cents ?? 0) / 100)

            return (
              <div className="space-y-3">
                {/* Total */}
                <div className="flex items-end justify-between rounded-xl bg-indigo-50 dark:bg-indigo-900/20 px-4 py-3">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-indigo-500 dark:text-indigo-400">Amount Due</p>
                    <p className="text-3xl font-black text-indigo-700 dark:text-indigo-300">
                      {formatCurrency(totalUsd, 'INR')}
                    </p>
                  </div>
                  <div className="text-right">
                    {hasStripe && preview!.period_end ? (
                      <p className="text-xs text-gray-400">
                        Due {new Date(preview!.period_end * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                      </p>
                    ) : (
                      <p className="text-xs text-gray-400">Due {formatDate(sub.current_period_end)}</p>
                    )}
                    <p className="text-xs text-gray-400 uppercase">{hasStripe ? preview!.currency.toUpperCase() : 'INR'}</p>
                  </div>
                </div>

                {/* Line items */}
                <div className="divide-y divide-gray-100 dark:divide-gray-700/40">
                  {hasStripe ? (
                    // Stripe live lines — show all, including $0 usage lines
                    (preview!.lines ?? []).map((line, i) => (
                      <div key={i} className="flex items-center justify-between gap-3 py-2.5">
                        <div className="flex-1 min-w-0">
                          <p className="text-sm text-gray-700 dark:text-gray-300 truncate">{line.description}</p>
                          {line.quantity > 0 && line.quantity > 1 && (
                            <p className="text-xs text-gray-400">{line.quantity.toLocaleString()} units</p>
                          )}
                        </div>
                        <p className="text-sm font-bold flex-shrink-0 text-gray-900 dark:text-white">
                          {formatCurrency(line.amount_usd, 'INR')}
                        </p>
                      </div>
                    ))
                  ) : (
                    // Local fallback
                    <>
                      {(nextBill?.flat_fee_usd ?? 0) > 0 && (
                        <div className="flex items-center justify-between gap-3 py-2.5">
                          <p className="text-sm text-gray-700 dark:text-gray-300 capitalize">
                            {nextBill?.plan_name ?? currentPlan?.name ?? 'Plan'} — monthly recurring
                          </p>
                          <p className="text-sm font-bold text-gray-900 dark:text-white">{formatCurrency(nextBill!.flat_fee_usd, 'INR')}</p>
                        </div>
                      )}
                      {(nextBill?.active_overage_usd ?? 0) > 0 && (
                        <div className="flex items-center justify-between gap-3 py-2.5">
                          <p className="text-sm text-gray-700 dark:text-gray-300">Usage overage</p>
                          <p className="text-sm font-bold text-red-600 dark:text-red-400">{formatCurrency(nextBill!.active_overage_usd, 'INR')}</p>
                        </div>
                      )}
                      {(nextBill?.deleted_revenue_usd ?? 0) > 0 && (
                        <div className="flex items-center justify-between gap-3 py-2.5">
                          <p className="text-sm text-gray-700 dark:text-gray-300">Deleted streams — accrued usage</p>
                          <p className="text-sm font-bold text-orange-600 dark:text-orange-400">{formatCurrency(nextBill!.deleted_revenue_usd, 'INR')}</p>
                        </div>
                      )}
                      {totalUsd === 0 && (
                        <p className="py-3 text-sm text-gray-400 text-center">No charges this period</p>
                      )}
                    </>
                  )}
                </div>
              </div>
            )
          })()}
        </div>
      )}

      {/* Plan cards */}
      <div>
        <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-white">Available Plans</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {plans.map(plan => {
            const meta = PLAN_META[plan.name]
            const isCurrent = plan.id === currentPlan?.id
            const isPaid = plan.price_cents > 0
            const isCardLoading = stripeLoading === plan.name
            const currentPriceCents = currentPlan?.price_cents ?? 0
            const isDowngrade = plan.price_cents < currentPriceCents
            const isFreeWhilePaid = plan.price_cents === 0 && currentPriceCents > 0

            return (
              <div
                key={plan.id}
                className={`relative flex flex-col overflow-hidden rounded-2xl border transition-all ${
                  meta?.highlight
                    ? 'border-indigo-400 shadow-lg shadow-indigo-100 dark:shadow-indigo-900/20'
                    : isCurrent
                    ? 'border-green-400'
                    : isDowngrade || isFreeWhilePaid
                    ? 'border-gray-200 opacity-60 dark:border-gray-700'
                    : 'border-gray-200 dark:border-gray-700'
                } bg-white dark:bg-gray-800`}
              >
                {/* Popular badge */}
                {meta?.highlight && (
                  <div className="absolute right-3 top-3 rounded-full bg-indigo-600 px-2.5 py-0.5 text-xs font-bold text-white">
                    Popular
                  </div>
                )}

                {/* Plan header */}
                <div className={`bg-gradient-to-br ${meta?.gradient ?? 'from-gray-500 to-gray-600'} p-5 text-white`}>
                  <div className="flex items-center gap-2">
                    {meta?.icon}
                    <span className="text-lg font-bold">{meta?.label ?? plan.name}</span>
                  </div>
                  <div className="mt-3">
                    <span className="text-3xl font-extrabold">
                      {plan.price_cents === 0 ? 'Free' : formatPaise(plan.price_cents)}
                    </span>
                    {plan.price_cents > 0 && <span className="ml-1 text-sm opacity-80">/mo</span>}
                  </div>
                  {/* Free forever badge — only on free plan */}
                  {plan.price_cents === 0 && (
                    <div className="mt-2 inline-flex items-center gap-1 rounded-full bg-white/20 px-2.5 py-1 text-xs font-semibold">
                      <Sparkles className="h-3 w-3" />
                      Free forever
                    </div>
                  )}
                </div>

                {/* Features */}
                <div className="flex flex-1 flex-col p-5">
                  <ul className="flex-1 space-y-2">
                    {(meta?.features ?? plan.features ?? []).map(f => (
                      <li key={f} className="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-400">
                        <CheckCircle className="mt-0.5 h-4 w-4 flex-shrink-0 text-green-500" />
                        {f}
                      </li>
                    ))}
                  </ul>

                  <div className="mt-5">
                    {isCurrent ? (
                      <button disabled className="w-full rounded-xl bg-green-100 py-2.5 text-sm font-semibold text-green-700 dark:bg-green-900/30 dark:text-green-400">
                        Current Plan
                      </button>
                    ) : isDowngrade || isFreeWhilePaid ? (
                      <button disabled className="w-full cursor-not-allowed rounded-xl bg-gray-100 py-2.5 text-sm font-semibold text-gray-400 dark:bg-gray-700 dark:text-gray-500">
                        Not Available
                      </button>
                    ) : (
                      <button
                        onClick={() => handleSelectPlan(plan)}
                        disabled={!!stripeLoading}
                        className={`flex w-full items-center justify-center gap-2 rounded-xl py-2.5 text-sm font-semibold transition-colors disabled:opacity-50 ${
                          meta?.highlight
                            ? 'bg-indigo-600 text-white hover:bg-indigo-700'
                            : 'border border-gray-300 text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700'
                        }`}
                      >
                        {isCardLoading && <LoadingSpinner size="sm" />}
                        {isPaid ? (isCardLoading ? 'Processing...' : 'Upgrade') : 'Select Plan'}
                      </button>
                    )}
                    {isPaid && !isCurrent && !isDowngrade && (
                      <p className="mt-2 text-center text-xs text-gray-400">
                        Billed via {payLabel} · {formatPaise(plan.price_cents)}/mo
                      </p>
                    )}
                    {(isDowngrade || isFreeWhilePaid) && !isCurrent && (
                      <p className="mt-2 text-center text-xs text-gray-400">
                        Downgrades not permitted
                      </p>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Invoice history */}
      <div>
        <h2 className="mb-3 text-lg font-semibold text-gray-900 dark:text-white">Invoice History</h2>
        {paymentMode?.mode === 'razorpay' ? (
          rzpPaymentsLoading ? (
            <LoadingSpinner size="md" />
          ) : !rzpPaymentsData?.payments?.length ? (
            <div className="rounded-2xl border border-gray-200 bg-white p-8 text-center dark:border-gray-700 dark:bg-gray-800">
              <p className="text-sm text-gray-400">No payments yet. They'll appear here once you're on a paid plan.</p>
            </div>
          ) : (
            <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-100 dark:border-gray-700">
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Billing Period</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Payment ID</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Amount</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Paid On</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Status</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Invoice</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50 dark:divide-gray-700/40">
                  {rzpPaymentsData.payments.map(p => (
                    <tr key={p.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/30">
                      <td className="px-5 py-3 text-gray-700 dark:text-gray-300">
                        {p.billing_start && p.billing_end
                          ? `${formatDate(p.billing_start)} – ${formatDate(p.billing_end)}`
                          : formatDate(p.created_at)}
                      </td>
                      <td className="px-5 py-3 font-mono text-xs text-gray-500 dark:text-gray-400">{p.payment_id || '—'}</td>
                      <td className="px-5 py-3 font-medium text-gray-900 dark:text-white">{formatPaise(p.amount)}</td>
                      <td className="px-5 py-3 text-gray-600 dark:text-gray-400">{p.paid_at ? formatDate(p.paid_at) : '—'}</td>
                      <td className="px-5 py-3">
                        <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${
                          p.status === 'paid' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                          : p.status === 'partially_paid' ? 'bg-blue-100 text-blue-700'
                          : 'bg-gray-100 text-gray-600'
                        }`}>
                          {p.status}
                        </span>
                      </td>
                      <td className="px-5 py-3">
                        {p.short_url
                          ? <a href={p.short_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1 text-indigo-600 hover:underline dark:text-indigo-400"><Download className="h-4 w-4" /> View</a>
                          : <span className="text-gray-400">—</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        ) : (
          invoicesLoading ? (
            <LoadingSpinner size="md" />
          ) : !invoicesData?.invoices?.length ? (
            <div className="rounded-2xl border border-gray-200 bg-white p-8 text-center dark:border-gray-700 dark:bg-gray-800">
              <p className="text-sm text-gray-400">No invoices yet. They'll appear here once you're on a paid plan.</p>
            </div>
          ) : (
            <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-100 dark:border-gray-700">
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Date</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Amount</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">Status</th>
                    <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-400">PDF</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50 dark:divide-gray-700/40">
                  {invoicesData.invoices.map(inv => (
                    <tr key={inv.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/30">
                      <td className="px-5 py-3 text-gray-700 dark:text-gray-300">{formatDate(inv.created_at)}</td>
                      <td className="px-5 py-3 font-medium text-gray-900 dark:text-white">{formatCurrency(inv.amount_cents / 100, inv.currency?.toUpperCase() === 'INR' ? 'INR' : 'INR')}</td>
                      <td className="px-5 py-3">
                        <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${
                          inv.status === 'paid' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                          : inv.status === 'open' ? 'bg-yellow-100 text-yellow-700'
                          : 'bg-gray-100 text-gray-600'
                        }`}>
                          {inv.status}
                        </span>
                      </td>
                      <td className="px-5 py-3">
                        {inv.invoice_pdf_url ? (
                          <a href={inv.invoice_pdf_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1 text-indigo-600 hover:underline dark:text-indigo-400">
                            <Download className="h-4 w-4" /> PDF
                          </a>
                        ) : <span className="text-gray-400">—</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        )}
      </div>
    </div>
  )
}
