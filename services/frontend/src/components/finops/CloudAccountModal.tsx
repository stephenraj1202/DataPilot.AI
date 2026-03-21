import { useState, useEffect } from 'react'
import { X, Cloud, ChevronDown, Loader2, CheckCircle2, AlertCircle } from 'lucide-react'
import { finopsService } from '../../services/finops.service'
import toast from 'react-hot-toast'

interface Account {
  id: string
  provider: string
  account_name: string
  status: string
  last_sync_at: string | null
}

interface Props {
  account?: Account | null   // null = add mode, Account = edit mode
  onClose: () => void
  onSaved: () => void
}

const PROVIDERS = [
  { value: 'aws',   label: 'Amazon Web Services', icon: '☁', color: '#f97316' },
  { value: 'azure', label: 'Microsoft Azure',      icon: '⬡', color: '#3b82f6' },
  { value: 'gcp',   label: 'Google Cloud',         icon: '◈', color: '#10b981' },
]

const FIELDS: Record<string, Array<{ key: string; label: string; type?: string; placeholder?: string; required?: boolean }>> = {
  aws: [
    { key: 'access_key_id',     label: 'Access Key ID',     placeholder: 'AKIAIOSFODNN7EXAMPLE', required: true },
    { key: 'secret_access_key', label: 'Secret Access Key', type: 'password', placeholder: '••••••••', required: true },
    { key: 'session_token',     label: 'Session Token',     type: 'password', placeholder: 'Optional — for temporary credentials' },
  ],
  azure: [
    { key: 'subscription_id', label: 'Subscription ID', placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx', required: true },
    { key: 'tenant_id',       label: 'Tenant ID',       placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx', required: true },
    { key: 'client_id',       label: 'Client ID',       placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx', required: true },
    { key: 'client_secret',   label: 'Client Secret',   type: 'password', placeholder: '••••••••', required: true },
  ],
  gcp: [
    { key: 'project_id',          label: 'Project ID',                  placeholder: 'my-project-123', required: true },
    { key: 'service_account_key', label: 'Service Account Key (JSON)',  type: 'textarea', placeholder: '{\n  "type": "service_account",\n  ...\n}', required: true },
    { key: 'billing_dataset',     label: 'BigQuery Billing Dataset',    placeholder: 'my_billing_dataset' },
    { key: 'billing_table',       label: 'BigQuery Billing Table',      placeholder: 'gcp_billing_export_v1_XXXXXX' },
  ],
}

export default function CloudAccountModal({ account, onClose, onSaved }: Props) {
  const isEdit = !!account
  const [provider, setProvider] = useState(account?.provider ?? 'aws')
  const [accountName, setAccountName] = useState(account?.account_name ?? '')
  const [creds, setCreds] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Reset creds when provider changes
  useEffect(() => { setCreds({}) }, [provider])

  const fields = FIELDS[provider] ?? []

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!accountName.trim()) { setError('Account name is required'); return }

    // Validate required fields
    for (const f of fields) {
      if (f.required && !creds[f.key]?.trim()) {
        setError(`${f.label} is required`)
        return
      }
    }

    setLoading(true)
    try {
      if (isEdit) {
        const payload: { account_name: string; credentials?: Record<string, string> } = {
          account_name: accountName.trim(),
        }
        // Only send credentials if any were filled in
        const filledCreds = Object.fromEntries(Object.entries(creds).filter(([, v]) => v.trim()))
        if (Object.keys(filledCreds).length > 0) payload.credentials = filledCreds
        await finopsService.updateCloudAccount(account!.id, payload)
        toast.success('Account updated')
      } else {
        await finopsService.addCloudAccount({
          provider,
          account_name: accountName.trim(),
          credentials: Object.fromEntries(Object.entries(creds).filter(([, v]) => v.trim())),
        })
        toast.success('Account connected')
      }
      onSaved()
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Something went wrong'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  const selectedProvider = PROVIDERS.find(p => p.value === provider)!

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={onClose} />

      {/* Modal */}
      <div className="relative w-full max-w-lg overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-gray-900">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl text-lg"
              style={{ background: `${selectedProvider.color}22`, color: selectedProvider.color }}>
              {selectedProvider.icon}
            </div>
            <div>
              <h2 className="text-base font-bold text-gray-900 dark:text-white">
                {isEdit ? 'Edit Account' : 'Connect Cloud Account'}
              </h2>
              <p className="text-xs text-gray-400">{isEdit ? account!.account_name : 'Add your cloud credentials'}</p>
            </div>
          </div>
          <button onClick={onClose} className="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 transition-colors">
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="max-h-[80vh] overflow-y-auto">
          <div className="space-y-4 px-6 py-5">

            {/* Provider selector — only in add mode */}
            {!isEdit && (
              <div>
                <label className="mb-1.5 block text-xs font-semibold text-gray-600 dark:text-gray-400">Cloud Provider</label>
                <div className="grid grid-cols-3 gap-2">
                  {PROVIDERS.map(p => (
                    <button
                      key={p.value}
                      type="button"
                      onClick={() => setProvider(p.value)}
                      className={`flex flex-col items-center gap-1.5 rounded-xl border-2 py-3 text-xs font-bold transition-all ${
                        provider === p.value
                          ? 'border-transparent text-white shadow-md'
                          : 'border-gray-200 bg-gray-50 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'
                      }`}
                      style={provider === p.value ? { background: `linear-gradient(135deg, ${p.color}, ${p.color}cc)` } : {}}
                    >
                      <span className="text-xl">{p.icon}</span>
                      <span>{p.label.split(' ')[0]}</span>
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Account name */}
            <div>
              <label className="mb-1.5 block text-xs font-semibold text-gray-600 dark:text-gray-400">
                Account Name <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={accountName}
                onChange={e => setAccountName(e.target.value)}
                placeholder="e.g. Production AWS, Dev Azure"
                className="w-full rounded-xl border border-gray-200 bg-gray-50 px-3.5 py-2.5 text-sm text-gray-900 placeholder-gray-400 outline-none transition focus:border-indigo-400 focus:bg-white focus:ring-2 focus:ring-indigo-100 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500 dark:focus:border-indigo-500"
              />
            </div>

            {/* Credential fields */}
            <div className="space-y-3">
              <p className="text-xs font-semibold text-gray-600 dark:text-gray-400">
                {isEdit ? 'Update Credentials (leave blank to keep existing)' : 'Credentials'}
              </p>
              {fields.map(f => (
                <div key={f.key}>
                  <label className="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">
                    {f.label}
                    {f.required && !isEdit && <span className="ml-0.5 text-red-500">*</span>}
                  </label>
                  {f.type === 'textarea' ? (
                    <textarea
                      value={creds[f.key] ?? ''}
                      onChange={e => setCreds(c => ({ ...c, [f.key]: e.target.value }))}
                      placeholder={f.placeholder}
                      rows={5}
                      className="w-full rounded-xl border border-gray-200 bg-gray-50 px-3.5 py-2.5 font-mono text-xs text-gray-900 placeholder-gray-400 outline-none transition focus:border-indigo-400 focus:bg-white focus:ring-2 focus:ring-indigo-100 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500"
                    />
                  ) : (
                    <input
                      type={f.type ?? 'text'}
                      value={creds[f.key] ?? ''}
                      onChange={e => setCreds(c => ({ ...c, [f.key]: e.target.value }))}
                      placeholder={f.placeholder}
                      className="w-full rounded-xl border border-gray-200 bg-gray-50 px-3.5 py-2.5 text-sm text-gray-900 placeholder-gray-400 outline-none transition focus:border-indigo-400 focus:bg-white focus:ring-2 focus:ring-indigo-100 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500"
                    />
                  )}
                </div>
              ))}
            </div>

            {/* Error */}
            {error && (
              <div className="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 dark:border-red-900/40 dark:bg-red-950/30">
                <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0 text-red-500" />
                <p className="text-sm text-red-700 dark:text-red-400">{error}</p>
              </div>
            )}
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 border-t border-gray-100 px-6 py-4 dark:border-gray-800">
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm font-semibold text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex items-center gap-2 rounded-xl px-5 py-2 text-sm font-semibold text-white shadow-md transition hover:opacity-90 disabled:opacity-70"
              style={{ background: `linear-gradient(135deg, ${selectedProvider.color}, ${selectedProvider.color}cc)` }}
            >
              {loading ? (
                <><Loader2 className="h-4 w-4 animate-spin" /> {isEdit ? 'Saving...' : 'Connecting...'}</>
              ) : (
                <><CheckCircle2 className="h-4 w-4" /> {isEdit ? 'Save Changes' : 'Connect Account'}</>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
