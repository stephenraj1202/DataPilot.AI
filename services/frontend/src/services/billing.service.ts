import api from './api'

export interface Plan {
  id: string
  name: string
  price_cents: number
  max_cloud_accounts: number | null
  max_database_connections: number | null
  rate_limit_per_minute: number
  features: string[]
}

export interface Subscription {
  id: string
  plan: Plan
  status: string
  current_period_start: string
  current_period_end: string
  cancel_at_period_end: boolean
}

export interface Invoice {
  id: string
  stripe_invoice_id: string
  amount_cents: number
  currency: string
  status: string
  invoice_pdf_url: string | null
  created_at: string
}

export const billingService = {
  async getSubscription(): Promise<{ subscription: Subscription }> {
    const { data } = await api.get('/api/billing/subscription')
    return data
  },

  async getPlans(): Promise<{ plans: Plan[] }> {
    const { data } = await api.get('/api/billing/plans')
    return data
  },

  async subscribe(planName: string, paymentMethodId?: string): Promise<{ subscription_id: string; status: string }> {
    const { data } = await api.post('/api/billing/subscribe', {
      plan: planName,
      payment_method_id: paymentMethodId,
    })
    return data
  },

  async createCheckoutSession(planName: string): Promise<{ checkout_url: string; local?: boolean }> {
    const { data } = await api.post('/api/billing/checkout', { plan_name: planName })
    return data
  },

  async confirmCheckoutSession(sessionId: string): Promise<{ plan: string; status: string }> {
    const { data } = await api.post('/api/billing/checkout/confirm', { session_id: sessionId })
    return data
  },

  async updateSubscription(newPlan: string): Promise<{ proration_amount: number; effective_date: string }> {
    const { data } = await api.put('/api/billing/subscription', { new_plan: newPlan })
    return data
  },

  async getInvoices(): Promise<{ invoices: Invoice[] }> {
    const { data } = await api.get('/api/billing/invoices')
    return data
  },

  async downloadInvoice(invoiceId: string): Promise<string> {
    const { data } = await api.get(`/api/billing/invoices/${invoiceId}/pdf`)
    return data.url
  },

  async getNextBillSummary(): Promise<{
    plan_name: string
    billing_period: string
    flat_fee_usd: number
    active_overage_usd: number
    deleted_revenue_usd: number
    total_usd: number
  }> {
    const { data } = await api.get('/api/billing/ubb/next-bill')
    return data
  },
}
