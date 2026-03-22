import { Link } from 'react-router-dom'
import Logo from '../components/common/Logo'

const LAST_UPDATED = 'March 21, 2026'

const SECTIONS = [
  {
    title: '1. Acceptance of Terms',
    body: `By accessing or using DataPilot.AI ("the Service"), you agree to be bound by these Terms of Service ("Terms"). If you do not agree to these Terms, do not use the Service. These Terms apply to all visitors, users, and others who access or use the Service.`,
  },
  {
    title: '2. Description of Service',
    body: `DataPilot.AI is a cloud cost intelligence platform that provides multi-cloud cost monitoring, AI-powered analytics, natural language database querying, and usage-based billing management. The Service connects to third-party cloud providers (AWS, Azure, GCP) using credentials you supply.`,
  },
  {
    title: '3. Account Registration',
    body: `You must sign in using a valid Google account. You are responsible for maintaining the confidentiality of your account credentials and for all activities that occur under your account. You agree to notify us immediately at support@datapilot.co.in of any unauthorised use of your account.`,
  },
  {
    title: '4. Acceptable Use',
    body: `You agree not to: (a) use the Service for any unlawful purpose; (b) attempt to gain unauthorised access to any part of the Service or its related systems; (c) reverse-engineer, decompile, or disassemble any part of the Service; (d) use the Service to transmit malware or harmful code; (e) resell or sublicense the Service without written permission; (f) use automated tools to scrape or extract data beyond normal API usage.`,
  },
  {
    title: '5. Cloud Credentials & Data Security',
    body: `When you connect cloud accounts, you provide read-only API credentials. These credentials are encrypted at rest using AES-256 and are never shared with third parties. You are solely responsible for ensuring the credentials you provide have appropriate, minimal permissions. DataPilot.AI is not liable for any costs, charges, or security incidents arising from misconfigured cloud permissions.`,
  },
  {
    title: '6. Billing & Payments',
    body: `Paid plans are billed monthly via our payment gateway. All charges are in USD. You authorise us to charge your payment method on a recurring basis. Subscriptions auto-renew unless cancelled before the renewal date. Usage-based overage charges are calculated at the end of each billing period and charged separately. Refunds are issued at our discretion within 7 days of a charge for technical failures attributable to DataPilot.AI.`,
  },
  {
    title: '7. Intellectual Property',
    body: `The Service, including all software, algorithms, designs, and content, is owned by DataPilot.AI and protected by applicable intellectual property laws. You retain ownership of your data. By using the Service, you grant DataPilot.AI a limited, non-exclusive licence to process your data solely to provide the Service.`,
  },
  {
    title: '8. Limitation of Liability',
    body: `To the maximum extent permitted by law, DataPilot.AI shall not be liable for any indirect, incidental, special, consequential, or punitive damages, including loss of profits, data, or goodwill, arising from your use of or inability to use the Service. Our total liability for any claim shall not exceed the amount you paid us in the 3 months preceding the claim.`,
  },
  {
    title: '9. Disclaimer of Warranties',
    body: `The Service is provided "as is" and "as available" without warranties of any kind, either express or implied, including but not limited to implied warranties of merchantability, fitness for a particular purpose, or non-infringement. We do not warrant that the Service will be uninterrupted, error-free, or free of viruses.`,
  },
  {
    title: '10. Termination',
    body: `We may suspend or terminate your access to the Service at any time, with or without cause, with or without notice. Upon termination, your right to use the Service ceases immediately. You may delete your account at any time from the Settings page. Data deletion requests are processed within 30 days.`,
  },
  {
    title: '11. Governing Law',
    body: `These Terms are governed by the laws of the State of Delaware, United States, without regard to its conflict of law provisions. Any disputes shall be resolved exclusively in the state or federal courts located in Delaware.`,
  },
  {
    title: '12. Changes to Terms',
    body: `We reserve the right to modify these Terms at any time. We will notify you of material changes via email or a prominent notice on the Service. Continued use of the Service after changes constitutes acceptance of the new Terms.`,
  },
  {
    title: '13. Contact',
    body: `For questions about these Terms, contact us at:\n\nDataPilot.AI Legal Team\nEmail: legal@datapilot.co.in\nAddress: 19/6, 4th Cross Street, Adambakkam, Chennai - 600088, India\nPhone: +91 9965099462`,
  },
]

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-[#07070d] text-white">
      {/* Nav */}
      <header className="sticky top-0 z-50 border-b border-white/5 bg-[#07070d]/90 backdrop-blur-xl">
        <div className="mx-auto flex max-w-4xl items-center justify-between px-5 py-4">
          <Link to="/" className="flex items-center gap-2">
            <Logo size={28} variant="light" />
          </Link>
          <Link to="/login" className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors">
            Sign In
          </Link>
        </div>
      </header>

      <main className="mx-auto max-w-4xl px-5 py-16">
        {/* Header */}
        <div className="mb-12">
          <p className="mb-3 text-xs font-semibold uppercase tracking-widest text-indigo-400">Legal</p>
          <h1 className="text-4xl font-extrabold tracking-tight">Terms of Service</h1>
          <p className="mt-3 text-sm text-gray-500">Last updated: {LAST_UPDATED}</p>
          <p className="mt-4 max-w-2xl text-base leading-relaxed text-gray-400">
            Please read these Terms of Service carefully before using DataPilot.AI. These Terms govern your access to and use of our cloud cost intelligence platform.
          </p>
        </div>

        {/* Sections */}
        <div className="space-y-10">
          {SECTIONS.map(s => (
            <section key={s.title} className="rounded-2xl border border-white/5 bg-white/[0.03] p-6">
              <h2 className="mb-3 text-lg font-bold text-white">{s.title}</h2>
              <p className="whitespace-pre-line text-sm leading-relaxed text-gray-400">{s.body}</p>
            </section>
          ))}
        </div>

        {/* Footer links */}
        <div className="mt-12 flex flex-wrap items-center justify-between gap-4 border-t border-white/5 pt-8 text-sm text-gray-600">
          <p>© {new Date().getFullYear()} DataPilot.AI. All rights reserved.</p>
          <div className="flex gap-5">
            <Link to="/privacy" className="hover:text-gray-400 transition-colors">Privacy Policy</Link>
            <Link to="/" className="hover:text-gray-400 transition-colors">Home</Link>
          </div>
        </div>
      </main>
    </div>
  )
}
