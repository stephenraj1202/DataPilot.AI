import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  X, RefreshCw, MapPin, Layers, DollarSign,
  Server, Database, Globe, Zap, HardDrive, Box, BarChart2,
} from 'lucide-react'
import { finopsService, type CloudAccountResources, type ResourceGroup } from '../../services/finops.service'
import { formatCurrency } from '../../utils/formatters'
import LoadingSpinner from '../common/LoadingSpinner'

interface Props {
  accountId: string
  accountName: string
  provider: string
  onClose: () => void
  onSync: () => void
  syncing: boolean
}

const PROVIDER_CONFIG: Record<string, { gradient: string; icon: string }> = {
  aws:   { gradient: 'from-orange-500 to-amber-400',  icon: '🟡' },
  azure: { gradient: 'from-blue-600 to-cyan-500',     icon: '🔵' },
  gcp:   { gradient: 'from-green-500 to-emerald-400', icon: '🟢' },
}

const RANK_COLORS = [
  'bg-indigo-500', 'bg-blue-500', 'bg-cyan-500', 'bg-teal-500',
  'bg-green-500', 'bg-amber-500', 'bg-orange-500', 'bg-rose-500',
  'bg-purple-500', 'bg-pink-500',
]

const GROUP_COLORS: Record<string, string> = {
  compute:    'bg-orange-500',
  database:   'bg-blue-500',
  storage:    'bg-teal-500',
  serverless: 'bg-purple-500',
  containers: 'bg-cyan-500',
  networking: 'bg-green-500',
  other:      'bg-gray-400',
}

function ServiceIcon({ name }: { name: string }) {
  const n = name.toLowerCase()
  if (n.includes('ec2') || n.includes('compute') || n.includes('virtual machine'))
    return <Server className="h-4 w-4" />
  if (n.includes('rds') || n.includes('sql') || n.includes('database') || n.includes('aurora'))
    return <Database className="h-4 w-4" />
  if (n.includes('s3') || n.includes('storage') || n.includes('blob') || n.includes('gcs'))
    return <HardDrive className="h-4 w-4" />
  if (n.includes('lambda') || n.includes('function'))
    return <Zap className="h-4 w-4" />
  if (n.includes('eks') || n.includes('aks') || n.includes('gke') || n.includes('kubernetes'))
    return <Box className="h-4 w-4" />
  if (n.includes('cloudfront') || n.includes('cdn') || n.includes('network'))
    return <Globe className="h-4 w-4" />
  return <Layers className="h-4 w-4" />
}

function ResourceGroupCard({ group }: { group: ResourceGroup }) {
  const [expanded, setExpanded] = useState(false)
  const colorClass = GROUP_COLORS[group.type] ?? 'bg-gray-400'
  const maxCost = group.services[0]?.cost ?? 1

  const iconEl = (() => {
    switch (group.icon) {
      case 'server':    return <Server className="h-4 w-4" />
      case 'database':  return <Database className="h-4 w-4" />
      case 'harddrive': return <HardDrive className="h-4 w-4" />
      case 'zap':       return <Zap className="h-4 w-4" />
      case 'box':       return <Box className="h-4 w-4" />
      case 'globe':     return <Globe className="h-4 w-4" />
      default:          return <Layers className="h-4 w-4" />
    }
  })()

  return (
    <div className="rounded-xl border border-gray-100 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <button
        onClick={() => setExpanded(e => !e)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left"
      >
        <div className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg ${colorClass} text-white`}>
          {iconEl}
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-semibold text-gray-800 dark:text-white">{group.label}</p>
          <p className="text-xs text-gray-400">
            {group.services.length} service{group.services.length !== 1 ? 's' : ''}
          </p>
        </div>
        <div className="flex-shrink-0 text-right">
          <p className="text-sm font-bold text-gray-900 dark:text-white">{formatCurrency(group.total_cost)}</p>
          <p className="text-xs text-gray-400">{expanded ? '▲' : '▼'}</p>
        </div>
      </button>

      {expanded && (
        <div className="space-y-2 border-t border-gray-100 px-4 pb-3 pt-2 dark:border-gray-700">
          {group.services.map((s, i) => {
            const pct = maxCost > 0 ? (s.cost / maxCost) * 100 : 0
            return (
              <div key={i} className="flex items-start gap-2">
                <div className={`mt-0.5 flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md ${RANK_COLORS[i % RANK_COLORS.length]} text-white`}>
                  <ServiceIcon name={s.name} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-1">
                    <p className="truncate text-xs font-medium text-gray-700 dark:text-gray-300" title={s.name}>
                      {s.name}
                    </p>
                    <span className="flex-shrink-0 text-xs font-bold text-gray-800 dark:text-white">
                      {formatCurrency(s.cost)}
                    </span>
                  </div>
                  <div className="mt-0.5 flex items-center gap-1.5">
                    <MapPin className="h-2.5 w-2.5 flex-shrink-0 text-gray-400" />
                    <span className="truncate text-xs text-gray-400">{s.region}</span>
                  </div>
                  <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                    <div className={`h-1 rounded-full ${colorClass}`} style={{ width: `${pct}%` }} />
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

export default function CloudAccountPanel({ accountId, accountName, provider, onClose, onSync, syncing }: Props) {
  const [tab, setTab] = useState<'costs' | 'resources'>('costs')
  const cfg = PROVIDER_CONFIG[provider.toLowerCase()] ?? { gradient: 'from-gray-500 to-gray-400', icon: '⚪' }

  const { data: resources, isLoading: loadingResources } = useQuery<CloudAccountResources>({
    queryKey: ['account-resources', accountId],
    queryFn: () => finopsService.getCloudAccountResources(accountId),
  })

  const { data: vmResources, isLoading: loadingVM } = useQuery({
    queryKey: ['account-vm-resources', accountId],
    queryFn: () => finopsService.getCloudAccountVMResources(accountId),
    enabled: tab === 'resources',
  })

  const maxServiceCost = resources?.services?.[0]?.cost ?? 1

  return (
    <div className="flex h-full min-h-0 flex-col bg-gray-50 dark:bg-gray-900 overflow-hidden">
      {/* Header */}
      <div className={`bg-gradient-to-r ${cfg.gradient} px-5 py-4`}>
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <span className="text-2xl">{cfg.icon}</span>
            <div>
              <h2 className="text-base font-bold text-white leading-tight">{accountName}</h2>
              <p className="text-xs text-white/70 uppercase tracking-wide">{provider}</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={onSync}
              disabled={syncing}
              className="flex items-center gap-1.5 rounded-lg bg-white/20 px-3 py-1.5 text-xs font-medium text-white hover:bg-white/30 disabled:opacity-50 transition-colors"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${syncing ? 'animate-spin' : ''}`} />
              {syncing ? 'Syncing…' : 'Sync'}
            </button>
            <button
              onClick={onClose}
              className="rounded-lg bg-white/20 p-1.5 text-white hover:bg-white/30 transition-colors"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        {/* KPI strip */}
        <div className="mt-4 grid grid-cols-3 gap-2">
          <div className="rounded-lg bg-white/15 px-3 py-2 text-center">
            <p className="text-xs text-white/70">MTD Cost</p>
            <p className="text-sm font-bold text-white">
              {loadingResources ? '…' : formatCurrency(resources?.mtd_cost ?? 0)}
            </p>
          </div>
          <div className="rounded-lg bg-white/15 px-3 py-2 text-center">
            <p className="text-xs text-white/70">Services</p>
            <p className="text-sm font-bold text-white">
              {loadingResources ? '…' : (resources?.service_count ?? 0)}
            </p>
          </div>
          <div className="rounded-lg bg-white/15 px-3 py-2 text-center">
            <p className="text-xs text-white/70">Regions</p>
            <p className="text-sm font-bold text-white">
              {loadingResources ? '…' : (resources?.region_count ?? 0)}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
        {(['costs', 'resources'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex-1 py-2.5 text-sm font-medium transition-colors ${
              tab === t
                ? 'border-b-2 border-indigo-500 text-indigo-600 dark:text-indigo-400'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'
            }`}
          >
            {t === 'costs' ? (
              <span className="flex items-center justify-center gap-1.5">
                <DollarSign className="h-3.5 w-3.5" /> Cost Breakdown
              </span>
            ) : (
              <span className="flex items-center justify-center gap-1.5">
                <BarChart2 className="h-3.5 w-3.5" /> Resources
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4">
        {tab === 'costs' && (
          <>
            {loadingResources ? (
              <div className="flex justify-center py-10"><LoadingSpinner /></div>
            ) : !resources?.services?.length ? (
              <p className="py-10 text-center text-sm text-gray-400">No cost data — run a sync first.</p>
            ) : (
              <div className="space-y-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-gray-400">
                  Top Services by Cost
                </p>
                {resources.services.map((s, i) => {
                  const pct = maxServiceCost > 0 ? (s.cost / maxServiceCost) * 100 : 0
                  return (
                    <div key={i} className="rounded-xl border border-gray-100 bg-white p-3 shadow-sm dark:border-gray-700 dark:bg-gray-800">
                      <div className="flex items-center gap-2">
                        <div className={`flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg ${RANK_COLORS[i % RANK_COLORS.length]} text-white`}>
                          <ServiceIcon name={s.service} />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center justify-between gap-1">
                            <p className="truncate text-xs font-semibold text-gray-800 dark:text-white" title={s.service}>
                              {s.service}
                            </p>
                            <span className="flex-shrink-0 text-xs font-bold text-gray-900 dark:text-white">
                              {formatCurrency(s.cost)}
                            </span>
                          </div>
                          <div className="mt-0.5 flex items-center gap-1">
                            <MapPin className="h-2.5 w-2.5 text-gray-400" />
                            <span className="text-xs text-gray-400">{s.region}</span>
                          </div>
                          <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                            <div
                              className={`h-1.5 rounded-full ${RANK_COLORS[i % RANK_COLORS.length]}`}
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                        </div>
                      </div>
                    </div>
                  )
                })}

                {resources.regions?.length > 0 && (
                  <div className="mt-4">
                    <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
                      Active Regions
                    </p>
                    <div className="flex flex-wrap gap-1.5">
                      {resources.regions.map(r => (
                        <span key={r} className="flex items-center gap-1 rounded-full bg-indigo-50 px-2.5 py-1 text-xs font-medium text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300">
                          <MapPin className="h-2.5 w-2.5" /> {r}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
        )}

        {tab === 'resources' && (
          <>
            {loadingVM ? (
              <div className="flex justify-center py-10"><LoadingSpinner /></div>
            ) : !vmResources?.resource_groups?.length ? (
              <p className="py-10 text-center text-sm text-gray-400">No resource data — run a sync first.</p>
            ) : (
              <div className="space-y-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-gray-400">
                  Resource Groups
                </p>
                {vmResources.resource_groups.map((g, i) => (
                  <ResourceGroupCard key={i} group={g} />
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
