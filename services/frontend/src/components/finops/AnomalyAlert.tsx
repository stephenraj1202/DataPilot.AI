import { AlertTriangle } from 'lucide-react'
import { Anomaly } from '../../services/finops.service'
import { formatCurrency, formatDate, formatPercent, severityColor } from '../../utils/formatters'

interface AnomalyAlertProps {
  anomaly: Anomaly
}

export default function AnomalyAlert({ anomaly }: AnomalyAlertProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-yellow-500" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium text-gray-900 dark:text-white">
              Cost Anomaly — {formatDate(anomaly.date)}
            </span>
            <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${severityColor(anomaly.severity)}`}>
              {anomaly.severity.toUpperCase()}
            </span>
          </div>
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
            Actual: <strong>{formatCurrency(anomaly.actual_cost)}</strong> vs Baseline:{' '}
            <strong>{formatCurrency(anomaly.baseline_cost)}</strong> (+{formatPercent(anomaly.deviation_percentage)})
          </p>
          {anomaly.contributing_services?.length > 0 && (
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Services: {anomaly.contributing_services.join(', ')}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
