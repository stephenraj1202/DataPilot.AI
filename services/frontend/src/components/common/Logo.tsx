interface LogoProps {
  size?: number
  className?: string
  iconOnly?: boolean
  variant?: 'light' | 'dark'
}

export default function Logo({ size = 32, iconOnly = false, variant = 'dark', className = '' }: LogoProps) {
  const textColor = variant === 'light' ? 'text-white' : 'text-gray-900 dark:text-white'
  const subColor  = variant === 'light' ? 'text-violet-300' : 'text-violet-500 dark:text-violet-400'
  const tagColor  = variant === 'light' ? 'text-indigo-200' : 'text-gray-400 dark:text-gray-500'

  return (
    <div className={`flex items-center gap-2.5 select-none ${className}`}>
      {/* ── Icon mark ── */}
      <svg
        width={size}
        height={size}
        viewBox="0 0 44 44"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
      >
        <defs>
          <linearGradient id="dp-bg" x1="0" y1="0" x2="44" y2="44" gradientUnits="userSpaceOnUse">
            <stop offset="0%"   stopColor="#4f46e5" />
            <stop offset="100%" stopColor="#7c3aed" />
          </linearGradient>
          <linearGradient id="dp-shine" x1="0" y1="0" x2="44" y2="44" gradientUnits="userSpaceOnUse">
            <stop offset="0%"   stopColor="#ffffff" stopOpacity="0.18" />
            <stop offset="100%" stopColor="#ffffff" stopOpacity="0" />
          </linearGradient>
        </defs>

        {/* Background pill */}
        <rect width="44" height="44" rx="12" fill="url(#dp-bg)" />
        {/* Subtle top-left shine */}
        <rect width="44" height="44" rx="12" fill="url(#dp-shine)" />

        {/* Pilot / compass needle — rotated 45° */}
        {/* Outer ring */}
        <circle cx="22" cy="22" r="11" stroke="white" strokeWidth="1.6" strokeOpacity="0.35" fill="none" />

        {/* North needle (bright) */}
        <polygon points="22,11 24.2,21 22,19.5 19.8,21" fill="white" fillOpacity="0.95" />
        {/* South needle (dim) */}
        <polygon points="22,33 19.8,23 22,24.5 24.2,23" fill="white" fillOpacity="0.4" />

        {/* Center dot */}
        <circle cx="22" cy="22" r="2.2" fill="white" />

        {/* Sparkle top-right — "AI" hint */}
        <circle cx="33" cy="11" r="1.4" fill="#a5b4fc" />
        <line x1="33" y1="7.5" x2="33" y2="8.8"  stroke="#a5b4fc" strokeWidth="1.2" strokeLinecap="round" />
        <line x1="33" y1="13.2" x2="33" y2="14.5" stroke="#a5b4fc" strokeWidth="1.2" strokeLinecap="round" />
        <line x1="29.5" y1="11" x2="30.8" y2="11" stroke="#a5b4fc" strokeWidth="1.2" strokeLinecap="round" />
        <line x1="35.2" y1="11" x2="36.5" y2="11" stroke="#a5b4fc" strokeWidth="1.2" strokeLinecap="round" />
      </svg>

      {!iconOnly && (
        <div className="flex flex-col leading-none gap-[2px]">
          <span className={`text-[15px] font-black tracking-tight ${textColor}`}>
            DataPilot<span className={subColor}>.AI</span>
          </span>
          <span className={`text-[9px] font-semibold uppercase tracking-[0.12em] ${tagColor}`}>
            FinOps Platform
          </span>
        </div>
      )}
    </div>
  )
}
