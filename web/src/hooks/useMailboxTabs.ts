import { useState, useEffect, useCallback, useRef } from 'react'
import * as api from '@/api/client'
import type { Email, Mailbox } from '@/types'

// ─── Types ───────────────────────────────────────────────────────────────────

export interface MailboxTab {
  id: string
  mailbox: Mailbox | null
  emails: Email[]
  loading: boolean
  wsConnected: boolean
}

export interface UseMailboxTabsReturn {
  tabs: MailboxTab[]
  initializing: boolean
  activeTabId: string
  activeTab: MailboxTab | null
  unreadCount: number
  addTab: (localPart?: string, ttlHours?: number) => Promise<void>
  closeTab: (id: string) => void
  setActiveTab: (id: string) => void
  replaceTabMailbox: (tabId: string, localPart?: string, ttlHours?: number) => Promise<void>
  deleteTabMailbox: (tabId: string) => Promise<void>
  setTabTTL: (tabId: string, ttlHours: number) => Promise<void>
  refreshTab: (tabId: string) => Promise<void>
  addEmailToTab: (tabId: string, email: Email) => void
  removeEmailFromTab: (tabId: string, emailId: string) => void
  markEmailReadInTab: (tabId: string, emailId: string) => void
  setTabWsConnected: (tabId: string, v: boolean) => void
}

// ─── Storage ─────────────────────────────────────────────────────────────────
// Mailboxes live server-side (per SSO user); only the active selection is local.

const ACTIVE_KEY = 'shitmail_active_tab'

/** activeTabId value for the merged "all mailboxes" view. */
export const ALL_TABS = '__all__'

// ─── Hook ────────────────────────────────────────────────────────────────────

export function useMailboxTabs(): UseMailboxTabsReturn {
  const [tabs, setTabs] = useState<MailboxTab[]>([])
  const [activeTabId, setActiveTabIdState] = useState('')
  const [initializing, setInitializing] = useState(true)
  const didInit = useRef(false)

  // Stable ref so callbacks don't need tabs/activeTabId in deps
  const tabsRef = useRef<MailboxTab[]>([])
  tabsRef.current = tabs
  const activeTabIdRef = useRef(activeTabId)
  activeTabIdRef.current = activeTabId

  useEffect(() => {
    if (activeTabId) try { localStorage.setItem(ACTIVE_KEY, activeTabId) } catch { /* ignore */ }
  }, [activeTabId])

  // ── Init: load the user's mailboxes from the server ───────────────────────
  useEffect(() => {
    if (didInit.current) return
    didInit.current = true

    async function init() {
      try {
        const { mailboxes } = await api.listMailboxes()
        const loaded = await Promise.all(
          (mailboxes ?? []).map(async (mailbox): Promise<MailboxTab> => {
            let emails: Email[] = []
            try { emails = (await api.listEmails(mailbox.address)).emails ?? [] } catch { /* empty */ }
            return { id: mailbox.id, mailbox, emails, loading: false, wsConnected: false }
          }),
        )
        setTabs(loaded)
        const saved = localStorage.getItem(ACTIVE_KEY)
        if (saved === ALL_TABS || (!saved && loaded.length > 1)) {
          setActiveTabIdState(ALL_TABS)
        } else {
          const active = loaded.find(t => t.id === saved) ?? loaded[0]
          if (active) setActiveTabIdState(active.id)
        }
      } finally {
        setInitializing(false)
      }
    }

    init()
  }, [])

  // ── Helpers ───────────────────────────────────────────────────────────────

  const mutateTab = useCallback((tabId: string, updater: (t: MailboxTab) => MailboxTab) => {
    setTabs(prev => prev.map(t => (t.id === tabId ? updater(t) : t)))
  }, [])

  // ── Public API ────────────────────────────────────────────────────────────

  const addTab = useCallback(async (localPart?: string, ttlHours?: number) => {
    const mb = await api.createMailbox(localPart, ttlHours)
    setTabs(prev => [...prev, { id: mb.id, mailbox: mb, emails: [], loading: false, wsConnected: false }])
    setActiveTabIdState(mb.id)
  }, [])

  // Removes the tab locally; the mailbox itself is deleted by deleteTabMailbox.
  const closeTab = useCallback((id: string) => {
    const remaining = tabsRef.current.filter(t => t.id !== id)
    setTabs(remaining)
    setActiveTabIdState(prev => (prev !== id ? prev : remaining[0]?.id ?? ''))
  }, [])

  const setActiveTab = useCallback((id: string) => {
    setActiveTabIdState(id)
  }, [])

  const replaceTabMailbox = useCallback(async (tabId: string, localPart?: string, ttlHours?: number) => {
    mutateTab(tabId, t => ({ ...t, loading: true, emails: [] }))
    try {
      const mb = await api.createMailbox(localPart, ttlHours)
      mutateTab(tabId, t => ({ ...t, mailbox: mb, loading: false }))
    } catch (err) {
      mutateTab(tabId, t => ({ ...t, loading: false }))
      throw err
    }
  }, [mutateTab])

  // Deletes the mailbox server-side and drops its tab. No replacement is
  // generated: addresses are explicit, never auto-created.
  const deleteTabMailbox = useCallback(async (tabId: string) => {
    const tab = tabsRef.current.find(t => t.id === tabId)
    if (!tab?.mailbox) return
    await api.deleteMailbox(tab.mailbox.address)
    closeTab(tabId)
  }, [closeTab])

  const setTabTTL = useCallback(async (tabId: string, ttlHours: number) => {
    const tab = tabsRef.current.find(t => t.id === tabId)
    if (!tab?.mailbox) return
    const mb = await api.setMailboxTTL(tab.mailbox.address, ttlHours)
    mutateTab(tabId, t => ({ ...t, mailbox: mb }))
  }, [mutateTab])

  const refreshTab = useCallback(async (tabId: string) => {
    const tab = tabsRef.current.find(t => t.id === tabId)
    if (!tab?.mailbox) return
    try {
      const { emails } = await api.listEmails(tab.mailbox.address)
      mutateTab(tabId, t => ({ ...t, emails: emails ?? [] }))
    } catch { /* ignore */ }
  }, [mutateTab])

  const addEmailToTab = useCallback((tabId: string, email: Email) => {
    setTabs(prev => prev.map(t => {
      if (t.id !== tabId) return t
      if (t.emails.some(e => e.id === email.id)) return t
      return { ...t, emails: [email, ...t.emails] }
    }))
  }, [])

  const removeEmailFromTab = useCallback((tabId: string, emailId: string) => {
    setTabs(prev => prev.map(t =>
      t.id !== tabId ? t : { ...t, emails: t.emails.filter(e => e.id !== emailId) }
    ))
  }, [])

  const markEmailReadInTab = useCallback((tabId: string, emailId: string) => {
    setTabs(prev => prev.map(t =>
      t.id !== tabId ? t : {
        ...t,
        emails: t.emails.map(e => e.id === emailId ? { ...e, isRead: true } : e),
      }
    ))
  }, [])

  const setTabWsConnected = useCallback((tabId: string, v: boolean) => {
    mutateTab(tabId, t => ({ ...t, wsConnected: v }))
  }, [mutateTab])

  const activeTab = tabs.find(t => t.id === activeTabId) ?? null
  const unreadCount = tabs.reduce(
    (sum, t) => sum + t.emails.filter(e => !e.isRead).length,
    0,
  )

  return {
    tabs,
    initializing,
    activeTabId,
    activeTab,
    unreadCount,
    addTab,
    closeTab,
    setActiveTab,
    replaceTabMailbox,
    deleteTabMailbox,
    setTabTTL,
    refreshTab,
    addEmailToTab,
    removeEmailFromTab,
    markEmailReadInTab,
    setTabWsConnected,
  }
}
