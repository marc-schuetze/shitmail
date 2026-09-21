import {
  Search, X as XIcon, Sun, Moon, Monitor,
} from 'lucide-react'
import { useTheme, type Theme } from '@/contexts/ThemeContext'

interface Props {
  /** What the list shows: a mailbox name or "All mailboxes". */
  context: string
  initial: string
  searchQuery: string
  onSearchChange: (q: string) => void
  searchRef: React.RefObject<HTMLInputElement>
}

const THEME_CYCLE: Record<Theme, Theme> = { dark: 'light', light: 'system', system: 'dark' }

const THEME_ICON: Record<Theme, React.ReactNode> = {
  dark:   <Moon className="size-3.5" />,
  light:  <Sun className="size-3.5" />,
  system: <Monitor className="size-3.5" />,
}

export function AppBar({
  context,
  initial,
  searchQuery,
  onSearchChange,
  searchRef,
}: Props) {
  const { theme, setTheme } = useTheme()

  return (
    <div className="h-10 shrink-0 flex items-center gap-1.5 px-3 border-b border-border bg-surface-1/90 backdrop-blur-sm">

      <span className="text-xs text-muted">{context}</span>

      {/* ── Spacer ───────────────────────────────── */}
      <div className="flex-1" />

      {/* ── Search ───────────────────────────────── */}
      <div className="flex items-center gap-1.5 bg-surface-0 border border-border rounded-lg px-2.5 h-6 w-[200px] focus-within:border-emerald-500/40 focus-within:shadow-emerald-sm transition-all">
        <Search className="size-3 text-muted shrink-0" />
        <input
          ref={searchRef}
          value={searchQuery}
          onChange={e => onSearchChange(e.target.value)}
          placeholder="Search…  ⌘K"
          className="flex-1 min-w-0 bg-transparent text-xs text-primary placeholder-muted outline-none"
        />
        {searchQuery && (
          <button
            onClick={() => onSearchChange('')}
            className="text-muted hover:text-secondary transition-colors"
          >
            <XIcon className="size-3" />
          </button>
        )}
      </div>

      {/* ── Theme cycle ──────────────────────────── */}
      <button
        onClick={() => setTheme(THEME_CYCLE[theme])}
        title={`Theme: ${theme} (click to cycle)`}
        className="icon-btn size-7"
      >
        {THEME_ICON[theme]}
      </button>

      {/* ── Avatar ───────────────────────────────── */}
      <div className="size-6 rounded-full bg-emerald-700 flex items-center justify-center text-[10px] font-bold text-white shrink-0 select-none uppercase">
        {initial}
      </div>
    </div>
  )
}
