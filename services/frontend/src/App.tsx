import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Suspense, lazy } from 'react'
import { ThemeProvider } from './context/ThemeContext'
import { AuthProvider } from './context/AuthContext'
import ProtectedRoute from './components/common/ProtectedRoute'
import AppLayout from './components/common/AppLayout'
import LoadingSpinner from './components/common/LoadingSpinner'

// Lazy-load pages for better initial load performance
const LandingPage = lazy(() => import('./pages/LandingPage'))
const LoginPage = lazy(() => import('./pages/LoginPage'))
const OtpPage = lazy(() => import('./pages/OtpPage'))
const ForgotPasswordPage = lazy(() => import('./pages/ForgotPasswordPage'))
const ResetPasswordPage = lazy(() => import('./pages/ResetPasswordPage'))
const DashboardPage = lazy(() => import('./pages/DashboardPage'))
const FinOpsPage = lazy(() => import('./pages/FinOpsPage'))
const QueryPage = lazy(() => import('./pages/QueryPage'))
const BillingPage = lazy(() => import('./pages/BillingPage'))
const BillingSuccessPage = lazy(() => import('./pages/BillingSuccessPage'))
const SettingsPage = lazy(() => import('./pages/SettingsPage'))
const UBBPage = lazy(() => import('./pages/UBBPage'))
const DocsPage = lazy(() => import('./pages/DocsPage'))
const TermsPage = lazy(() => import('./pages/TermsPage'))
const PrivacyPage = lazy(() => import('./pages/PrivacyPage'))

function PageLoader() {
  return (
    <div className="flex h-screen items-center justify-center">
      <LoadingSpinner size="lg" />
    </div>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <BrowserRouter>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              {/* Public routes */}
              <Route path="/" element={<LandingPage />} />
              <Route path="/login" element={<LoginPage />} />
              <Route path="/verify-otp" element={<OtpPage />} />
              <Route path="/forgot-password" element={<ForgotPasswordPage />} />
              <Route path="/reset-password" element={<ResetPasswordPage />} />
              <Route path="/terms" element={<TermsPage />} />
              <Route path="/privacy" element={<PrivacyPage />} />

              {/* Protected routes */}
              <Route element={<ProtectedRoute />}>
                <Route element={<AppLayout />}>
                  <Route path="/dashboard" element={<DashboardPage />} />
                  <Route path="/finops" element={<FinOpsPage />} />
                  <Route path="/query" element={<QueryPage />} />
                  <Route path="/billing" element={<BillingPage />} />
                  <Route path="/billing/success" element={<BillingSuccessPage />} />
                  <Route path="/billing/ubb" element={<UBBPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="/docs" element={<DocsPage />} />
                </Route>
              </Route>

              {/* Fallback */}
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </BrowserRouter>
      </AuthProvider>
    </ThemeProvider>
  )
}
