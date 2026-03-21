import { Lightbulb } from 'lucide-react'
import { Recommendation } from '../../services/finops.service'
import { formatCurrency } from '../../utils/formatters'

interface RecommendationCardProps {
  recommendation: Recommendation
}

export default function RecommendationCard({ recommendation }: RecommendationCardProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-start gap-3">
        <Lightbulb className="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2 flex-wrap">
            <span className="text-sm font-medium text-gray-900 dark:text-white capitalize">
              {recommendation.type.replace(/_/g, ' ')}
            </span>
            <span className="text-sm font-bold text-green-600 dark:text-green-400">
              Save {formatCurrency(recommendation.potential_monthly_savings)}/mo
            </span>
          </div>
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">{recommendation.description}</p>
          {recommendation.resource_id && (
            <p className="mt-1 text-xs font-mono text-gray-500 dark:text-gray-400">{recommendation.resource_id}</p>
          )}
        </div>
      </div>
    </div>
  )
}
