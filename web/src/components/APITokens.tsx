import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import * as api from '@/api/client'

export function APITokens() {
  const [tokens, setTokens] = useState<api.APIToken[]>([])
  const [name, setName] = useState('')
  const [days, setDays] = useState(30)
  const [secret, setSecret] = useState('')
  const [busy, setBusy] = useState(false)
  const refresh = () => api.listTokens().then(r => setTokens(r.tokens))
  useEffect(() => { refresh().catch(e => toast.error(e.message)) }, [])
  async function create() {
    setBusy(true)
    try {
      const result = await api.createToken(name.trim(), days)
      setSecret(result.secret)
      setName('')
      await refresh()
    } catch (e) { toast.error((e as Error).message) }
    finally { setBusy(false) }
  }
  async function revoke(id: string) {
    setBusy(true)
    try { await api.revokeToken(id); setSecret(''); await refresh(); toast.success('Token revoked') }
    catch (e) { toast.error((e as Error).message) }
    finally { setBusy(false) }
  }
  return <div className="space-y-3 text-xs">
    <p className="text-secondary">Give an agent read-only access to your mailboxes and messages. Tokens expire and can be revoked at any time.</p>
    <div className="flex flex-wrap gap-2">
      <input aria-label="Token name" placeholder="Agent name" maxLength={80} value={name} onChange={e => setName(e.target.value)} className="min-w-0 flex-1 rounded-sm border border-border bg-surface-0 p-2 text-primary" />
      <select aria-label="Token expiry" value={days} onChange={e => setDays(Number(e.target.value))} className="rounded-sm border border-border bg-surface-0 p-2 text-primary">
        {[7, 30, 90, 365].map(d => <option key={d} value={d}>{d} days</option>)}
      </select>
      <button disabled={busy || !name.trim()} onClick={create} className="rounded-sm bg-emerald-600 px-3 py-2 text-white disabled:opacity-40">Create token</button>
    </div>
    {secret && <div className="space-y-2 rounded-sm border border-border p-3">
      <p className="text-secondary">Copy this token now. It will not be shown again.</p>
      <code className="block break-all select-all text-primary">{secret}</code>
      <div className="flex gap-3">
        <button onClick={() => navigator.clipboard.writeText(secret).then(() => toast.success('Token copied')).catch(() => toast.error('Could not copy token'))} className="text-emerald-500">Copy token</button>
        <button onClick={() => setSecret('')} className="text-muted">Dismiss</button>
      </div>
    </div>}
    {tokens.map(token => <div key={token.id} className="flex items-center gap-2 border-t border-border pt-2">
      <div className="flex-1 min-w-0"><p className="break-all text-primary">{token.name}</p><p className="text-muted">Expires {new Date(token.expiresAt).toLocaleDateString()}</p></div>
      <button disabled={busy} onClick={() => revoke(token.id)} className="text-red-400 disabled:opacity-40">Revoke</button>
    </div>)}
    <p className="text-muted">Agent API base URL:</p>
    <code className="block break-all select-text text-secondary">{window.location.origin}/api/agent/v1</code>
    <p className="text-muted">Send Authorization: Bearer YOUR_TOKEN. Start with GET /mailboxes, then GET /mailbox/ADDRESS/emails. Reading through this API keeps messages unread.</p>
  </div>
}
