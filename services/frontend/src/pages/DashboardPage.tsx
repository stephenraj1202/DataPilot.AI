import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { DollarSign, AlertTriangle, Database, Activity } from 'lucide-react'
import { finopsService } from '../services/finops.service'
import { queryService } from '../services/query.service'
import MetricCard from '../components/dashboard/MetricCard'
import DashboardGrid from '../components/dashboard/DashboardGrid'
import LoadingSpinner from '../components/common/LoadingSpinner'
import { formatCurrency } from '../utils/formatters'

function getDateRange() {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth(), 1)
  return {
    startDate: start.toISOString().split('T')[0],
    endDate: end.toISOString().split('T')[0],
  }
}

export default function DashboardPage() {
  const { startDate, endDate } = getDateRange()

  const { data: costData, isLoading: costLoading } = useQuery({
    queryKey: ['cost-summary', startDate, endDate],
    queryFn: () => finopsService.getCostSummary(startDate, endDate),
    refetchInterval: 5 * 60 * 1000,
  })

  const { data: anomalyData, isLoading: anomalyLoading } = useQuery({
    queryKey: ['anomalies', 30],
    queryFn: () => finopsService.getAnomalies(30),
    refetchInterval: 5 * 60 * 1000,
  })

  const { data: connectionsData } = useQuery({
    queryKey: ['connections'],
    queryFn: () => queryService.getConnections(),
    refetchInterval: 5 * 60 * 1000,
  })

  const { data: cloudAccountsData } = useQuery({
    queryKey: ['cloud-accounts'],
    queryFn: () => finopsService.getCloudAccounts(),
    refetchInterval: 5 * 60 * 1000,
  })

  const isLoading = costLoading || anomalyLoading

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  const anomalyCount = anomalyData?.anomalies?.filter(a => !a.acknowledged).length ?? 0
  const highAnomalies = anomalyData?.anomalies?.filter(a => a.severity === 'high' && !a.acknowledged).length ?? 0

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Dashboard</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Overview for {new Date().toLocaleString('default', { month: 'long', year: 'numeric' })}
        </p>
      </div>

      <DashboardGrid>
        <MetricCard
          title="Total Monthly Cost"
          value={formatCurrency(costData?.total_cost ?? 0)}
          subtitle={`${costData?.currency ?? 'USD'} this month`}
          icon={DollarSign}
          color="blue"
        />
        <MetricCard
          title="Active Anomalies"
          value={anomalyCount}
          subtitle={`${highAnomalies} high severity`}
          icon={AlertTriangle}
          color={anomalyCount > 0 ? 'red' : 'green'}
        />
        <MetricCard
          title="Database Connections"
          value={connectionsData?.connections?.length ?? 0}
          subtitle="Connected databases"
          icon={Database}
          color="green"
        />
        <MetricCard
          title="Cloud Accounts"
          value={cloudAccountsData?.cloud_accounts?.length ?? 0}
          subtitle="Connected providers"
          icon={Activity}
          color="yellow"
        />
      </DashboardGrid>

      {/* Provider breakdown */}
      {costData?.breakdown_by_provider && Object.keys(costData.breakdown_by_provider).length > 0 && (
        <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <h2 className="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-300">Cost by Provider</h2>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {Object.entries(costData.breakdown_by_provider).map(([provider, cost]) => (
              <div key={provider} className="rounded-lg bg-gray-50 p-3 dark:bg-gray-700/50">
                <p className="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">{provider}</p>
                <p className="mt-1 text-lg font-bold text-gray-900 dark:text-white">{formatCurrency(cost)}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent anomalies */}
      {anomalyData?.anomalies && anomalyData.anomalies.length > 0 && (
        <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <h2 className="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-300">Recent Anomalies</h2>
          <div className="space-y-2">
            {anomalyData.anomalies.slice(0, 3).map(a => (
              <div key={a.id} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 dark:bg-gray-700/50">
                <span className="text-sm text-gray-700 dark:text-gray-300">{a.date}</span>
                <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                  a.severity === 'high' ? 'bg-red-100 text-red-700' :
                  a.severity === 'medium' ? 'bg-yellow-100 text-yellow-700' :
                  'bg-blue-100 text-blue-700'
                }`}>
                  {a.severity} +{a.deviation_percentage.toFixed(0)}%
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
