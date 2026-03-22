import { useQuery } from '@tanstack/react-query'
import { billingService } from '../services/billing.service'

export function usePaymentMode() {
  const { data } = useQuery({
    queryKey: ['payment-mode'],
    queryFn: () => billingService.getPaymentMode(),
    staleTime: Infinity,
  })
  const mode = data?.mode ?? 'stripe'
  const label = mode === 'razorpay' ? 'Razorpay' : 'Stripe'
  return { mode, label }
}
