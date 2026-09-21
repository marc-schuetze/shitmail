import { useState, useEffect, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Inbox, Star, Paperclip, Settings,
  Copy, Plus, Pin, PinOff, Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import { cn, formatCountdown, isForever, mailboxName } from '@/lib/utils'
import { TTL_OPTIONS, TTL_FOREVER, type TTLHours } from '@/hooks/useSettings'
import type { MailboxTab } from '@/hooks/useMailboxTabs'

export type NavView = 'inbox' | 'starred' | 'attachments' | 'settings'

interface Props {
  tabs: MailboxTab[]
  activeTabId: string
  onSelectTab: (id: string) => void
  allActive: boolean
  onSelectAll: () => void
  onAddTab: (localPart?: string, ttlHours?: number) => Promise<void>
  onDeleteTab: (id: string) => Promise<void>
  onSetTabTTL: (id: string, ttlHours: number) => Promise<void>

  loading: boolean
  emailCount: number
  unreadCount: number
  starredCount: number
  attachmentEmailCount: number

  view: NavView
  onViewChange: (v: NavView) => void
  defaultTTLHours: TTLHours

  /** "-<tag>@<domain>" appended to every address the user creates. */
  addressSuffix?: string
}

// ── Short expiry label for a row ("6d", "3h", "12m", "exp") ─────────────────

function shortExpiry(expiresAt: string): string {
  const l = formatCountdown(expiresAt)
  if (l === 'expired') return 'exp'
  return l.split(' ')[0]
}

function RowExpiry({ expiresAt }: { expiresAt: string }) {
  const [label, setLabel] = useState(() => shortExpiry(expiresAt))
  useEffect(() => {
    const id = setInterval(() => setLabel(shortExpiry(expiresAt)), 30_000)
    return () => clearInterval(id)
  }, [expiresAt])
  return <span className="text-[9px] font-mono text-muted tabular-nums shrink-0">{label}</span>
}

// ── Logo mark ─────────────────────────────────────────────────────────────────

function LogoMark() {
  return (
    <img
      src="/favicon-96x96.png"
      alt="shitmail"
      className="size-7 rounded-lg shrink-0 object-cover"
    />
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function NavSidebar({
  tabs,
  activeTabId,
  onSelectTab,
  allActive,
  onSelectAll,
  onAddTab,
  onDeleteTab,
  onSetTabTTL,
  loading,
  emailCount,
  unreadCount,
  starredCount,
  attachmentEmailCount,
  view,
  onViewChange,
  defaultTTLHours,
  addressSuffix,
}: Props) {
  const [showNewForm, setShowNewForm] = useState(false)
  const [newLocalPart, setNewLocalPart] = useState('')
  const [newTTL, setNewTTL] = useState<TTLHours>(defaultTTLHours)
  const [creating, setCreating] = useState(false)
  // Row whose delete is awaiting the second click; resets by itself.
  const [confirmId, setConfirmId] = useState<string | null>(null)
  useEffect(() => {
    if (!confirmId) return
    const t = setTimeout(() => setConfirmId(null), 6_000)
    return () => clearTimeout(t)
  }, [confirmId])

  useEffect(() => { setNewTTL(defaultTTLHours) }, [defaultTTLHours])

  // Always copies the FULL address (name-tag@domain); the UI hides the tag.
  const copyAddress = useCallback((address: string) => {
    navigator.clipboard.writeText(address).then(() => toast.success(`Copied ${address}`))
  }, [])

  const handleCreate = useCallback(async () => {
    setCreating(true)
    try {
      await onAddTab(newLocalPart.trim() || undefined, newTTL)
      setShowNewForm(false)
      setNewLocalPart('')
    } catch (err) {
      toast.error((err as Error).message ?? 'Failed to create mailbox')
    } finally {
      setCreating(false)
    }
  }, [onAddTab, newLocalPart, newTTL])

  // The one irreversible action gets a second click, inline on the row.
  const handleDelete = useCallback(async (tab: MailboxTab) => {
    if (!tab.mailbox) return
    const name = mailboxName(tab.mailbox.localPart)
    setConfirmId(null)
    try {
      await onDeleteTab(tab.id)
      toast.success(`Deleted ${name}`)
    } catch {
      toast.error('Failed to delete mailbox')
    }
  }, [onDeleteTab])

  const handlePin = useCallback(async (tab: MailboxTab) => {
    if (!tab.mailbox) return
    const keep = !isForever(tab.mailbox.expiresAt)
    try {
      await onSetTabTTL(tab.id, keep ? TTL_FOREVER : 168)
      toast.success(keep ? `${mailboxName(tab.mailbox.localPart)} kept forever` : `${mailboxName(tab.mailbox.localPart)} expires in 7 days`)
    } catch {
      toast.error('Failed to update mailbox')
    }
  }, [onSetTabTTL])

  const navItems: { id: NavView; label: string; icon: React.ReactNode; count?: number }[] = [
    { id: 'inbox',       label: 'Inbox',       icon: <Inbox className="size-3.5" />,     count: emailCount > 0 ? emailCount : undefined },
    { id: 'starred',     label: 'Starred',     icon: <Star className="size-3.5" />,      count: starredCount > 0 ? starredCount : undefined },
    { id: 'attachments', label: 'Attachments', icon: <Paperclip className="size-3.5" />, count: attachmentEmailCount > 0 ? attachmentEmailCount : undefined },
    { id: 'settings',    label: 'Settings',    icon: <Settings className="size-3.5" /> },
  ]

  const rowClass = (active: boolean) => cn(
    'group flex items-center gap-1.5 rounded-md px-2 py-1 text-[11px] transition-colors cursor-pointer',
    active
      ? 'bg-emerald-600/15 text-emerald-600 dark:text-emerald-300'
      : 'text-secondary hover:bg-surface-2 hover:text-primary',
  )
  const badge = (n: number) => (
    <span className="shrink-0 min-w-[16px] px-1 text-center rounded-full bg-emerald-600 text-white text-[9px] font-semibold leading-4">{n}</span>
  )
  const iconBtn = 'shrink-0 p-0.5 rounded opacity-50 group-hover:opacity-100 transition-opacity text-muted hover:text-primary'

  return (
    <aside className="w-[220px] shrink-0 flex flex-col bg-surface-1 border-r border-border overflow-hidden select-none relative">

      {/* ── Brand header ─────────────────────────────────────── */}
      <div className="flex items-center gap-2 px-4 py-3.5 border-b border-border shrink-0">
        <LogoMark />
        <div>
          <div className="text-[13px] font-semibold text-primary leading-none">shitmail</div>
          <div className="text-[10px] text-muted leading-none mt-0.5">Disposable Email</div>
        </div>
      </div>

      {/* ── New mailbox: the primary action, first thing under the logo ── */}
      <div className="px-3 pt-3 pb-2 border-b border-border shrink-0">
        <AnimatePresence mode="wait">
          {showNewForm ? (
            <motion.div
              key="form"
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.15 }}
              className="overflow-hidden"
            >
              <div className="space-y-1.5">
                <input
                  value={newLocalPart}
                  onChange={e => setNewLocalPart(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleCreate()}
                  placeholder="name (optional, random if empty)"
                  className="w-full bg-surface-2 border border-border rounded-md px-2 py-1 text-[11px] font-mono text-primary placeholder-muted outline-none focus:border-emerald-500/50 transition-colors"
                  autoFocus
                />
                {addressSuffix && (
                  <p className="font-mono text-[10px] text-muted truncate px-0.5">
                    {(newLocalPart.trim() || 'name') + addressSuffix.replace(/^-[^@]*/, '')}
                  </p>
                )}
                <div className="flex gap-1">
                  {TTL_OPTIONS.map(opt => (
                    <button
                      key={opt.hours}
                      onClick={() => setNewTTL(opt.hours)}
                      title={opt.label}
                      className={cn(
                        'flex-1 py-1 rounded text-[10px] font-medium transition-colors border',
                        newTTL === opt.hours
                          ? 'bg-emerald-600/25 text-emerald-600 dark:text-emerald-300 border-emerald-600/40'
                          : 'bg-surface-2 text-muted border-border hover:text-secondary',
                      )}
                    >
                      {opt.short}
                    </button>
                  ))}
                </div>
                <div className="flex gap-1">
                  <button
                    onClick={() => { setShowNewForm(false); setNewLocalPart('') }}
                    className="flex-1 py-1 rounded text-[10px] text-muted hover:text-secondary border border-border transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleCreate}
                    disabled={creating}
                    className="flex-1 py-1 rounded text-[10px] font-medium bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white transition-colors"
                  >
                    {creating ? 'Creating…' : 'Create'}
                  </button>
                </div>
              </div>
            </motion.div>
          ) : (
            <motion.button
              key="btn"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setShowNewForm(true)}
              className="w-full flex items-center justify-center gap-1.5 py-1.5 rounded-md border border-border text-[11px] text-secondary hover:text-primary hover:border-emerald-500/30 hover:bg-emerald-600/8 transition-all"
            >
              <Plus className="size-3" />
              New Mailbox
            </motion.button>
          )}
        </AnimatePresence>
      </div>

      {/* ── Mailboxes: all + one row each; copy / pin / delete live on the row ── */}
      <div className="flex flex-col px-2 py-1.5 border-b border-border shrink-0 max-h-[45vh] overflow-y-auto">
        <p className="text-[9px] font-semibold uppercase tracking-widest text-muted px-2 mb-1">Mailboxes</p>
        {loading && tabs.length === 0 && (
          <div className="space-y-1 animate-pulse px-2"><div className="h-5 bg-surface-3 rounded" /><div className="h-5 bg-surface-3 rounded w-3/4" /></div>
        )}
        {!loading && tabs.length === 0 && (
          <p className="text-[11px] text-muted px-2 py-1">No mailbox yet.</p>
        )}
        {tabs.length > 1 && (
          <div className={cn(rowClass(allActive), 'mb-0.5')} onClick={onSelectAll}>
            <Inbox className="size-3 shrink-0" />
            <span className="flex-1 min-w-0 truncate font-medium">All mailboxes</span>
            {unreadCount > 0 && badge(unreadCount)}
          </div>
        )}
        {tabs.map(tab => {
          const unread = tab.emails.filter(e => !e.isRead).length
          const mb = tab.mailbox
          const kept = mb ? isForever(mb.expiresAt) : false
          if (confirmId === tab.id && mb) {
            return (
              <div key={tab.id} className="flex items-center gap-1 rounded-md px-2 py-1 text-[11px] bg-red-950/40 text-red-300 border border-red-900/50">
                <span className="flex-1 min-w-0 truncate">Delete {mailboxName(mb.localPart)}, {tab.emails.length} mail{tab.emails.length === 1 ? '' : 's'}?</span>
                <button onClick={e => { e.stopPropagation(); handleDelete(tab) }} className="shrink-0 px-1.5 rounded bg-red-600 text-white text-[10px] font-semibold hover:bg-red-500">Delete</button>
                <button onClick={e => { e.stopPropagation(); setConfirmId(null) }} className="shrink-0 px-1.5 rounded text-[10px] text-muted hover:text-primary">Keep</button>
              </div>
            )
          }
          return (
            <div key={tab.id} className={rowClass(tab.id === activeTabId)} onClick={() => onSelectTab(tab.id)} title={mb?.address}>
              <span className="flex-1 min-w-0 truncate font-medium">
                {mb ? mailboxName(mb.localPart) : 'Loading…'}
              </span>
              {unread > 0 && badge(unread)}
              {mb && (kept
                ? <Pin className="size-2.5 shrink-0 text-emerald-500/70" />
                : <RowExpiry expiresAt={mb.expiresAt} />)}
              {mb && (
                <span className="flex items-center shrink-0 ml-0.5">
                  <button onClick={e => { e.stopPropagation(); copyAddress(mb.address) }} title={`Copy ${mb.address}`} className={iconBtn}>
                    <Copy className="size-3" />
                  </button>
                  <button onClick={e => { e.stopPropagation(); handlePin(tab) }} title={kept ? 'Let it expire in 7 days' : 'Keep forever'} className={iconBtn}>
                    {kept ? <PinOff className="size-3" /> : <Pin className="size-3" />}
                  </button>
                  <button onClick={e => { e.stopPropagation(); setConfirmId(tab.id) }} title="Delete mailbox" className={cn(iconBtn, 'hover:text-red-400')}>
                    <Trash2 className="size-3" />
                  </button>
                </span>
              )}
            </div>
          )
        })}
      </div>

      {/* ── Menu navigation ────────────────────────────────────── */}
      <nav className="flex-1 px-2 py-2 overflow-y-auto">
        <p className="text-[9px] font-semibold uppercase tracking-widest text-muted px-2 mb-1">Menu</p>
        <ul className="space-y-0.5">
          {navItems.map(item => (
            <li key={item.id}>
              <button
                onClick={() => onViewChange(item.id)}
                className={cn(
                  'w-full flex items-center gap-2.5 px-2 py-1.5 rounded-md text-[12px] font-medium transition-all text-left',
                  view === item.id
                    ? 'bg-emerald-600/15 text-emerald-700 dark:text-emerald-300'
                    : 'text-secondary hover:text-primary hover:bg-surface-2',
                )}
              >
                <span className={cn('shrink-0', view === item.id ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted')}>
                  {item.icon}
                </span>
                <span className="flex-1">{item.label}</span>
                {item.count != null && (
                  <span className={cn(
                    'text-[10px] font-semibold px-1.5 py-0.5 rounded-full',
                    view === item.id ? 'bg-emerald-600/30 text-emerald-700 dark:text-emerald-300' : 'bg-surface-3 text-secondary',
                  )}>
                    {item.count > 99 ? '99+' : item.count}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </nav>
    </aside>
  )
}
