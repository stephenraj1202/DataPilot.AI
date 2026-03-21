import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { CheckCircle, Loader2, XCircle } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { billingService } from '../services/billing.service'

export default function BillingSuccessPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const [state, setState] = useState<'loading' | 'success' | 'error'>('loading')
  const [planName, setPlanName] = useState('')

  useEffect(() => {
    const sessionId = searchParams.get('session_id')
    if (!sessionId) { navigate('/billing'); return }

    billingService.confirmCheckoutSession(sessionId)
      .then((res) => {
        setPlanName(res.plan)
        setState('success')
        queryClient.invalidateQueries({ queryKey: ['subscription'] })
        setTimeout(() => navigate('/billing'), 3000)
      })
      .catch(() => {
        setState('error')
        setTimeout(() => navigate('/billing'), 3000)
      })
  }, [navigate, queryClient, searchParams])

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 dark:bg-gray-900">
      <div className="rounded-2xl bg-white p-10 text-center shadow-lg dark:bg-gray-800 w-80">
        {state === 'loading' && (
          <>
            <Loader2 className="mx-auto h-16 w-16 animate-spin text-indigo-500" />
            <h1 className="mt-4 text-2xl font-bold text-gray-900 dark:text-white">Activating Plan</h1>
            <p className="mt-2 text-gray-500 dark:text-gray-400">Please wait...</p>
          </>
        )}
        {state === 'success' && (
          <>
            <CheckCircle className="mx-auto h-16 w-16 text-green-500" />
            <h1 className="mt-4 text-2xl font-bold text-gray-900 dark:text-white">Payment Successful</h1>
            <p className="mt-2 text-gray-500 dark:text-gray-400 capitalize">{planName} plan is now active. Redirecting...</p>
          </>
        )}
        {state === 'error' && (
          <>
            <XCircle className="mx-auto h-16 w-16 text-red-500" />
            <h1 className="mt-4 text-2xl font-bold text-gray-900 dark:text-white">Something went wrong</h1>
            <p className="mt-2 text-gray-500 dark:text-gray-400">Redirecting to billing...</p>
          </>
        )}
      </div>
    </div>
  )
}
