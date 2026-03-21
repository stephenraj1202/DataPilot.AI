import api from './api'

export interface CostSummary {
  total_cost: number
  forecast_cost: number
  currency: string
  breakdown_by_provider: Record<string, number>
  breakdown_by_service: Array<{ service: string; provider: string; cost: number }>
  service_count_by_provider: Record<string, number>
  daily_costs?: Array<{ date: string; cost: number }>
  monthly_trends?: Array<{ month: string; cost: number }>
}

export interface Anomaly {
  id: string
  date: string
  baseline_cost: number
  actual_cost: number
  deviation_percentage: number
  severity: 'low' | 'medium' | 'high'
  contributing_services: string[]
  acknowledged: boolean
}

export interface Recommendation {
  id: string
  type: string
  resource_id: string
  description: string
  potential_monthly_savings: number
  cloud_provider?: string
}

export interface CloudAccount {
  id: string
  provider: string
  account_name: string
  status: string
  last_sync_at: string | null
}

export interface CloudAccountResources {
  id: string
  provider: string
  account_name: string
  last_sync_at: string | null
  mtd_cost: number
  service_count: number
  region_count: number
  services: Array<{ service: string; cost: number; region: string }>
  regions: string[]
}

export interface ResourceGroup {
  type: string
  label: string
  icon: string
  total_cost: number
  services: Array<{ name: string; region: string; cost: number }>
}

export interface CloudAccountVMResources {
  cloud_account_id: string
  provider: string
  resource_groups: ResourceGroup[]
}

export interface ResourceTile {
  icon: string
  label: string
  color: string
  count: number
}

export interface CloudAccountTiles {
  cloud_account_id: string
  provider: string
  account_name: string
  tiles: ResourceTile[]
}

export const finopsService = {
  async getCostSummary(startDate: string, endDate: string): Promise<CostSummary> {
    const { data } = await api.get('/api/finops/costs/summary', {
      params: { start_date: startDate, end_date: endDate },
    })
    return data
  },

  async getAnomalies(days = 30): Promise<{ anomalies: Anomaly[] }> {
    const { data } = await api.get('/api/finops/anomalies', { params: { days } })
    return data
  },

  async getRecommendations(): Promise<{ recommendations: Recommendation[]; total_potential_savings: number }> {
    const { data } = await api.get('/api/finops/recommendations')
    return data
  },

  async getCloudAccounts(): Promise<{ cloud_accounts: CloudAccount[] }> {
    const { data } = await api.get('/api/finops/cloud-accounts')
    return data
  },

  async addCloudAccount(payload: {
    provider: string
    account_name: string
    credentials: Record<string, string>
  }): Promise<CloudAccount> {
    const { data } = await api.post('/api/finops/cloud-accounts', payload)
    return data
  },

  async deleteCloudAccount(id: string): Promise<void> {
    await api.delete(`/api/finops/cloud-accounts/${id}`)
  },

  async syncCloudAccount(id: string): Promise<void> {
    await api.post(`/api/finops/cloud-accounts/${id}/sync`)
  },

  async updateCloudAccount(id: string, payload: {
    account_name: string
    credentials?: Record<string, string>
  }): Promise<void> {
    await api.put(`/api/finops/cloud-accounts/${id}`, payload)
  },

  async getCloudAccountResources(id: string): Promise<CloudAccountResources> {
    const { data } = await api.get(`/api/finops/cloud-accounts/${id}/resources`)
    return data
  },

  async getCloudAccountVMResources(id: string): Promise<CloudAccountVMResources> {
    const { data } = await api.get(`/api/finops/cloud-accounts/${id}/vm-resources`)
    return data
  },

  async getCloudAccountTiles(id: string): Promise<CloudAccountTiles> {
    const { data } = await api.get(`/api/finops/cloud-accounts/${id}/tiles`)
    return data
  },

  async sendReportNow(payload: { recipients: string[]; report_type: string }): Promise<{ message: string }> {
    const { data } = await api.post('/api/finops/reports/send', payload)
    return data
  },

  async getReportSchedules(): Promise<{ schedules: ReportSchedule[] }> {
    const { data } = await api.get('/api/finops/reports/schedules')
    return data
  },

  async createReportSchedule(payload: ReportSchedulePayload): Promise<{ id: string; next_run_at: string }> {
    const { data } = await api.post('/api/finops/reports/schedules', payload)
    return data
  },

  async updateReportSchedule(id: string, payload: ReportSchedulePayload): Promise<void> {
    await api.put(`/api/finops/reports/schedules/${id}`, payload)
  },

  async deleteReportSchedule(id: string): Promise<void> {
    await api.delete(`/api/finops/reports/schedules/${id}`)
  },
}

export interface ReportSchedule {
  id: string
  name: string
  frequency: 'daily' | 'weekly' | 'monthly'
  day_of_week: number | null
  day_of_month: number | null
  send_hour: number
  recipients: string[]
  report_type: string
  is_active: boolean
  last_sent_at: string | null
  next_run_at: string
  created_at: string
}

export interface ReportSchedulePayload {
  name: string
  frequency: 'daily' | 'weekly' | 'monthly'
  day_of_week?: number | null
  day_of_month?: number | null
  send_hour: number
  recipients: string[]
  report_type: string
  is_active: boolean
}
