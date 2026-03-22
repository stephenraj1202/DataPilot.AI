import api from './api'

export interface UBBStream {
  id: string
  stream_name: string
  resolver_id: string
  api_key: string
  stripe_sub_item_id: string
  stripe_customer_id: string
  plan_name: string
  included_units: number
  overage_price_cents: number
  status: string
  created_at: string
}

export interface UsageSummary {
  stream_id: string
  stream_name: string
  total_usage: number
  included_units: number
  overage_units: number
  overage_cost_inr: string
  overage_price_cents: number
  period_start: number
  period_end: number
  local_total: number
  stripe_total: number
}

export interface UsagePostResult {
  recorded: boolean
  idempotent_skip?: boolean
  quantity: number
  timestamp: number
  billed_via: string
  stripe_record_id?: string
  stream_name: string
  stripe_error?: string
}

export interface InvoicePreview {
  amount_due: number
  currency: string
  period_start: number
  period_end: number
  lines: Array<{ description: string; amount_usd: number; quantity: number; unit_amount_zero?: boolean }>
}

export interface DryRunInvoice {
  dry_run: boolean
  plan_name: string
  period: string
  flat_fee_usd: number
  overage_usd: number
  total_usd: number
  currency: string
  stream_count: number
  lines: Array<{
    description: string
    amount_usd: number
    units: number
    included_units: number
    overage_units: number
    is_overage: boolean
  }>
}

export interface PayInvoiceResult {
  paid: boolean
  invoice_id?: string
  invoice_url?: string
  pdf_url?: string
  total_usd: number
  status?: string
  message?: string
  // Razorpay UBB overage fields
  razorpay?: boolean
  order_id?: string
  amount?: number   // paise
  currency?: string
  key_id?: string
  description?: string
}

export const ubbService = {
  async listStreams(): Promise<{ streams: UBBStream[] }> {
    const { data } = await api.get('/api/billing/ubb/streams')
    return data
  },

  async createStream(payload: {
    stream_name: string
    resolver_id: string
    included_units?: number
    overage_price_cents?: number
  }): Promise<UBBStream> {
    const { data } = await api.post('/api/billing/ubb/streams', payload)
    return data
  },

  async deleteStream(id: string): Promise<void> {
    await api.delete(`/api/billing/ubb/streams/${id}`)
  },

  async postUsage(id: string, payload: {
    quantity: number
    timestamp?: number
    action?: 'increment' | 'set'
    idempotency_key?: string
  }): Promise<UsagePostResult> {
    const { data } = await api.post(`/api/billing/ubb/streams/${id}/usage`, payload)
    return data
  },

  async getUsageSummary(id: string): Promise<UsageSummary> {
    const { data } = await api.get(`/api/billing/ubb/streams/${id}/usage`)
    return data
  },

  async previewInvoice(): Promise<{ preview: InvoicePreview | null; message?: string }> {
    const { data } = await api.get('/api/billing/ubb/invoice/preview')
    return data
  },

  async getSubscriptionItems(): Promise<{ items: Array<{ id: string; price_id: string; price_name: string; usage_type: string }> }> {
    const { data } = await api.get('/api/billing/ubb/subscription-items')
    return data
  },

  async dryRunInvoice(): Promise<DryRunInvoice> {
    const { data } = await api.get('/api/billing/ubb/invoice/dryrun')
    return data
  },

  async payInvoice(): Promise<PayInvoiceResult> {
    const { data } = await api.post('/api/billing/ubb/invoice/pay', {})
    return data
  },

  async refreshStreamSubItem(id: string): Promise<{ stream_id: string; stripe_sub_item_id: string; plan_name: string }> {
    const { data } = await api.post(`/api/billing/ubb/streams/${id}/refresh-sub-item`, {})
    return data
  },
}
