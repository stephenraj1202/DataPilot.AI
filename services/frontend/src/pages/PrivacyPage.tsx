import { Link } from 'react-router-dom'
import Logo from '../components/common/Logo'

const LAST_UPDATED = 'March 21, 2026'

const SECTIONS = [
  {
    title: '1. Introduction',
    body: `DataPilot.AI ("we", "us", "our") is committed to protecting your personal information. This Privacy Policy explains what data we collect, how we use it, and your rights regarding that data. By using the Service, you consent to the practices described in this Policy.`,
  },
  {
    title: '2. Information We Collect',
    body: `We collect the following categories of information:\n\n• Account Information: Your name, email address, and profile picture obtained via Google OAuth when you sign in.\n\n• Cloud Credentials: Read-only API keys and access tokens for AWS, Azure, and GCP that you voluntarily provide to connect cloud accounts. These are encrypted at rest using AES-256.\n\n• Usage Data: Information about how you interact with the Service, including pages visited, queries executed, features used, and timestamps.\n\n• Billing Information: Payment method details are processed and stored by Stripe. We do not store full card numbers on our servers.\n\n• Log Data: IP addresses, browser type, operating system, referring URLs, and error logs for security and debugging purposes.`,
  },
  {
    title: '3. How We Use Your Information',
    body: `We use your information to:\n\n• Provide, operate, and improve the Service\n• Authenticate your identity and maintain your session\n• Process billing and subscription management via Stripe\n• Send transactional emails (account verification, billing receipts, password resets)\n• Detect and prevent fraud, abuse, and security incidents\n• Analyse aggregate usage patterns to improve product features\n• Comply with legal obligations`,
  },
  {
    title: '4. Data Sharing & Third Parties',
    body: `We do not sell your personal data. We share data only with:\n\n• Stripe: For payment processing. Stripe's privacy policy applies to payment data.\n• Google: For OAuth authentication. Google's privacy policy applies to sign-in data.\n• Cloud Providers (AWS/Azure/GCP): Your credentials are used solely to fetch cost and resource data on your behalf.\n• Infrastructure Providers: Hosting and database services under strict data processing agreements.\n• Legal Authorities: When required by law, court order, or to protect our rights.`,
  },
  {
    title: '5. Data Retention',
    body: `We retain your data for as long as your account is active or as needed to provide the Service. Specifically:\n\n• Account data: Retained until account deletion + 30 days\n• Cloud cost data: Retained for 24 months for trend analysis\n• Audit logs: Retained for 12 months\n• Billing records: Retained for 7 years as required by financial regulations\n\nYou may request deletion of your account and associated data at any time via Settings or by emailing privacy@datapilot.co.in.`,
  },
  {
    title: '6. Security',
    body: `We implement industry-standard security measures including:\n\n• AES-256 encryption for cloud credentials at rest\n• TLS 1.3 for all data in transit\n• JWT-based authentication with short-lived access tokens\n• Role-based access control (RBAC)\n• Regular security audits and penetration testing\n• Rate limiting and DDoS protection\n\nNo method of transmission over the internet is 100% secure. We cannot guarantee absolute security but are committed to protecting your data.`,
  },
  {
    title: '7. Cookies & Tracking',
    body: `We use minimal cookies:\n\n• Session cookies: Required for authentication (httpOnly, secure, SameSite=Strict)\n• Preference cookies: To remember your theme and UI preferences\n\nWe do not use third-party advertising cookies or tracking pixels. We do not use Google Analytics or similar tracking services.`,
  },
  {
    title: '8. Your Rights',
    body: `Depending on your jurisdiction, you may have the following rights:\n\n• Access: Request a copy of the personal data we hold about you\n• Correction: Request correction of inaccurate data\n• Deletion: Request deletion of your account and personal data\n• Portability: Request your data in a machine-readable format\n• Objection: Object to certain processing activities\n• Restriction: Request restriction of processing in certain circumstances\n\nTo exercise any of these rights, contact us at privacy@datapilot.co.in. We will respond within 30 days.`,
  },
  {
    title: '9. Children\'s Privacy',
    body: `The Service is not directed to individuals under the age of 16. We do not knowingly collect personal information from children. If you believe we have inadvertently collected data from a child, please contact us immediately at privacy@datapilot.co.in.`,
  },
  {
    title: '10. International Data Transfers',
    body: `DataPilot.AI is based in the United States. If you access the Service from outside the US, your data may be transferred to and processed in the US. We ensure appropriate safeguards are in place for international transfers in compliance with applicable data protection laws, including GDPR Standard Contractual Clauses where required.`,
  },
  {
    title: '11. Changes to This Policy',
    body: `We may update this Privacy Policy from time to time. We will notify you of material changes via email or a prominent notice on the Service at least 14 days before the changes take effect. Your continued use of the Service after the effective date constitutes acceptance of the updated Policy.`,
  },
  {
    title: '12. Contact Us',
    body: `For privacy-related questions, requests, or concerns:\n\nDataPilot.AI Privacy Team\nEmail: privacy@datapilot.co.in\nAddress: 19/6, 4th Cross Street, Adambakkam, Chennai - 600088, India\nPhone: +91 9965099462\n\nFor EU/UK residents, our Data Protection Officer can be reached at legal@datapilot.co.in.`,
  },
]

export default function PrivacyPage() {
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
          <h1 className="text-4xl font-extrabold tracking-tight">Privacy Policy</h1>
          <p className="mt-3 text-sm text-gray-500">Last updated: {LAST_UPDATED}</p>
          <p className="mt-4 max-w-2xl text-base leading-relaxed text-gray-400">
            Your privacy matters to us. This policy explains how DataPilot.AI collects, uses, and protects your personal information when you use our cloud cost intelligence platform.
          </p>
        </div>

        {/* Quick summary */}
        <div className="mb-10 rounded-2xl border border-indigo-500/20 bg-indigo-500/5 p-6">
          <p className="mb-3 text-xs font-bold uppercase tracking-widest text-indigo-400">TL;DR Summary</p>
          <ul className="space-y-2 text-sm text-gray-300">
            {[
              'We collect only what we need to run the Service.',
              'We never sell your data to third parties.',
              'Cloud credentials are AES-256 encrypted and never shared.',
              'You can delete your account and data at any time.',
              'We use Stripe for payments — we never store card numbers.',
            ].map(item => (
              <li key={item} className="flex items-start gap-2">
                <span className="mt-1 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-indigo-400" />
                {item}
              </li>
            ))}
          </ul>
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
            <Link to="/terms" className="hover:text-gray-400 transition-colors">Terms of Service</Link>
            <Link to="/" className="hover:text-gray-400 transition-colors">Home</Link>
          </div>
        </div>
      </main>
    </div>
  )
}
