import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useQuery } from '@tanstack/react-query'
import { billingService } from '../../services/billing.service'
import LoadingSpinner from './LoadingSpinner'

export default function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth()
  const location = useLocation()

  const { data: subData, isLoading: subLoading } = useQuery({
    queryKey: ['subscription'],
    queryFn: () => billingService.getSubscription(),
    enabled: isAuthenticated,
    retry: false,
    staleTime: 5 * 60 * 1000,
  })

  if (isLoading || (isAuthenticated && subLoading)) {
    return (
      <div className="flex h-screen items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  if (!isAuthenticated) return <Navigate to="/login" replace />

  const sub = subData?.subscription
  const isBillingPage = location.pathname === '/billing'

  // Only block access if a paid subscription has expired (free plan is free forever)
  if (sub && !isBillingPage) {
    const periodEnd = new Date(sub.current_period_end)
    const isExpired = periodEnd < new Date()
    const isPaidExpired = sub.plan.price_cents > 0 && isExpired && sub.status !== 'active'

    if (isPaidExpired) {
      return <Navigate to="/billing" replace state={{ trialExpired: true }} />
    }
  }

  return <Outlet />
}
